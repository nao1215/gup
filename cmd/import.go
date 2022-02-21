package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use: "import",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(runImport())
	},
	Short: "Install command according to gup.conf",
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func runImport() int {
	if !file.IsFile(config.FilePath()) {
		print.Fatal(fmt.Errorf("%s is not found", config.FilePath()))
	}

	pkgs, err := config.ReadConfFile()
	if err != nil {
		print.Fatal(err)
	}

	if len(pkgs) == 0 {
		print.Fatal("unable to update package: no package information")
	}

	pkgs, result := update(pkgs)
	for k, v := range result {
		if v == "Failure" {
			print.Err(fmt.Errorf("update failure: %s ", k))
		} else {
			print.Info("update success: " + k)
		}
	}

	// Record only successfully installed packages in the config file
	if err := config.WriteConfFile(pkgs); err != nil {
		print.Err(err)
		return 1
	}
	return 0
}
