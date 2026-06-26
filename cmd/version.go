package cmd

import (
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "Show " + cmdinfo.Name + " command version information",
		Example:           "  gup version",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			printerFor(cmd).Info(cmdinfo.GetVersion())
		},
	}
}
