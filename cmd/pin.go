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

// pinPackageInfo lists the binaries gup manages under $GOBIN. It is a seam so
// tests can drive 'gup pin' without a real $GOBIN populated by 'go install'.
var pinPackageInfo = pkgselect.PackageInfo //nolint:gochecknoglobals // swapped in tests

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
			OsExit(runPin(printerFor(cmd), cmd, args))
		},
	}
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write")
	mustMarkFileFlagAsJSON(cmd)
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
			OsExit(runUnpin(printerFor(cmd), cmd, args))
		},
	}
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write")
	mustMarkFileFlagAsJSON(cmd)
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

func runPin(p *print.Printer, cmd *cobra.Command, args []string) int {
	// pin only reads local build info and edits gup.json (channel "pinned"); it
	// never runs the Go toolchain, so it must not fail when 'go' is absent. This
	// mirrors 'gup unpin', which already works without go.
	target, version, err := parsePinArgs(args)
	if err != nil {
		p.Err(err)
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		p.Err(err)
		return 1
	}

	installed, err := pinPackageInfo(p)
	if err != nil {
		p.Err(err)
		return 1
	}
	pkg, err := resolvePinTarget(installed, target)
	if err != nil {
		p.Err(err)
		return 1
	}

	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		p.Err(err)
		return 1
	}
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		p.Err(err)
		return 1
	}

	merged, err := configstate.SetPin(confPkgs, pkg, version)
	if err != nil {
		p.Err(err)
		return 1
	}

	writePath := configstate.ResolveWritePath(confFile, confReadPath)
	if err := writeConfigFile(writePath, merged); err != nil {
		p.Err(err)
		return 1
	}

	p.Info(fmt.Sprintf("Pinned %s to %s (run 'gup update' to apply)", pkg.Name, version))
	return 0
}

func runUnpin(p *print.Printer, cmd *cobra.Command, args []string) int {
	// unpin only edits gup.json (channel back to @latest); it never runs the Go
	// toolchain, so it must not fail when 'go' is absent.
	target := strings.TrimSpace(args[0])
	if i := strings.LastIndex(target, "@"); i >= 0 {
		target = strings.TrimSpace(target[:i])
	}
	if target == "" {
		p.Err(errors.New("missing tool name"))
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		p.Err(err)
		return 1
	}

	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		p.Err(err)
		return 1
	}
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		p.Err(err)
		return 1
	}

	merged, changed := configstate.RemovePin(confPkgs, targetPackage(target))
	if !changed {
		p.Info(target + " is not pinned; nothing to do")
		return 0
	}

	writePath := configstate.ResolveWritePath(confFile, confReadPath)
	if err := writeConfigFile(writePath, merged); err != nil {
		p.Err(err)
		return 1
	}

	p.Info("Unpinned " + target)
	return 0
}

// resolvePinTarget resolves target to the single installed package that can be
// pinned. An exact import-path match is unambiguous; otherwise target is matched
// by cross-OS normalized binary name and must match exactly one installed
// package - if two managed tools share a binary name, the pin would land on the
// wrong import path, so this requires the caller to disambiguate with the full
// import path. It also rejects a tool whose import path gup cannot determine (an
// old binary without build info): pinning it would write an entry with an empty
// import_path, which ReadConfFile rejects as invalid, so a single 'gup pin'
// would break every later check/update/export (export already drops such
// packages; pin must not be the one command that lets them through).
func resolvePinTarget(installed []goutil.Package, target string) (goutil.Package, error) {
	for _, p := range installed {
		if p.ImportPath != "" && p.ImportPath == target {
			return p, nil
		}
	}

	want := binname.NormalizeForMatch(target)
	var match *goutil.Package
	for i := range installed {
		if binname.NormalizeForMatch(installed[i].Name) != want {
			continue
		}
		if match != nil {
			return goutil.Package{}, fmt.Errorf("'%s' matches multiple installed binaries; pin by full import path", target)
		}
		match = &installed[i]
	}
	if match == nil {
		return goutil.Package{}, fmt.Errorf("'%s' is not managed by gup: install it with 'go install' first", target)
	}
	if strings.TrimSpace(match.ImportPath) == "" {
		return goutil.Package{}, fmt.Errorf("can't pin '%s': gup can't determine its import path (it was built by an old Go version); reinstall it with 'go install' first", match.Name)
	}
	return *match, nil
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
