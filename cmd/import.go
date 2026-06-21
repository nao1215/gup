package cmd

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var installByVersionCtx = goutil.InstallWithContext //nolint:gochecknoglobals // swapped in tests

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Install commands according to gup.json.",
		Long: `Install commands according to gup.json.

Use export/import if you want to install the same Go binaries
across multiple systems.
First, run 'gup export' on the source environment and copy gup.json.
Then run 'gup import' on the target environment to install the
versions recorded in that gup.json.`,
		Example: `  gup import
  gup import --file gup.json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(runImport(cmd, args))
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
	cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to import")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}
	addTimeoutFlag(cmd)

	return cmd
}

func runImport(cmd *cobra.Command, _ []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		print.Err(err)
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}
	confFile, err = config.ResolveImportFilePath(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}

	notify, err := getFlagBool(cmd, "notify")
	if err != nil {
		print.Err(err)
		return 1
	}

	cpus, err := getFlagInt(cmd, "jobs")
	if err != nil {
		print.Err(err)
		return 1
	}
	cpus = clampJobs(cpus)

	timeout, err := getTimeoutFlag(cmd)
	if err != nil {
		print.Err(err)
		return 1
	}

	if !fileutil.IsFile(confFile) {
		print.Err(fmt.Errorf("%s is not found", confFile))
		return 1
	}

	pkgs, err := config.ReadConfFile(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}

	if len(pkgs) == 0 {
		print.Err("unable to import package: no package information")
		return 1
	}

	print.Info("start import based on " + confFile)
	return installFromConfig(pkgs, dryRun, notify, cpus, timeout)
}

func installFromConfig(pkgs []goutil.Package, dryRun, notification bool, cpus int, timeout time.Duration) (exitCode int) {
	dryRunManager := goutil.NewGoPaths()

	if dryRun {
		if err := dryRunManager.StartDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change to dry run mode: %w", err))
			return 1
		}
		// Restore the environment and remove the temp dir via defer so it runs
		// even if a package install panics (see issue #297).
		defer func() {
			if err := dryRunManager.EndDryRunMode(); err != nil {
				print.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
				exitCode = 1
			}
		}()
	}

	installer := func(ctx context.Context, p goutil.Package) updateResult {
		ver, err := versionFromConfig(p)
		if err != nil {
			return updateResult{
				updated: false,
				pkg:     p,
				err:     fmt.Errorf("%s: %w", p.Name, err),
			}
		}
		if p.ImportPath == "" {
			return updateResult{
				updated: false,
				pkg:     p,
				err:     fmt.Errorf("%s: import path is empty", p.Name),
			}
		}

		// Store resolved version for display in the result loop
		if p.Version == nil {
			p.Version = &goutil.Version{}
		}
		p.Version.Current = ver

		if err := installByVersionCtx(ctx, p.ImportPath, ver); err != nil {
			return updateResult{
				updated: false,
				pkg:     p,
				err:     fmt.Errorf("%s: %w", p.Name, err),
			}
		}

		return updateResult{
			updated: true,
			pkg:     p,
			err:     nil,
		}
	}

	result, _ := executePackages(pkgs, cpus, timeout, installer, func(prefix string, v updateResult) {
		print.Info(fmt.Sprintf("%s %s@%s", prefix, v.pkg.ImportPath, v.pkg.Version.Current))
	})

	desktopNotifyIfNeeded(result, notification)
	return result
}

func versionFromConfig(pkg goutil.Package) (string, error) {
	if pkg.Version == nil {
		return "", errors.New("version is missing in gup.json")
	}
	ver := strings.TrimSpace(pkg.Version.Current)
	if ver == "" {
		return "", errors.New("version is empty in gup.json")
	}
	if ver == develVersionParen || ver == develVersion {
		return latestKeyword, nil
	}
	return ver, nil
}
