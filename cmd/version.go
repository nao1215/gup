package cmd

import (
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               cmdNameVersion,
		Short:             "Show " + cmdinfo.Name + " command version information",
		Example:           "  gup version",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printerFor(cmd)
			p.Info(cmdinfo.GetVersion())
			return p.OutputError()
		},
	}
}
