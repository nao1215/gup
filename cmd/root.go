package cmd

import (
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/completion"
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
	assets.DeployIconIfNeeded()
	completion.DeployShellCompletionFileIfNeeded(rootCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}
