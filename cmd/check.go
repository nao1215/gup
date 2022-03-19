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
	for i, v := range pkgs {
		if v.ModulePath == "" {
			print.Err(fmt.Errorf(countFmt+" check failure: %s",
				i+1, len(pkgs), v.Name))
			result = 1
			continue
		}

		latestVer, err := goutil.GetLatestVer(v.ModulePath)
		if err != nil {
			print.Err(fmt.Errorf(countFmt+" check failure: %w",
				i+1, len(pkgs), err))
			result = 1
			continue
		}
		v.Version.Latest = latestVer

		print.Info(fmt.Sprintf(countFmt+" check success: %s (%s)",
			i+1, len(pkgs), v.ModulePath, v.VersionCheckResultStr()))

		if !goutil.IsAlreadyUpToDate(*v.Version) {
			needUpdatePkgs = append(needUpdatePkgs, v)
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
