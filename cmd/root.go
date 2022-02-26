package cmd

import (
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "gup",
	Short: `gup command update binaries installed by 'go install'.
If you update all binaries, just run '$ gup update'`,
}

// Execute run gup process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}
