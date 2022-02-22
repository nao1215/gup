package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the binary names under $GOPATH/bin and their path info. to gup.conf.",
	Long: `Export the binary names under $GOPATH/bin and their path info. to gup.conf.

Use the export subcommand if you want to install the same golang
binaries across multiple systems. By default, this sub-command 
exports the file to $HOME/.config/gup/gup.conf. After you have 
placed gup.conf in the same path hierarchy on another system,
you execute import subcommand. gup start the installation 
according to the contents of gup.conf.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(export(cmd, args))
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func export(cmd *cobra.Command, args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}
	pkgs = validPkgInfo(pkgs)

	if len(pkgs) == 0 {
		print.Fatal("no package information")
	}

	if err := config.WriteConfFile(pkgs); err != nil {
		print.Err(err)
		return 1
	}
	print.Info("Export " + config.FilePath())
	return 0
}

func validPkgInfo(pkgs []goutil.Package) []goutil.Package {
	result := []goutil.Package{}
	for _, v := range pkgs {
		if v.ImportPath == "" {
			print.Warn("can't get '" + v.Name + "'package path information. old go version binary")
			continue
		}
		result = append(result, goutil.Package{Name: v.Name, ImportPath: v.ImportPath})
	}
	return result
}
