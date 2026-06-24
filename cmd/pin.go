package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nao1215/gup/internal/binname"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/pkgselect"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

const (
	// pinMinArgs/pinMaxArgs bound the pin command: "gup pin tool@version" (1) or
	// "gup pin tool version" (2).
	pinMinArgs = 1
	pinMaxArgs = 2
)

func newPinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pin TOOL[@VERSION] [VERSION]",
		Short: "Pin a binary to a specific version so 'gup update' keeps it there",
		Long: `Pin a binary to a specific version.

A pinned binary is recorded in gup.json with channel "pinned" and a concrete
version. 'gup update' then installs that exact version with
'go install <import_path>@<version>' instead of resolving @latest, so the tool
stays on the version you rely on (for example to match CI or a team-wide
development environment). Run 'gup unpin' to allow the tool to update again.`,
		Example: `  gup pin golangci-lint v1.62.0
  gup pin golangci-lint@v1.62.0`,
		Args:              cobra.RangeArgs(pinMinArgs, pinMaxArgs),
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(runPin(cmd, args))
		},
	}
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}
	return cmd
}

func newUnpinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpin TOOL",
		Short: "Remove the version pin from a binary so 'gup update' can update it again",
		Long: `Remove the version pin from a binary.

The binary's gup.json entry is reset to the @latest channel, so the next
'gup update' updates it normally. Unpinning a tool that is not pinned does
nothing and succeeds.`,
		Example:           `  gup unpin golangci-lint`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(runUnpin(cmd, args))
		},
	}
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}
	return cmd
}

// parsePinArgs splits the pin arguments into a target and a concrete version,
// accepting both "gup pin tool v1.2.3" and "gup pin tool@v1.2.3". The version is
// validated so an unsafe pin (empty, or a channel keyword such as "latest") is
// rejected up front.
func parsePinArgs(args []string) (target, version string, err error) {
	switch len(args) {
	case pinMinArgs:
		at := strings.LastIndex(args[0], "@")
		if at < 0 {
			return "", "", fmt.Errorf("missing version: use 'gup pin %s VERSION' or 'gup pin %s@VERSION'", args[0], args[0])
		}
		target = strings.TrimSpace(args[0][:at])
		version = strings.TrimSpace(args[0][at+1:])
	case pinMaxArgs:
		target = strings.TrimSpace(args[0])
		version = strings.TrimSpace(args[1])
		if strings.Contains(target, "@") {
			return "", "", errors.New("specify the version once: either 'gup pin TOOL@VERSION' or 'gup pin TOOL VERSION'")
		}
	default:
		return "", "", errors.New("pin takes a tool and a version")
	}

	if target == "" {
		return "", "", errors.New("missing tool name")
	}
	if err := goutil.ValidatePinnedVersion(version); err != nil {
		return "", "", err
	}
	return target, version, nil
}

func runPin(cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	target, version, err := parsePinArgs(args)
	if err != nil {
		print.Err(err)
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}

	installed, err := pkgselect.PackageInfo()
	if err != nil {
		print.Err(err)
		return 1
	}
	pkg, ok := resolveManagedTarget(installed, target)
	if !ok {
		print.Err(fmt.Errorf("'%s' is not managed by gup: install it with 'go install' first", target))
		return 1
	}

	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		print.Err(err)
		return 1
	}

	merged, err := configstate.SetPin(confPkgs, pkg, version)
	if err != nil {
		print.Err(err)
		return 1
	}

	writePath := configstate.ResolveWritePath(confFile, confReadPath)
	if err := writeConfigFile(writePath, merged); err != nil {
		print.Err(err)
		return 1
	}

	print.Info(fmt.Sprintf("Pinned %s to %s (run 'gup update' to apply)", pkg.Name, version))
	return 0
}

func runUnpin(cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	target := strings.TrimSpace(args[0])
	if i := strings.LastIndex(target, "@"); i >= 0 {
		target = strings.TrimSpace(target[:i])
	}
	if target == "" {
		print.Err(errors.New("missing tool name"))
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}

	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		print.Err(err)
		return 1
	}

	merged, changed := configstate.RemovePin(confPkgs, targetPackage(target))
	if !changed {
		print.Info(target + " is not pinned; nothing to do")
		return 0
	}

	writePath := configstate.ResolveWritePath(confFile, confReadPath)
	if err := writeConfigFile(writePath, merged); err != nil {
		print.Err(err)
		return 1
	}

	print.Info("Unpinned " + target)
	return 0
}

// resolveManagedTarget finds the installed package named by target, matching by
// import path (when target looks like an import path) and then by cross-OS
// normalized binary name. It returns false when no installed binary matches, so
// pinning a tool gup does not manage fails fast.
func resolveManagedTarget(installed []goutil.Package, target string) (goutil.Package, bool) {
	for _, p := range installed {
		if p.ImportPath != "" && p.ImportPath == target {
			return p, true
		}
	}
	want := binname.NormalizeForMatch(target)
	for _, p := range installed {
		if binname.NormalizeForMatch(p.Name) == want {
			return p, true
		}
	}
	return goutil.Package{}, false
}

// targetPackage builds the minimal package used to identify an unpin target: an
// import path when the target looks like one, otherwise a bare binary name. The
// shared identity rule matches either against the saved config entry.
func targetPackage(target string) goutil.Package {
	if strings.Contains(target, "/") {
		return goutil.Package{ImportPath: target, Name: target[strings.LastIndex(target, "/")+1:]}
	}
	return goutil.Package{Name: target}
}
