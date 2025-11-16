package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nao1215/gorky/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove the binary under $GOPATH/bin or $GOBIN",
		Long: `Remove command in $GOPATH/bin or $GOBIN.
If you want to specify multiple binaries at once, separate them with space.
[e.g.] gup remove a_cmd b_cmd c_cmd`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(remove(cmd, args))
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Forcibly remove the file")

	return cmd
}

func remove(cmd *cobra.Command, args []string) int {
	if len(args) == 0 {
		print.Err("no command name specified")
		return 1
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not parse command line argument (--force)", err))
		return 1
	}

	gobin, err := goutil.GoBin()
	if err != nil {
		print.Err(err)
		return 1
	}

	return removeLoop(gobin, force, args)
}

const goosWindows = "windows"

// GOOS is wrapper for runtime.GOOS variable. It's for unit test.
var GOOS = runtime.GOOS //nolint:gochecknoglobals

func removeLoop(gobin string, force bool, target []string) int {
	result := 0
	for _, v := range target {
		// In Windows, $GOEXE is set to the ".exe" extension.
		// The user-specified command name (arguments) may not have an extension.
		execSuffix := os.Getenv("GOEXE")
		if GOOS == goosWindows && !strings.HasSuffix(v, execSuffix) {
			v += execSuffix
		}

		target := filepath.Join(gobin, v)
		if !file.IsFile(target) {
			print.Err(fmt.Errorf("no such file or directory: %s", target))
			result = 1
			continue
		}
		if !force {
			if !print.Question(fmt.Sprintf("remove %s?", target)) {
				print.Info("cancel removal " + target)
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
