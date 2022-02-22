package cmd

import (
	"fmt"
	"os"

	"github.com/cheggaaa/pb/v3"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gup",
	Short: "gup command update binaries installed by 'go install'",
	Long: `gup command update binaries installed by 'go install'

If you execute '$ gup', gup gets the package path of all commands
under $GOPATH/bin and automatically updates commans to the latest
version.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(gup(cmd, args))
	},
}

func init() {
	rootCmd.Flags().BoolP("dry-run", "d", false, "perform the trial update with no changes")
}

// Execute run gup process.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}

// gup is main sequence.
// All errors are handled in this function.
func gup(cmd *cobra.Command, args []string) int {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can not parse command line argument", err))
	}

	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}

	if len(pkgs) == 0 {
		print.Fatal("unable to update package: no package information")
	}

	result := update(pkgs, dryRun)
	print.InstallResult(result)

	return 0
}

func update(pkgs []goutil.Package, dryRun bool) map[string]string {
	result := map[string]string{}
	bar := pb.Simple.Start(len(pkgs))
	bar.SetMaxWidth(80)
	for _, v := range pkgs {
		bar.Increment()
		if !dryRun {
			if v.ImportPath == "" {
				result[v.Name] = "Failure"
				continue
			}
			if err := goutil.Install(v.ImportPath); err != nil {
				result[v.ImportPath] = "Failure"
				continue
			}
		}
		result[v.ImportPath] = "Success"
	}
	bar.Finish()
	return result
}

func getPackageInfo() ([]goutil.Package, error) {
	goBin, err := goutil.GoBin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't find installed binaries", err)
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err)
	}

	return goutil.GetPackageInformation(binList), nil
}
