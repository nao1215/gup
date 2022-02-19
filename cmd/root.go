package cmd

import (
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gup",
	Short: "Update binaries installed by 'go install'",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(gup(args))
	},
}

// Execute run gup process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}

// gup is main sequence.
// All errors are handled in this function.
func gup(args []string) int {
	goBin, err := goutil.GoBin()
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can't find installed binaries", err))
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err))
	}

	pkgList, err := goutil.GetPackageInformation(binList)
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can't get package information from shell history", err))
	}

	for _, v := range pkgList {
		fmt.Printf("%s: %s\n", v.Name, v.ImportPath)
	}
	return 0
}
