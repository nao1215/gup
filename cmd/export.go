package cmd

import (
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export installed binaries and their versions under $GOPATH/bin or $GOBIN to gup.json.",
		Long: `Export installed binaries and their versions under $GOPATH/bin or $GOBIN to gup.json.

Use export/import if you want to install the same Go binaries
across multiple systems. This sub-command writes gup.json
(default: $XDG_CONFIG_HOME/gup/gup.json), and the target system can
apply it with 'gup import'.`,
		Example: `  gup export
  gup export --output > gup.json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(export(cmd, args))
		},
	}
	cmd.Flags().BoolP("output", "o", false, "print command path information at STDOUT")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to export")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}

	return cmd
}

func export(cmd *cobra.Command, _ []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	output, err := getFlagBool(cmd, "output")
	if err != nil {
		print.Err(err)
		return 1
	}
	configPath, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}
	configPath = config.ResolveExportFilePath(configPath)

	pkgs, err := getPackageInfo()
	if err != nil {
		print.Err(err)
		return 1
	}
	pkgs = validPkgInfo(pkgs)
	confPkgs, err := readConfFileIfExists(configPath)
	if err != nil {
		print.Warn("failed to read " + configPath + ": " + err.Error())
		confPkgs = []goutil.Package{}
	}
	pkgs = applySavedChannels(pkgs, confPkgs)

	if len(pkgs) == 0 {
		print.Err("no package information")
		return 1
	}

	if output {
		err = outputConfig(pkgs)
	} else {
		err = writeConfigFile(configPath, pkgs)
	}
	if err != nil {
		print.Err(err)
		return 1
	}
	if !output {
		print.Info("Export " + configPath)
	}
	return 0
}

func outputConfig(pkgs []goutil.Package) error {
	return config.WriteConfFile(print.Stdout, pkgs)
}

func validPkgInfo(pkgs []goutil.Package) []goutil.Package {
	result := []goutil.Package{}
	for _, v := range pkgs {
		if v.ImportPath == "" {
			print.Warn("can't get '" + v.Name + "' package path information. old go version binary")
			continue
		}
		result = append(result, goutil.Package{Name: v.Name, ImportPath: v.ImportPath, Version: v.Version})
	}
	return result
}

func applySavedChannels(pkgs, confPkgs []goutil.Package) []goutil.Package {
	channelByName := make(map[string]goutil.UpdateChannel, len(confPkgs))
	for _, p := range confPkgs {
		channelByName[p.Name] = goutil.NormalizeUpdateChannel(string(p.UpdateChannel))
	}

	result := make([]goutil.Package, 0, len(pkgs))
	for _, p := range pkgs {
		channel, ok := channelByName[p.Name]
		if !ok {
			channel = goutil.UpdateChannelLatest
		}
		p.UpdateChannel = channel
		result = append(result, p)
	}
	return result
}
