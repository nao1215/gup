package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the latest version of the binary installed by 'go install'",
		Long: `Check the latest version and build toolchain of the binary installed by 'go install'

check subcommand checks if the binary is the latest version
and if it has been built with the current version of go installed,
and displays the name of the binary that needs to be updated.
It does not update them.`,
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(check(cmd, args))
		},
	}

	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}
	cmd.Flags().Bool("ignore-go-update", false, "Ignore updates to the Go toolchain")
	addTimeoutFlag(cmd)

	return cmd
}

func check(cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	cpus, err := getFlagInt(cmd, "jobs")
	if err != nil {
		print.Err(err)
		return 1
	}
	cpus = clampJobs(cpus)

	ignoreGoUpdate, err := getFlagBool(cmd, "ignore-go-update")
	if err != nil {
		print.Err(err)
		return 1
	}

	timeout, err := getTimeoutFlag(cmd)
	if err != nil {
		print.Err(err)
		return 1
	}

	pkgs, err := getPackageInfoByTargets(args)
	if err != nil {
		print.Err(err)
		return 1
	}
	pkgs = extractUserSpecifyPkg(pkgs, args)

	if len(pkgs) == 0 {
		print.Err("unable to check package: no package information")
		return 1
	}

	pkgs = applyCheckChannels(pkgs)
	return doCheck(pkgs, cpus, timeout, ignoreGoUpdate)
}

// applyCheckChannels resolves the saved per-package update channel from
// gup.json so that 'gup check' compares each binary against the same source
// 'gup update' would install from. The config is located with the same
// resolution rules used by import/update; when no config exists every package
// keeps the default @latest behavior.
func applyCheckChannels(pkgs []goutil.Package) []goutil.Package {
	confReadPath, resolveErr := config.ResolveImportFilePath("")
	if resolveErr != nil {
		// check only reads the config for channel hints and is not the command
		// targeted by the ambiguity check, so fall back to the user-level config
		// instead of failing.
		confReadPath = config.FilePath()
	}

	confPkgs, err := readConfFileIfExists(confReadPath)
	if err != nil {
		print.Warn(fmt.Sprintf("failed to read %s: %s (continuing without config)", confReadPath, err))
		confPkgs = []goutil.Package{}
	}

	return applySavedChannels(pkgs, confPkgs)
}

func doCheck(pkgs []goutil.Package, cpus int, timeout time.Duration, ignoreGoUpdate bool) int {
	var mu sync.Mutex
	needUpdatePkgs := []goutil.Package{}
	verCache := newLatestVerCache()

	print.Info("check binary under $GOPATH/bin or $GOBIN")

	checker := func(ctx context.Context, p goutil.Package) updateResult {
		var err error
		if p.ModulePath == "" {
			err = fmt.Errorf("%s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			var latestVer string
			modulePathChanged := false
			latestVer, err = verCache.getByChannel(ctx, p.ModulePath, p.UpdateChannel)
			if err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					err = fmt.Errorf("%s %w", p.Name, err)
				} else {
					modulePathChanged = true
					p = newPkg
					latestVer, err = verCache.getByChannel(ctx, p.ModulePath, p.UpdateChannel)
					if err != nil {
						err = fmt.Errorf("%s %w", p.Name, err)
					}
				}
			}
			if err == nil {
				p.Version.Latest = latestVer

				shouldUpdate := modulePathChanged || !p.IsPackageUpToDate() || (!ignoreGoUpdate && !p.IsGoUpToDate())
				if shouldUpdate {
					mu.Lock()
					needUpdatePkgs = append(needUpdatePkgs, p)
					mu.Unlock()
				}
			}
		}

		return updateResult{
			pkg: p,
			err: err,
		}
	}

	result := executePackages(pkgs, cpus, timeout, checker, func(prefix string, v updateResult) {
		print.Info(fmt.Sprintf("%s %s (%s)", prefix, v.pkg.ImportPath, v.pkg.VersionCheckResultStr()))
	})

	printUpdatablePkgInfo(needUpdatePkgs)
	return result
}

func printUpdatablePkgInfo(pkgs []goutil.Package) {
	if len(pkgs) == 0 {
		return
	}

	var b strings.Builder
	for _, v := range pkgs {
		b.WriteString(v.Name)
		b.WriteString(" ")
	}

	const indentSpaces = 11
	fmt.Println("")
	print.Info("If you want to update binaries, run the following command.\n" +
		strings.Repeat(" ", indentSpaces) +
		"$ gup update " + b.String())
}
