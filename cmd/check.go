package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the latest version of the binary installed by 'go install'",
		Long: `Check the latest version of the binary installed by 'go install'

check subcommand checks if the binary is the latest version
and displays the name of the binary that needs to be updated.
However, do not update`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(check(cmd, args))
		},
	}

	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}

	return cmd
}

func check(cmd *cobra.Command, args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Err(fmt.Errorf("%s: %w", "you didn't install golang", err))
		return 1
	}

	cpus, err := cmd.Flags().GetInt("jobs")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--jobs)", err))
		return 1
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Err(err)
		return 1
	}
	pkgs = extractUserSpecifyPkg(pkgs, args)

	if len(pkgs) == 0 {
		print.Err("unable to check package: no package information")
		return 1
	}
	return doCheck(pkgs, cpus)
}

func doCheck(pkgs []goutil.Package, cpus int) int {
	result := 0
	countFmt := "[%" + pkgDigit(pkgs) + "d/%" + pkgDigit(pkgs) + "d]"
	var mu sync.Mutex
	needUpdatePkgs := []goutil.Package{}

	print.Info("check binary under $GOPATH/bin or $GOBIN")
	ch := make(chan updateResult)
	weighted := semaphore.NewWeighted(int64(cpus))
	checker := func(ctx context.Context, p goutil.Package, result chan updateResult) {
		if err := weighted.Acquire(ctx, 1); err != nil {
			r := updateResult{
				pkg: p,
				err: err,
			}
			result <- r
			return
		}
		defer weighted.Release(1)

		var err error
		if p.ModulePath == "" {
			err = fmt.Errorf(" %s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			var latestVer string
			latestVer, err = goutil.GetLatestVer(p.ModulePath)
			if err != nil {
				err = fmt.Errorf(" %s %w", p.Name, err)
			}
			p.Version.Latest = latestVer
			if !goutil.IsAlreadyUpToDate(*p.Version) {
				mu.Lock()
				needUpdatePkgs = append(needUpdatePkgs, p)
				mu.Unlock()
			}
		}

		r := updateResult{
			pkg: p,
			err: err,
		}
		result <- r
	}

	// check all package
	ctx := context.Background()
	for _, v := range pkgs {
		go checker(ctx, v, ch)
	}

	// print result
	for i := 0; i < len(pkgs); i++ {
		v := <-ch
		if v.err == nil {
			print.Info(fmt.Sprintf(countFmt+" %s (%s)",
				i+1, len(pkgs), v.pkg.ModulePath, v.pkg.VersionCheckResultStr()))
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

	var p string
	for _, v := range pkgs {
		p += v.Name + " "
	}
	fmt.Println("")
	print.Info("If you want to update binaries, run the following command.\n" +
		strings.Repeat(" ", 11) +
		"$ gup update " + p)
}
