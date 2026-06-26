package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/binname"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/pkgselect"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// installByVersionMigrateCtx installs a package at an exact version.
// It is a package-level variable so tests can swap the implementation.
var installByVersionMigrateCtx = goutil.InstallWithContext //nolint:gochecknoglobals // swapped in tests

const (
	// migrateMinArgs is the minimum number of positional arguments for migrate
	// (BEFORE_PATH and AFTER_PATH).
	migrateMinArgs = 2
	// afterPathDirPerm is the permission used when creating AFTER_PATH.
	afterPathDirPerm = 0o755
	// commandLineArguments is the import path 'go install' records for binaries
	// built from local files; such binaries cannot be reinstalled by path.
	commandLineArguments = "command-line-arguments"
	// develVersion and develVersionParen are the version strings recorded for
	// development builds, which are skipped instead of being upgraded.
	develVersion      = "devel"
	develVersionParen = "(devel)"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate BEFORE_PATH AFTER_PATH [BINARY...]",
		Short: "Reinstall go-install binaries from one GOBIN directory into another",
		Long: `Reinstall go-install binaries from one GOBIN directory into another.

gup migrate scans the binaries under BEFORE_PATH, reads the exact
'import path@version' recorded in each binary's build info, and reinstalls
them into AFTER_PATH using the same versions (no automatic upgrade to @latest).

This is useful when the real path of $GOBIN changes between Go toolchains.
For example, when 'mise' updates Go, the resolved $GOBIN can differ per Go
version, so tools installed under the previous $GOBIN are no longer visible.
'gup migrate /old/gobin /new/gobin' reinstalls the same Go tools into the new
$GOBIN.

migrate is add-only: it never deletes files in AFTER_PATH, and by default it
skips binaries that already exist there. Use --force to reinstall over them.

If BINARY arguments are given, only those binaries are migrated.`,
		Example: `  gup migrate /old/gobin /new/gobin
  gup migrate /old/gobin /new/gobin gopls`,
		Args: requireMinArgs(migrateMinArgs,
			"requires BEFORE_PATH and AFTER_PATH",
			"gup migrate /old/gobin /new/gobin"),
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(runMigrate(printerFor(cmd), cmd, args))
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "perform the trial migration with no changes")
	cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "specify the number of CPU cores to use")
	mustRegisterFlagCompletion(cmd, "jobs", completeNCPUs)
	cmd.Flags().Bool("force", false, "reinstall even if the binary already exists in AFTER_PATH")
	addTimeoutFlag(cmd)

	return cmd
}

func runMigrate(p *print.Printer, cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		p.Err(err)
		return 1
	}

	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		p.Err(err)
		return 1
	}
	notify, err := getFlagBool(cmd, "notify")
	if err != nil {
		p.Err(err)
		return 1
	}
	cpus, err := getFlagInt(cmd, "jobs")
	if err != nil {
		p.Err(err)
		return 1
	}
	cpus = clampJobs(cpus)
	force, err := getFlagBool(cmd, "force")
	if err != nil {
		p.Err(err)
		return 1
	}
	timeout, err := getTimeoutFlag(cmd)
	if err != nil {
		p.Err(err)
		return 1
	}

	beforePath := args[0]
	afterPath := args[1]
	binaries := args[2:]

	if err := validateMigratePaths(beforePath, afterPath, dryRun); err != nil {
		p.Err(err)
		return 1
	}

	binList, err := goutil.BinaryPathList(beforePath)
	if err != nil {
		p.Err(fmt.Errorf("can't read binaries under %s: %w", beforePath, err))
		return 1
	}
	binList = pkgselect.FilterBinaryPaths(binList, binaries)
	// migrate never reads GoVersion, so skip the "go version" subprocess.
	pkgs := goutil.GetPackageInformationWithoutGoVersion(p, binList)
	warnMissingMigrateTargets(p, binaries, pkgs, beforePath)

	if len(pkgs) == 0 {
		p.Err(fmt.Errorf("no go-install binary to migrate under %s", beforePath))
		return 1
	}

	p.Info(fmt.Sprintf("start migration from %s to %s", beforePath, afterPath))
	return migratePackages(p, pkgs, afterPath, dryRun, notify, cpus, force, timeout)
}

// validateMigratePaths validates BEFORE_PATH and AFTER_PATH.
// BEFORE_PATH must be an existing directory. AFTER_PATH is created when it does
// not exist (unless dryRun), and must not be an existing file. The two paths
// must not resolve to the same directory.
func validateMigratePaths(beforePath, afterPath string, dryRun bool) error {
	beforeInfo, err := os.Stat(beforePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("BEFORE_PATH does not exist: %s", beforePath)
		}
		return fmt.Errorf("can't access BEFORE_PATH %s: %w", beforePath, err)
	}
	if !beforeInfo.IsDir() {
		return fmt.Errorf("BEFORE_PATH is not a directory: %s", beforePath)
	}

	// Reject the same-directory case before creating AFTER_PATH, so that a
	// validation failure never mutates the filesystem.
	same, err := sameDirPath(beforePath, afterPath)
	if err != nil {
		return err
	}
	if same {
		return fmt.Errorf("BEFORE_PATH and AFTER_PATH are the same directory: %s", beforePath)
	}

	afterInfo, err := os.Stat(afterPath)
	switch {
	case err == nil:
		if !afterInfo.IsDir() {
			return fmt.Errorf("AFTER_PATH is not a directory: %s", afterPath)
		}
	case errors.Is(err, os.ErrNotExist):
		if !dryRun {
			if mkErr := os.MkdirAll(afterPath, afterPathDirPerm); mkErr != nil {
				return fmt.Errorf("can't create AFTER_PATH %s: %w", afterPath, mkErr)
			}
		}
	default:
		return fmt.Errorf("can't access AFTER_PATH %s: %w", afterPath, err)
	}

	return nil
}

