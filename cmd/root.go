package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gup",
	Short: "Update binaries installed by 'go install'",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(gup(args))
	},
}

// Execute run gup process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}

// gup is main sequence.
// All errors are handled in this function.
func gup(args []string) int {
	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}

	for _, v := range pkgs {
		if v.ImportPath == "" {
			print.Warn(v.Name + " does not know the import path")
			continue
		}

		print.Info("Start installing " + v.Name)
		goutil.Install(v.ImportPath)
	}

	if err := config.WriteConfFile(pkgs); err != nil {
		print.Err(err)
		return 1
	}
	return 0
}

func getPackageInfo() ([]goutil.Package, error) {
	var err error
	pkgInfoFromConf := []goutil.Package{}

	if file.IsFile(config.FilePath()) {
		pkgInfoFromConf, err = config.ReadConfFile()
		if err != nil {
			print.Warn(err)
		}
	}

	goBin, err := goutil.GoBin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't find installed binaries", err)
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err)
	}

	pkgInfoFromShellHistory, err := goutil.GetPackageInformation(binList)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get package information from shell history", err)
	}

	pkgs := []goutil.Package{}
	for _, v := range pkgInfoFromShellHistory {
		pkg := goutil.Package{Name: v.Name, ImportPath: v.ImportPath}
		for _, p := range pkgInfoFromConf {
			if p.Name == v.Name && p.ImportPath != "" {
				pkg.ImportPath = p.ImportPath
			}
		}
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
}
