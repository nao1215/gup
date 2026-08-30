package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/lockfile"
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
			p := printerFor(cmd)
			OsExit(withStateLock(p, cmd, args, cmdNameRemove, func() int {
				return remove(p, cmd, args)
			}))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "forcibly remove the file")

	return cmd
}

func remove(p *print.Printer, cmd *cobra.Command, args []string) int {
	force, err := getFlagBool(cmd, "force")
	if err != nil {
		p.Err(err)
		return 1
	}

	gobin, err := goutil.GoBin()
	if err != nil {
		p.Err(err)
		return 1
	}

	return removeLoop(p, gobin, force, args)
}

const goosWindows = "windows"

// exeSuffix is the Windows executable file extension.
const exeSuffix = ".exe"

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

func removeLoop(p *print.Printer, gobin string, force bool, target []string) int {
	result := 0
	for _, v := range target {
		orig := v
		v = strings.TrimSpace(v)
		// The lock guarding $GOBIN lives in $GOBIN, and nothing installed it. It is
		// checked before the Windows suffix is applied and again after, because
		// $GOEXE is whatever the user set it to and both spellings resolve to the
		// same file.
		if lockfile.IsReservedName(v) {
			p.Err(reservedNameError(orig))
			result = 1
			continue
		}
		// In Windows, $GOEXE is set to the ".exe" extension.
		// The user-specified command name (arguments) may not have an extension.
		execSuffix := normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
		if GOOS == goosWindows && !hasSuffixFold(v, execSuffix) {
			v += execSuffix
		}
		if lockfile.IsReservedName(v) {
			p.Err(reservedNameError(orig))
			result = 1
			continue
		}
		if !isSafeBinaryName(v) {
			p.Err(fmt.Errorf("invalid command name: %s", orig))
			result = 1
			continue
		}

		target := filepath.Join(gobin, v)
		if !fileutil.IsFile(target) {
			p.Err(fmt.Errorf("no such file or directory: %s", target))
			result = 1
			continue
		}
		if !force {
			if !stdinIsTerminal() {
				p.Err(errors.New("gup remove requires confirmation, but stdin is not a TTY.\nUse --force to skip confirmation"))
				result = 1
				continue
			}
			ok, err := p.Question(fmt.Sprintf("remove %s?", target))
			if err != nil {
				// A failed confirmation read (EOF, closed pipe, ...) is not a
				// cancellation: report it and fail so the caller does not mistake a
				// never-confirmed removal for a successful one.
				p.Err(fmt.Errorf("confirmation could not be read: %w\nUse --force to skip confirmation", err))
				result = 1
				continue
			}
			if !ok {
				p.Info("cancel removal " + target)
				continue
			}
		}

		//nolint:gosec // target is constrained to a file name under gobin by isSafeBinaryName.
		if err := os.Remove(target); err != nil {
			p.Err(err)
			result = 1
			continue
		}
		p.Info("removed " + target)
	}
	return result
}

// reservedNameError explains why a name gup keeps for itself is refused.
//
// The refusal is not cosmetic. `gup remove .gup.lock --force` used to delete the
// lock file of the very command running it: the kernel lock survived, because it
// lives on an open descriptor rather than on a name, but the next gup found the
// name free, created its own file there and locked that instead - so two
// commands rewrote one $GOBIN believing they had it to themselves.
func reservedNameError(name string) error {
	return fmt.Errorf("%s is gup's own lock file, not an installed binary: refusing to remove %s."+
		" gup keeps it in $GOBIN to serialize the commands that change your tools, and it is safe to leave there",
		lockfile.DirLockName, name)
}

func normalizeExecSuffix(goos, goExe string) string {
	if goos != goosWindows {
		return goExe
	}

	goExe = strings.TrimSpace(goExe)
	if goExe == "" {
		return exeSuffix
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
