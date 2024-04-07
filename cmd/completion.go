package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/completion"
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completions (bash, fish, zsh) for gup",
		Long: `Generate shell completions (bash, fish, zsh) for the gup command.
With no arguments, generate files to the file system if they are not already there,
with shell name as argument, output completion for the shell to standard output.`,
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "fish", "zsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rootCmd := newRootCmd()
			if len(args) == 0 {
				completion.DeployShellCompletionFileIfNeeded(rootCmd)
				return nil
			}
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletionV2(os.Stdout, false)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, false)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			default:
				return fmt.Errorf("internal error, should not happen with arg %q", args[0])
			}
		},
	}
}
