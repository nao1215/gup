// Package cmd define subcommands provided by the gup command
package cmd

import (
	"os"
	"runtime"
	"strconv"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/completion"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// OsExit is wrapper for  os.Exit(). It's for unit test.
var OsExit = os.Exit //nolint:gochecknoglobals

// printerFor builds a Printer from a command's output streams. It uses
// cmd.OutOrStdout()/ErrOrStderr() so output flows through cobra's configurable
// writers: production wires the colorable process streams onto the root command
// in Execute, while a test can capture output by calling SetOut/SetErr with a
// buffer instead of redirecting the process-wide os.Stdout/os.Stderr.
func printerFor(cmd *cobra.Command) *print.Printer {
	return print.New(cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "gup",
		Short: `gup updates binaries installed by 'go install'.
If you update all binaries, just run '$ gup update'`,
		Long: `gup updates binaries installed by "go install" to the latest version.

gup updates all binaries in parallel, so it is very fast. It also provides
subcommands for manipulating binaries under $GOPATH/bin ($GOBIN).
gup is cross-platform software that runs on Windows, Mac and Linux.

If you are using oh-my-zsh, then gup has an alias set up. The alias
is gup - git pull --rebase. Therefore, please make sure that the
oh-my-zsh alias is disabled (e.g. $ \gup update).

If you find gup useful, please consider sponsoring the project:
  https://github.com/sponsors/nao1215
`,
		Example: `  gup update
  gup update --dry-run
  gup list`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			noColor, err := getFlagBool(cmd, noColorFlagName)
			if err != nil {
				return err
			}
			applyColorPreference(noColor)
			return nil
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	addNoColorFlag(cmd)

	// Support the standard top-level "gup --version" / "gup -V" while keeping
	// the "gup version" subcommand. The template reuses cmdinfo.GetVersion() so
	// both paths print identical output (issue #325).
	cmd.Version = cmdinfo.GetVersion()
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.Flags().BoolP("version", "V", false, "version for gup")

	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newPinCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newUnpinCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newBugReportCmd())

	if !completion.IsWindows() {
		cmd.AddCommand(newManCmd())
	}

	return cmd
}

// Execute run gup process.
func Execute() error {
	rootCmd := newRootCmd()
	// Wire the colorable process streams onto the root command. cobra propagates
	// these to every subcommand (OutOrStdout/ErrOrStderr walk up to the parent),
	// so printerFor builds production Printers over them while tests can override
	// with SetOut/SetErr.
	rootCmd.SetOut(print.ColorableStdout())
	rootCmd.SetErr(print.ColorableStderr())
	return rootCmd.Execute()
}

// completeNCPUs returns the number of CPU cores as a string.
func completeNCPUs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	n := runtime.NumCPU()
	ret := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ret = append(ret, strconv.FormatInt(int64(i), 10))
	}
	return ret, cobra.ShellCompDirectiveNoFileComp
}
