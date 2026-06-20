package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

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
However, do not update`,
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
	ctx, cancel, signals := newSignalCancelContext()
	defer stopSignalCancelContext(cancel, signals)
	return doCheck(ctx, pkgs, cpus, timeout, ignoreGoUpdate)
}

func doCheck(ctx context.Context, pkgs []goutil.Package, cpus int, timeout time.Duration, ignoreGoUpdate bool) int {
	result := 0
	countFmt := "[%" + pkgDigit(pkgs) + "d/%" + pkgDigit(pkgs) + "d]"
	var mu sync.Mutex
	needUpdatePkgs := []goutil.Package{}
	verCache := newLatestVerCache()

	print.Info("check binary under $GOPATH/bin or $GOBIN")

	checker := func(ctx context.Context, p goutil.Package) updateResult {
		var err error
		if p.ModulePath == "" {
			err = fmt.Errorf(" %s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			var latestVer string
			modulePathChanged := false
			latestVer, err = verCache.get(ctx, p.ModulePath)
			if err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					err = fmt.Errorf(" %s %w", p.Name, err)
				} else {
					modulePathChanged = true
					p = newPkg
					latestVer, err = verCache.get(ctx, p.ModulePath)
					if err != nil {
						err = fmt.Errorf(" %s %w", p.Name, err)
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

	ch := forEachPackage(ctx, pkgs, cpus, timeout, checker)

	// print result
	for i := 0; i < len(pkgs); i++ {
		v := <-ch
		if v.err == nil {
			print.Info(fmt.Sprintf(countFmt+" %s (%s)",
				i+1, len(pkgs), v.pkg.ImportPath, v.pkg.VersionCheckResultStr()))
		} else {
			result = 1
			print.Err(fmt.Errorf(countFmt+"%s", i+1, len(pkgs), v.err.Error()))
		}
	}

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
