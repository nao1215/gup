package cmd

import (
	"fmt"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Install command according to gup.conf.",
	Long: `Install command according to gup.conf.

Use the export subcommand if you want to install the same golang
binaries across multiple systems. After you create gup.conf by 
import subcommand in another environment, you save conf-file in
$HOME/.config/gup/gup.conf.
Finally, you execute the export subcommand in this state.`,
	Run: func(cmd *cobra.Command, args []string) {
		OsExit(runImport(cmd, args))
	},
}

func init() {
	importCmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
	importCmd.Flags().StringP("input", "i", config.FilePath(), "specify gup.conf file path to import")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) int {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--dry-run)", err))
		return 1
	}

	confFile, err := cmd.Flags().GetString("input")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--input)", err))
		return 1
	}

	if !file.IsFile(confFile) {
		print.Err(fmt.Errorf("%s is not found", confFile))
		return 1
	}

	pkgs, err := config.ReadConfFile(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}

	if len(pkgs) == 0 {
		print.Err("unable to import package: no package information")
		return 1
	}

	print.Info("start update based on " + confFile)
	return update(pkgs, dryRun)
}
