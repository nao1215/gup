package cmd

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

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
		Short: "Install command according to gup.json.",
		Long: `Install command according to gup.json.
	
Use export/import if you want to install the same golang binaries
across multiple systems.
First, run 'gup export' on the source environment and copy gup.json.
Then run 'gup import' on the target environment to install the
versions recorded in that gup.json.`,
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
	confFile = config.ResolveImportFilePath(confFile)

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
	return installFromConfig(pkgs, dryRun, notify, cpus)
}

func installFromConfig(pkgs []goutil.Package, dryRun, notification bool, cpus int) int {
	result := 0
	countFmt := "[%" + pkgDigit(pkgs) + "d/%" + pkgDigit(pkgs) + "d]"
	dryRunManager := goutil.NewGoPaths()
	ctx, cancel, signals := newSignalCancelContext()
	defer stopSignalCancelContext(cancel, signals)

	if dryRun {
		if err := dryRunManager.StartDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change to dry run mode: %w", err))
			return 1
		}
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

	ch := forEachPackage(ctx, pkgs, cpus, installer)

	count := 0
	for v := range ch {
		if v.err == nil {
			print.Info(fmt.Sprintf(countFmt+" %s@%s", count+1, len(pkgs), v.pkg.ImportPath, v.pkg.Version.Current))
		} else {
			result = 1
			print.Err(fmt.Errorf(countFmt+" %s", count+1, len(pkgs), v.err.Error()))
		}
		count++
		if count == len(pkgs) {
			break
		}
	}

	if dryRun {
		if err := dryRunManager.EndDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
			return 1
		}
	}

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