// sameDirPath reports whether a and b point to the same location.
// It compares absolute paths first, then falls back to resolved symlinks.
func sameDirPath(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, fmt.Errorf("can't resolve path %s: %w", a, err)
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, fmt.Errorf("can't resolve path %s: %w", b, err)
	}
	if absA == absB {
		return true, nil
	}

	evalA, errA := filepath.EvalSymlinks(absA)
	evalB, errB := filepath.EvalSymlinks(absB)
	if errA == nil && errB == nil && evalA == evalB {
		return true, nil
	}
	return false, nil
}

func migratePackages(pr *print.Printer, pkgs []goutil.Package, afterPath string, dryRun, notification bool, cpus int, force bool, timeout time.Duration) int {
	// Point GOBIN at AFTER_PATH so the existing 'go install' path reinstalls
	// into the target directory. Restore the environment afterward. Dry-run
	// never installs, so the environment is left untouched.
	if !dryRun {
		restore, err := withGoBin(afterPath)
		if err != nil {
			pr.Err(fmt.Errorf("can't set GOBIN to %s: %w", afterPath, err))
			return 1
		}
		defer restore()
	}

	migrator := func(ctx context.Context, p goutil.Package) updateResult {
		version, skip, reason := resolveMigrateVersion(p)
		if skip {
			return updateResult{pkg: p, skipped: true, skipReason: reason}
		}

		targetName := binaryNameFromImportPath(p.ImportPath)
		if !force && binaryExistsInDir(afterPath, targetName) {
			return updateResult{pkg: p, skipped: true, skipReason: "already exists in AFTER_PATH (use --force to overwrite)"}
		}

		// Record the version that will be installed for display.
		if p.Version == nil {
			p.Version = &goutil.Version{}
		}
		p.Version.Current = version

		if dryRun {
			return updateResult{updated: true, pkg: p}
		}

		if err := installByVersionMigrateCtx(ctx, p.ImportPath, version); err != nil {
			newPkg, changed := resolveModulePathChange(p, err)
			if !changed {
				return updateResult{pkg: p, err: fmt.Errorf("%s: %w", p.Name, err)}
			}
			// The module was renamed: retry with the new import path, keeping
			// the same exact version. No old-binary removal is needed here.
			newPkg.Version = p.Version
			if retryErr := installByVersionMigrateCtx(ctx, newPkg.ImportPath, version); retryErr != nil {
				return updateResult{pkg: newPkg, err: fmt.Errorf("%s: %w", p.Name, retryErr)}
			}
			return updateResult{updated: true, pkg: newPkg}
		}
		return updateResult{updated: true, pkg: p}
	}

	result, _ := executePackages(pr, pkgs, cpus, timeout, migrator, func(prefix string, v updateResult) {
		if v.skipped {
			pr.Info(fmt.Sprintf("%s skip %s: %s", prefix, v.pkg.Name, v.skipReason))
			return
		}
		pr.Info(fmt.Sprintf("%s %s@%s", prefix, v.pkg.ImportPath, v.pkg.Version.Current))
	})

	desktopNotifyIfNeeded(pr, result, notification)
	return result
}

// warnMissingMigrateTargets warns about each requested binary name that was not
// found among the migration targets, mirroring the target-selection UX of
// 'gup update'. It is a no-op when no binaries were explicitly requested.
func warnMissingMigrateTargets(pr *print.Printer, targets []string, pkgs []goutil.Package, beforePath string) {
	if len(targets) == 0 {
		return
	}

	present := make(map[string]struct{}, len(pkgs))
	for _, p := range pkgs {
		present[binname.NormalizeForMatch(p.Name)] = struct{}{}
	}

	seen := make(map[string]struct{}, len(targets))
	for _, raw := range targets {
		normalized := binname.NormalizeForMatch(raw)
		if normalized == "" {
			continue
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		seen[normalized] = struct{}{}
		if _, ok := present[normalized]; !ok {
			pr.Warn("not found '" + strings.TrimSpace(raw) + "' package in " + beforePath)
		}
	}
}

// resolveMigrateVersion returns the exact version to reinstall, or reports that
// the package should be skipped. Binaries without a resolvable import path or
// version, and development builds, are skipped rather than upgraded to latest.
func resolveMigrateVersion(p goutil.Package) (version string, skip bool, reason string) {
	if p.ImportPath == "" {
		return "", true, "import path is unknown"
	}
	if p.ImportPath == commandLineArguments {
		return "", true, "devel binary copied from local environment"
	}

	var current string
	if p.Version != nil {
		current = strings.TrimSpace(p.Version.Current)
	}
	if current == "" {
		return "", true, "version is empty"
	}
	if current == develVersion || current == develVersionParen {
		return "", true, "version is a development build"
	}
	return current, false, ""
}

// binaryExistsInDir reports whether a regular file (or symlink) named name
// exists directly under dir.
func binaryExistsInDir(dir, name string) bool {
	if name == "" {
		return false
	}
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// withGoBin sets the GOBIN environment variable to path and returns a function
// that restores the previous value (or unsets it when it was not present).
func withGoBin(path string) (func(), error) {
	orig, had := os.LookupEnv("GOBIN")
	if err := os.Setenv("GOBIN", path); err != nil {
		return nil, err
	}
	return func() {
		if had {
			_ = os.Setenv("GOBIN", orig)
		} else {
			_ = os.Unsetenv("GOBIN")
		}
	}, nil
}
