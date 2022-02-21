package cmd

import (
	"fmt"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmdinfo.Version())
	},
	Short: "Show version information",
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
