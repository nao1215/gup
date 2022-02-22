package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List up command name with package path and version under $GOPATH/bin or $GOBIN",
	Long:  `List up command name with package path and version under $GOPATH/bin or $GOBIN`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(list(cmd, args))
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func list(cmd *cobra.Command, args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}

	if len(pkgs) == 0 {
		print.Fatal("unable to list up package: no package information")
	}
	printPackageList(pkgs)

	return 0
}

// PackageList list up command package in $GOPATH/bin or $GOBIN
func printPackageList(pkgs []goutil.Package) {
	max := 0
	for _, v := range pkgs {
		if len(v.Name) > max {
			max = len(v.Name)
		}
	}

	for _, v := range pkgs {
		fmt.Fprintf(print.Stdout, "%"+strconv.Itoa(max)+"s: %s%s\n",
			v.Name,
			v.ImportPath,
			color.GreenString("@"+goutil.GetPackageVersion(v.Name)))
	}
}
