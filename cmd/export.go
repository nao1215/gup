package cmd

import (
	"strings"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export installed binaries and their versions under $GOPATH/bin or $GOBIN to gup.json",
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
	// The source of truth for saved channels is always the canonical user-level
	// config; --file/--output only change the export destination, not where
	// channels are read from (#341).
	channelSource := config.FilePath()
	// A malformed channel-source config must fail fast instead of silently
	// exporting every package as @latest, which would drop intentionally pinned
	// channels such as @main from the written gup.json (#369).
	confPkgs, err := readConfFileIfExists(channelSource)
	if err != nil {
		print.Err(err)
		return 1
	}
	pkgs = applySavedChannels(pkgs, confPkgs)

	// An empty-but-valid environment is a normal first-run condition, not an
	// error (#350): export still succeeds and writes an empty configuration.
	if len(pkgs) == 0 {
		print.Warn(emptyEnvMessage + "; exporting an empty configuration")
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

// applySavedChannels copies each package's saved update channel from confPkgs.
// Matching prefers import_path (the stable identifier) and falls back to the
// binary name normalized by normalizeBinName (trimmed, with any case-insensitive
// ".exe"/".EXE" suffix stripped), so a channel is not silently reset to @latest
// across binary renames, hand-edited configs, or cross-OS name differences
// (#341). Packages with no saved entry default to @latest.
func applySavedChannels(pkgs, confPkgs []goutil.Package) []goutil.Package {
	channelByImportPath := make(map[string]goutil.UpdateChannel, len(confPkgs))
	channelByName := make(map[string]goutil.UpdateChannel, len(confPkgs))
	for _, p := range confPkgs {
		channel := goutil.NormalizeUpdateChannel(string(p.UpdateChannel))
		if p.ImportPath != "" {
			channelByImportPath[p.ImportPath] = channel
		}
		channelByName[normalizeBinName(p.Name)] = channel
	}

	result := make([]goutil.Package, 0, len(pkgs))
	for _, p := range pkgs {
		channel := goutil.UpdateChannelLatest
		if c, ok := channelByImportPath[p.ImportPath]; ok {
			channel = c
		} else if c, ok := channelByName[normalizeBinName(p.Name)]; ok {
			channel = c
		}
		p.UpdateChannel = channel
		result = append(result, p)
	}
	return result
}

// normalizeBinName makes a binary name comparable regardless of the OS the
// config was exported from. It trims surrounding whitespace and strips a
// trailing ".exe" suffix case-insensitively, so a channel saved on Windows
// ("foo.exe", or a hand-edited "foo.EXE") still matches the same binary named
// "foo" on any OS (#341). Unlike the exclude-matching normalization in update.go
// this is intentionally OS-independent: the saved config may originate from a
// different OS than the one currently running gup.
func normalizeBinName(name string) string {
	name = strings.TrimSpace(name)
	const exe = ".exe"
	if len(name) >= len(exe) && strings.EqualFold(name[len(name)-len(exe):], exe) {
		return name[:len(name)-len(exe)]
	}
	return name
}
