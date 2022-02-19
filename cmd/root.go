package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "gup",
}

// Execute run gup process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {

	}
}
