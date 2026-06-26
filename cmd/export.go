package cmd

import (
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/pkgselect"
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
			OsExit(export(printerFor(cmd), cmd, args))
		},
	}
	cmd.Flags().BoolP("output", "o", false, "print command path information at STDOUT")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to export")
	mustMarkFileFlagAsJSON(cmd)

	return cmd
}

func export(p *print.Printer, cmd *cobra.Command, _ []string) int {
	// export only reads local build info from $GOBIN and writes gup.json; it never
	// invokes the Go toolchain, so it must not fail when 'go' is absent (mirrors
	// 'gup unpin').
	output, err := getFlagBool(cmd, "output")
	if err != nil {
		p.Err(err)
		return 1
	}
	configPath, err := getFlagString(cmd, "file")
	if err != nil {
		p.Err(err)
		return 1
	}
	configPath = config.ResolveExportFilePath(configPath)

	pkgs, err := pkgselect.PackageInfo(p)
	if err != nil {
		p.Err(err)
		return 1
	}
	pkgs = validPkgInfo(p, pkgs)
	// Saved channels are read from the same file export writes to, so exporting
	// back to an alternate config passed with --file round-trips safely instead
	// of resetting that file's channels to @latest from the canonical config
	// (#341). configPath is the resolved destination: the explicit --file when
	// given, otherwise the canonical user-level config, so the default (no --file)
	// behavior of reading channels from the canonical config is unchanged.
	channelSource := configPath
	// A malformed channel-source config must fail fast instead of silently
	// exporting every package as @latest, which would drop intentionally pinned
	// channels such as @main from the written gup.json (#369).
	confPkgs, err := configstate.ReadFileIfExists(channelSource)
	if err != nil {
		p.Err(err)
		return 1
	}
	pkgs = configstate.ApplySavedChannels(pkgs, confPkgs)

	// An empty-but-valid environment is a normal first-run condition, not an
	// error (#350): export still succeeds and writes an empty configuration.
	if len(pkgs) == 0 {
		p.Warn(emptyEnvMessage + "; exporting an empty configuration")
	}

	if output {
		err = outputConfig(p, pkgs)
	} else {
		err = writeConfigFile(configPath, pkgs)
	}
	if err != nil {
		p.Err(err)
		return 1
	}
	if !output {
		p.Info("Export " + configPath)
	}
	return 0
}

func outputConfig(p *print.Printer, pkgs []goutil.Package) error {
	return config.WriteConfFile(p.Out(), pkgs)
}

func validPkgInfo(p *print.Printer, pkgs []goutil.Package) []goutil.Package {
	result := []goutil.Package{}
	for _, v := range pkgs {
		if v.ImportPath == "" {
			p.Warn("can't get '" + v.Name + "' package path information. old go version binary")
			continue
		}
		result = append(result, goutil.Package{Name: v.Name, ImportPath: v.ImportPath, Version: v.Version})
	}
	return result
}
