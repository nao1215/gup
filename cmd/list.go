package cmd

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "list",
		Short:             "List up command name with package path and version under $GOPATH/bin or $GOBIN",
		Long:              `List up command name with package path and version under $GOPATH/bin or $GOBIN`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(list(cmd, args))
		},
	}
}

func list(_ *cobra.Command, _ []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Err(fmt.Errorf("%s: %w", "you didn't install golang", err))
		return 1
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Err(err)
		return 1
	}

	if len(pkgs) == 0 {
		print.Err("unable to list up package: no package information")
		return 1
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
		_, _ = fmt.Fprintf(print.Stdout, "%"+strconv.Itoa(max)+"s: %s%s\n",
			v.Name,
			v.ImportPath,
			color.GreenString("@"+v.Version.Current))
	}
}
