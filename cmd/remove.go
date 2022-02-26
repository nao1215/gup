package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the binary under $GOPATH/bin or $GOBIN",
	Long: `Remove command in $GOPATH/bin or $GOBIN.
If you want to specify multiple binaries at once, separate them with space.
[e.g.] gup remove a_cmd b_cmd c_cmd`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(remove(cmd, args))
	},
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Forcibly remove the file")
	rootCmd.AddCommand(removeCmd)
}

func remove(cmd *cobra.Command, args []string) int {
	if len(args) == 0 {
		print.Fatal("No command name specified")
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can not parse command line argument (--force)", err))
	}

	gobin, err := goutil.GoBin()
	if err != nil {
		print.Fatal(err)
	}

	result := 0
	for _, v := range args {
		target := filepath.Join(gobin, v)
		if !file.IsFile(target) {
			print.Err(fmt.Errorf("no such file or directory: %s", target))
			result = 1
			continue
		}
		if !force {
			if !print.Question(fmt.Sprintf("remove %s?", target)) {
				continue
			}
		}

		if err := os.Remove(target); err != nil {
			print.Err(err)
			continue
		}
		print.Info("removed " + target)
	}
	return result
}
