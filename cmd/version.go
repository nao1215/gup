package cmd

import (
	"fmt"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show " + cmdinfo.Name + " command version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmdinfo.GetVersion())
		},
	}
}
