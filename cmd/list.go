package cmd

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List command names with package path and version under $GOPATH/bin or $GOBIN",
		Long:  `List command names with package path and version under $GOPATH/bin or $GOBIN`,
		Example: `  gup list
  gup list --json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(list(cmd, args))
		},
	}
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read saved update channels from (with --json)")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}
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

	jsonOut, err := getFlagBool(cmd, "json")
	if err != nil {
		print.Err(err)
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}

	if jsonOut {
		// An empty environment has no packages to annotate, so emit a valid
		// empty JSON array and exit 0 without resolving config (#350). An
		// explicitly named --file is still validated for consistency with
		// check/update (#368).
		if len(pkgs) == 0 {
			if err := configstate.ValidateExplicitFile(confFile); err != nil {
				print.Err(err)
				return 1
			}
			if err := encodeJSONPackages(listJSONRecords(nil)); err != nil {
				print.Err(err)
				return 1
			}
			return 0
		}
		// Use the same config-resolution rules as check/update/import so the
		// reported channel matches what those commands would actually use, and
		// fail fast on an ambiguous or malformed config instead of silently
		// picking the user-level one (#364).
		annotated, cerr := configstate.ResolveAndApplyChannels(pkgs, confFile)
		if cerr != nil {
			print.Err(cerr)
			return 1
		}
		if err := encodeJSONPackages(listJSONRecords(annotated)); err != nil {
			print.Err(err)
			return 1
		}
		return 0
	}

	// An empty-but-valid environment is a normal first-run condition, not an
	// error (#350).
	if len(pkgs) == 0 {
		print.Info(emptyEnvMessage)
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

// PackageList list up command package in $GOPATH/bin or $GOBIN.
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
