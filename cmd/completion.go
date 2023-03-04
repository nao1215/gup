package cmd

import (
	"github.com/nao1215/gup/internal/completion"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Create shell completion files (bash, fish, zsh) for the gup",
	Long: `Create shell completion files (bash, fish, zsh) for the gup command
if it is not already on the system`,
	Run: func(cmd *cobra.Command, args []string) {
		completion.DeployShellCompletionFileIfNeeded(rootCmd)
	},
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Create shell completion files (bash, fish, zsh) for the gup",
		Long: `Create shell completion files (bash, fish, zsh) for the gup command
if it is not already on the system`,
		Run: func(cmd *cobra.Command, args []string) {
			completion.DeployShellCompletionFileIfNeeded(rootCmd)
		},
	}
}
