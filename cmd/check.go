package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the latest version of the binary installed by 'go install'",
	Long: `Check the latest version of the binary installed by 'go install'

check subcommand gets the latest version of the binary. However, do not update`,
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

	print.Info("check all binary under $GOPATH/bin or $GOBIN")
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
			i+1, len(pkgs), v.ModulePath, v.CurrentToLatestStr()))
	}
	return result
}
