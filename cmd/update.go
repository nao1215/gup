package cmd

import (
	"fmt"
	"os"

	"github.com/cheggaaa/pb/v3"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/nao1215/gup/internal/slice"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update binaries installed by 'go install'",
	Long: `Update binaries installed by 'go install'

If you execute '$ gup update', gup gets the package path of all commands
under $GOPATH/bin and automatically updates commans to the latest version.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(gup(cmd, args))
	},
}

func init() {
	updateCmd.Flags().BoolP("dry-run", "d", false, "perform the trial update with no changes")
	updateCmd.Flags().StringSliceP("file", "f", []string{}, "specify binary name to be update (e.g.:--file=subaru,gup,go)")
	rootCmd.AddCommand(updateCmd)
}

// gup is main sequence.
// All errors are handled in this function.
func gup(cmd *cobra.Command, args []string) int {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can not parse command line argument (--dry-run)", err))
	}

	targets, err := cmd.Flags().GetStringSlice("file")
	if err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "can not parse command line argument (--file)", err))
	}

	if err := goutil.CanUseGoCmd(); err != nil {
		print.Fatal(fmt.Errorf("%s: %w", "you didn't install golang", err))
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Fatal(err)
	}
	pkgs = extractUserSpecifyPkg(pkgs, targets)

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
		if !dryRun {
			if v.ImportPath == "" {
				result[v.Name] = "Failure"
				bar.Increment()
				continue
			}
			if err := goutil.Install(v.ImportPath); err != nil {
				result[v.ImportPath] = "Failure"
				bar.Increment()
				continue
			}
		}
		result[v.ImportPath] = "Success"
		bar.Increment()
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

func extractUserSpecifyPkg(pkgs []goutil.Package, targets []string) []goutil.Package {
	result := []goutil.Package{}
	tmp := []string{}
	if len(targets) == 0 {
		return pkgs
	}

	for _, v := range pkgs {
		if slice.Contains(targets, v.Name) {
			result = append(result, v)
			tmp = append(tmp, v.Name)
		}
	}

	if len(tmp) != len(targets) {
		for _, target := range targets {
			if !slice.Contains(tmp, target) {
				print.Warn("not found '" + target + "' package in $GOPATH/bin or $GOBIN")
			}
		}
	}
	return result
}
