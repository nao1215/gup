package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove the binary under $GOPATH/bin or $GOBIN",
		Long: `Remove command in $GOPATH/bin or $GOBIN.
If you want to specify multiple binaries at once, separate them with space.
[e.g.] gup remove a_cmd b_cmd c_cmd`,
		Example: `  gup remove gopls
  gup remove --force air`,
		Args: requireMinArgs(1,
			"requires at least one binary name",
			"gup remove gopls",
			"gup remove --force air"),
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(remove(cmd, args))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Forcibly remove the file")

	return cmd
}

func remove(cmd *cobra.Command, args []string) int {
	force, err := getFlagBool(cmd, "force")
	if err != nil {
		print.Err(err)
		return 1
	}

	gobin, err := goutil.GoBin()
	if err != nil {
		print.Err(err)
		return 1
	}

	return removeLoop(gobin, force, args)
}

const goosWindows = "windows"

// GOOS is wrapper for runtime.GOOS variable. It's for unit test.
var GOOS = runtime.GOOS //nolint:gochecknoglobals

// stdinIsTerminal reports whether os.Stdin is connected to a terminal (TTY).
// It is a package-level variable so that it can be overridden in unit tests.
var stdinIsTerminal = func() bool { //nolint:gochecknoglobals
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func removeLoop(gobin string, force bool, target []string) int {
	result := 0
	for _, v := range target {
		orig := v
		v = strings.TrimSpace(v)
		// In Windows, $GOEXE is set to the ".exe" extension.
		// The user-specified command name (arguments) may not have an extension.
		execSuffix := normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
		if GOOS == goosWindows && !hasSuffixFold(v, execSuffix) {
			v += execSuffix
		}
		if !isSafeBinaryName(v) {
			print.Err(fmt.Errorf("invalid command name: %s", orig))
			result = 1
			continue
		}

		target := filepath.Join(gobin, v)
		if !fileutil.IsFile(target) {
			print.Err(fmt.Errorf("no such file or directory: %s", target))
			result = 1
			continue
		}
		if !force {
			if !stdinIsTerminal() {
				print.Err(fmt.Errorf("gup remove requires confirmation, but stdin is not a TTY.\nUse --force to skip confirmation"))
				result = 1
				continue
			}
			if !print.Question(fmt.Sprintf("remove %s?", target)) {
				print.Info("cancel removal " + target)
				continue
			}
		}

		//nolint:gosec // target is constrained to a file name under gobin by isSafeBinaryName.
		if err := os.Remove(target); err != nil {
			print.Err(err)
			result = 1
			continue
		}
		print.Info("removed " + target)
	}
	return result
}

func normalizeExecSuffix(goos, goExe string) string {
	if goos != goosWindows {
		return goExe
	}

	goExe = strings.TrimSpace(goExe)
	if goExe == "" {
		return ".exe"
	}
	return goExe
}

func hasSuffixFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func isSafeBinaryName(name string) bool {
	origName := name
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if origName != name {
		return false
	}
	if filepath.IsAbs(name) {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	if strings.Contains(name, ":") {
		return false
	}
	if name == "." || name == ".." {
		return false
	}
	if filepath.Base(name) != name {
		return false
	}
	return true
}
