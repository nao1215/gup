package cmd

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List command names with package path and version under $GOPATH/bin or $GOBIN",
		Long:              `List command names with package path and version under $GOPATH/bin or $GOBIN`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(list(cmd, args))
		},
	}
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	return cmd
}

func list(cmd *cobra.Command, _ []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Err(err)
		return 1
	}

	if len(pkgs) == 0 {
		print.Err("unable to list up package: no package information")
		return 1
	}

	jsonOut, err := getFlagBool(cmd, "json")
	if err != nil {
		print.Err(err)
		return 1
	}

	if jsonOut {
		pkgs = applyCheckChannels(pkgs)
		if err := encodeJSONPackages(listJSONRecords(pkgs)); err != nil {
			print.Err(err)
			return 1
		}
		return 0
	}

	printPackageList(pkgs)
	return 0
}

// listJSONRecords builds JSON records for 'list'. list only knows the installed
// (current) version, so latest/Go-version fields stay empty and status is
// reported as "installed".
func listJSONRecords(pkgs []goutil.Package) []jsonPackage {
	recs := make([]jsonPackage, 0, len(pkgs))
	for _, p := range pkgs {
		recs = append(recs, newJSONPackage(p, statusInstalled, nil))
	}
	return recs
}

// PackageList list up command package in $GOPATH/bin or $GOBIN
func printPackageList(pkgs []goutil.Package) {
	max := 0
	for _, v := range pkgs {
		if len(v.Name) > max {
			max = len(v.Name)
		}
	}

	for _, v := range pkgs {
		_, _ = fmt.Fprintf(print.Stdout, "%"+strconv.Itoa(max)+"s: %s%s\n",
			v.Name,
			v.ImportPath,
			color.GreenString("@"+v.Version.Current))
	}
}
