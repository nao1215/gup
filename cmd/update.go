package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/notify"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update binaries installed by 'go install'",
		Long: `Update binaries installed by 'go install'

If you execute '$ gup update', gup gets the package path of all commands
under $GOPATH/bin and automatically updates commands to the latest version,
using the current installed Go toolchain.`,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(gup(cmd, args))
		},
		ValidArgsFunction: completePathBinaries,
	}
	cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
	cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
	cmd.Flags().StringSliceP("exclude", "e", []string{}, "specify binaries which should not be updated (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc("exclude", completePathBinaries); err != nil {
		panic(err)
	}
	cmd.Flags().StringSliceP("main", "m", []string{}, "specify binaries which update by @main or @master (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc("main", completePathBinaries); err != nil {
		panic(err)
	}
	// cmd.Flags().BoolP("main-all", "M", false, "update all binaries by @main or @master (delimiter: ',')")
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}

	return cmd
}

// gup is main sequence.
// All errors are handled in this function.
func gup(cmd *cobra.Command, args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Err(fmt.Errorf("%s: %w", "you didn't install golang", err))
		return 1
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--dry-run)", err))
		return 1
	}

	notify, err := cmd.Flags().GetBool("notify")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--notify)", err))
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

	excludePkgList, err := cmd.Flags().GetStringSlice("exclude")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--exclude)", err))
		return 1
	}

	mainPkgNames, err := cmd.Flags().GetStringSlice("main")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--main)", err))
		return 1
	}

	pkgs = extractUserSpecifyPkg(pkgs, args)
	pkgs = excludePkgs(excludePkgList, pkgs)

	if len(pkgs) == 0 {
		print.Err("unable to update package: no package information or no package under $GOBIN")
		return 1
	}
	return update(pkgs, dryRun, notify, cpus, mainPkgNames)
}

func excludePkgs(excludePkgList []string, pkgs []goutil.Package) []goutil.Package {
	packageList := []goutil.Package{}
	for _, v := range pkgs {
		if slices.Contains(excludePkgList, v.Name) {
			print.Info(fmt.Sprintf("Exclude '%s' from the update target", v.Name))
			continue
		}
		packageList = append(packageList, v)
	}
	return packageList
}

type updateResult struct {
	pkg goutil.Package
	err error
}

// update updates all packages.
// If dryRun is true, it does not update.
// If notification is true, it notifies the result of update.
func update(pkgs []goutil.Package, dryRun, notification bool, cpus int, mainPkgNames []string) int {
	result := 0
	countFmt := "[%" + pkgDigit(pkgs) + "d/%" + pkgDigit(pkgs) + "d]"
	dryRunManager := goutil.NewGoPaths()

	print.Info("update binary under $GOPATH/bin or $GOBIN")
	signals := make(chan os.Signal, 1)
	if dryRun {
		if err := dryRunManager.StartDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change to dry run mode: %w", err))
			notify.Warn("gup", "Can not change to dry run mode")
			return 1
		}
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP,
			syscall.SIGQUIT, syscall.SIGABRT)
		go catchSignal(signals, dryRunManager)
	}

	ch := make(chan updateResult)
	weighted := semaphore.NewWeighted(int64(cpus))
	updater := func(ctx context.Context, p goutil.Package, result chan updateResult) {
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
		if p.ImportPath == "" {
			err = fmt.Errorf(" %s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			if slices.Contains(mainPkgNames, p.Name) {
				if err = goutil.InstallMainOrMaster(p.ImportPath); err != nil {
					err = fmt.Errorf(" %s %w", p.Name, err)
				}
			} else {
				if err = goutil.InstallLatest(p.ImportPath); err != nil {
					err = fmt.Errorf(" %s %w", p.Name, err)
				}
			}
		}

		p.SetLatestVer()
		r := updateResult{
			pkg: p,
			err: err,
		}
		result <- r
	}

	// update all package
	ctx := context.Background()
	for _, v := range pkgs {
		go updater(ctx, v, ch)
	}

	// print result
	for i := 0; i < len(pkgs); i++ {
		v := <-ch
		if v.err == nil {
			print.Info(fmt.Sprintf(countFmt+" %s (%s)",
				i+1, len(pkgs), v.pkg.ImportPath, v.pkg.CurrentToLatestStr()))
		} else {
			result = 1
			print.Err(fmt.Errorf(countFmt+"%s", i+1, len(pkgs), v.err.Error()))
		}
	}

	if dryRun {
		if err := dryRunManager.EndDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
			return 1
		}
		close(signals)
	}

	desktopNotifyIfNeeded(result, notification)

	return result
}

func desktopNotifyIfNeeded(result int, enable bool) {
	if enable {
		if result == 0 {
			notify.Info("gup", "All update success")
		} else {
			notify.Warn("gup", "Some package can't update")
		}
	}
}

func catchSignal(c chan os.Signal, dryRunManager *goutil.GoPaths) {
	for {
		select {
		case <-c:
			if err := dryRunManager.EndDryRunMode(); err != nil {
				print.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
			}
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func pkgDigit(pkgs []goutil.Package) string {
	return strconv.Itoa(len(strconv.Itoa(len(pkgs))))
}

func getBinaryPathList() ([]string, error) {
	goBin, err := goutil.GoBin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't find installed binaries", err)
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err)
	}

	return binList, nil
}

func getPackageInfo() ([]goutil.Package, error) {
	binList, err := getBinaryPathList()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	return goutil.GetPackageInformation(binList), nil
}

func extractUserSpecifyPkg(pkgs []goutil.Package, targets []string) []goutil.Package {
	result := []goutil.Package{}
	tmp := []string{}
	if len(targets) == 0 {
		return pkgs
	}

	if runtime.GOOS == "windows" {
		for i, target := range targets {
			if strings.HasSuffix(strings.ToLower(target), ".exe") {
				targets[i] = strings.TrimSuffix(strings.ToLower(target), ".exe")
			}
		}
	}

	for _, v := range pkgs {
		pkg := v.Name
		if runtime.GOOS == "windows" {
			if strings.HasSuffix(strings.ToLower(pkg), ".exe") {
				pkg = strings.TrimSuffix(strings.ToLower(pkg), ".exe")
			}
		}
		if slices.Contains(targets, pkg) {
			result = append(result, v)
			tmp = append(tmp, pkg)
		}
	}

	if len(tmp) != len(targets) {
		for _, target := range targets {
			if !slices.Contains(tmp, target) {
				print.Warn("not found '" + target + "' package in $GOPATH/bin or $GOBIN")
			}
		}
	}
	return result
}

func completePathBinaries(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	binList, _ := getBinaryPathList()
	for i, b := range binList {
		binList[i] = filepath.Base(b)
	}
	return binList, cobra.ShellCompDirectiveNoFileComp
}
