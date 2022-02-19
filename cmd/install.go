package cmd

import (
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use: "install",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(install(args))
	},
	Short: "wrapper of `go install`.",
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func install(args []string) int {
	if len(args) == 0 {
		print.Fatal("not specify package (command) path. usage is same as 'go install'")
	}

	var err error
	pkgInfoFromConf := []goutil.Package{}
	if file.IsFile(config.FilePath()) {
		pkgInfoFromConf, err = config.ReadConfFile()
		if err != nil {
			print.Warn(err)
		}
	}

	goBin, err := goutil.GoBin()
	if err != nil {
		print.Fatal("not specify package (command) path. usage is same as 'go install'")
	}
	binListBeforeInstall, _ := goutil.BinaryPathList(goBin)

	path := deleteVersion(args[0])
	print.Info("Start installing: " + path)
	if err := goutil.Install(path); err != nil {
		print.Fatal(err)
	}
	print.Info("Complete installation: " + path)

	binListAfterInstall, err := goutil.BinaryPathList(goBin)
	if err != nil {
		print.Fatal("can't find installed binary at $GOPATH/bin")
	}

	pkg := goutil.Package{
		Name:       installedBinName(binListBeforeInstall, binListAfterInstall, path),
		ImportPath: path,
	}
	if pkg.Name == "" {
		print.Warn("can't record 'binary name' and 'package (command) path' in the " + config.FilePath())
		return 0
	}

	if err := config.WriteConfFile(updatePkgInfo(pkgInfoFromConf, pkg)); err != nil {
		print.Err(err)
		return 1
	}

	return 0
}

func updatePkgInfo(fromConf []goutil.Package, new goutil.Package) []goutil.Package {
	result := []goutil.Package{}

	update := false
	for _, v := range fromConf {
		if v.Name == new.Name {
			v.ImportPath = new.ImportPath
			update = true
		}
		result = append(result, v)
	}
	if !update {
		result = append(result, new)
	}
	return result
}

func deleteVersion(importPath string) string {
	r := regexp.MustCompile(`@.*`)
	return r.ReplaceAllString(importPath, "")
}

func installedBinName(binListBeforeInstall, binListAfterInstall []string, importPaht string) string {
	for _, a := range binListAfterInstall {
		if !contains(binListBeforeInstall, a) {
			return a
		}
	}

	for _, a := range binListAfterInstall {
		if strings.Contains(importPaht, a) {
			return a
		}
	}
	return ""
}

func contains(list interface{}, elem interface{}) bool {
	rvList := reflect.ValueOf(list)

	if rvList.Kind() == reflect.Slice {
		for i := 0; i < rvList.Len(); i++ {
			item := rvList.Index(i).Interface()
			if !reflect.TypeOf(elem).ConvertibleTo(reflect.TypeOf(item)) {
				continue
			}
			target := reflect.ValueOf(elem).Convert(reflect.TypeOf(item)).Interface()
			if ok := reflect.DeepEqual(item, target); ok {
				return true
			}
		}
	}
	return false
}
