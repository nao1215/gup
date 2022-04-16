package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the latest version of the binary installed by 'go install'",
	Long: `Check the latest version of the binary installed by 'go install'

check subcommand checks if the binary is the latest version
and displays the name of the binary that needs to be updated.
However, do not update`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(check(cmd, args))
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func check(cmd *cobra.Command, args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}
	pkgs = extractUserSpecifyPkg(pkgs, args)

	if len(pkgs) == 0 {
		print.Fatal("unable to check package: no package information")
	}
	return doCheck(pkgs)
}

func doCheck(pkgs []goutil.Package) int {
	result := 0
	countFmt := "[%" + pkgDigit(pkgs) + "d/%" + pkgDigit(pkgs) + "d]"
	needUpdatePkgs := []goutil.Package{}

	print.Info("check binary under $GOPATH/bin or $GOBIN")
	ch := make(chan updateResult)
	checker := func(p goutil.Package, result chan updateResult) {
		var err error
		var latestVer string

		if p.ModulePath == "" {
			err = fmt.Errorf(" %s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {

			latestVer, err = goutil.GetLatestVer(p.ModulePath)
			if err != nil {
				err = fmt.Errorf(" %s %w", p.Name, err)
			}
		}
		p.Version.Latest = latestVer
		if !goutil.IsAlreadyUpToDate(*p.Version) {
			needUpdatePkgs = append(needUpdatePkgs, p)
		}

		r := updateResult{
			pkg: p,
			err: err,
		}
		result <- r
	}

	// check all package
	for _, v := range pkgs {
		go checker(v, ch)
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
