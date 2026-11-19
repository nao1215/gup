package cmd

import (
	"errors"
	"fmt"

	"github.com/nao1215/gup/internal/completion"
	"github.com/spf13/cobra"
)

// shellBash is the bash shell name used for completion arguments.
const shellBash = "bash"

// deployCompletion installs the completion files for the current platform. It is
// a package variable so a test can drive the command's failure handling without
// depending on which shells the host actually has. Which shells get installed is
// the completion package's decision (bash/fish/zsh on POSIX, PowerShell on
// Windows), and is tested there.
var deployCompletion = completion.DeployShellCompletionFileIfNeeded //nolint:gochecknoglobals // test seam

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completions (bash, fish, zsh, powershell) for gup",
		Long: `Generate shell completions (bash, fish, zsh, powershell) for the gup command.
With a shell name as argument, output completion for the shell to standard output.

Use --install to set completion up for the shells of the current platform:
bash, fish and zsh on Linux and macOS, PowerShell on Windows. The PowerShell
install writes the completion script next to your profile and adds a single
dot-source line to the profile inside a marked block, so re-running it never
duplicates the entry and never disturbs the rest of your profile.`,
		Example: `  gup completion bash
  gup completion --install`,
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{shellBash, "fish", "zsh", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rootCmd := newRootCmd()
			install, err := getFlagBool(cmd, "install")
			if err != nil {
				return err
			}
			if install {
				if len(args) != 0 {
					return errors.New("--install cannot be used with shell argument")
				}
				return deployCompletion(rootCmd)
			}
			if len(args) == 0 {
				return argsGuidance(
					"requires a shell name (bash, fish, zsh, powershell) or --install",
					"gup completion bash",
					"gup completion --install")
			}
			// Write to the command's configured output (cobra propagates the root's
			// writer) so completion output flows through the same sink as the rest
			// of gup and tests can capture it without redirecting os.Stdout.
			out := cmd.OutOrStdout()
			switch args[0] {
			case shellBash:
				return rootCmd.GenBashCompletionV2(out, false)
			case "fish":
				return rootCmd.GenFishCompletion(out, false)
			case "zsh":
				return rootCmd.GenZshCompletion(out)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(out)
			default:
				return fmt.Errorf("internal error, should not happen with arg %q", args[0])
			}
		},
	}
	cmd.Flags().Bool("install", false, "install completion for this platform's shells (bash/fish/zsh, or PowerShell on Windows)")
	return cmd
}
