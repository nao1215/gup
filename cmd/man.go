package cmd

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newManCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "man",
		Short:             "Generate man-pages under /usr/share/man/man1 (need root privilege)",
		Long:              `Generate man-pages under /usr/share/man/man1 (need root privilege)`,
		Example:           "  sudo gup man",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(man(cmd, args))
		},
	}
	return cmd
}

func man(_ *cobra.Command, _ []string) int {
	for _, dst := range manPaths(os.Getenv("MANPATH")) {
		if err := generateManpages(dst); err != nil {
			print.Err(fmt.Errorf("can not generate man-pages in %s: %w", dst, err))
			return 1
		}
	}
	return 0
}

// manPaths normalizes MANPATH entries into man1 directories.
// Empty or invalid MANPATH falls back to /usr/share/man/man1.
func manPaths(manpathEnv string) []string {
	defaultPath := filepath.Join("/", "usr", "share", "man", "man1")
	if manpathEnv == "" {
		return []string{defaultPath}
	}

	paths := make([]string, 0)
	for _, p := range strings.Split(manpathEnv, ":") {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || p == "." || !filepath.IsAbs(p) {
			continue
		}

		if filepath.Base(p) != "man1" {
			p = filepath.Join(p, "man1")
		}
		paths = append(paths, p)
	}

	if len(paths) == 0 {
		return []string{defaultPath}
	}

	return paths
}

func generateManpages(dst string) error {
	now := time.Now()
	header := &doc.GenManHeader{
		Title:   `gup - Update binaries installed by 'go install'`,
		Section: "1",
		Date:    &now,
	}

	tmpDir, err := os.MkdirTemp("", "gup")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			// TODO: If use go 1.20, rewrite like this.
			// err = errors.Join(err, closeErr)
			err = removeErr // overwrite error
		}
	}()

	err = doc.GenManTree(newRootCmd(), header, tmpDir)
	if err != nil {
		return err
	}

	manFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.1"))
	if err != nil {
		return err
	}

	return copyManpages(manFiles, dst)
}

func copyManpages(manFiles []string, dst string) error {
	dst = filepath.Clean(dst)

	for _, file := range manFiles {
		file = filepath.Clean(file)

		in, err := os.Open(file)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := in.Close(); closeErr != nil {
				// TODO: If use go 1.20, rewrite like this.
				// err = errors.Join(err, closeErr)
				err = closeErr // overwrite error
			}
		}()

		out, err := os.Create(filepath.Clean(filepath.Join(dst, fmt.Sprintf("%s%s", filepath.Base(file), ".gz"))))
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := out.Close(); closeErr != nil {
				// TODO: If use go 1.20, rewrite like this.
				// err = errors.Join(err, closeErr)
				err = closeErr // overwrite error
			}
		}()

		gz := gzip.NewWriter(out)
		gz.Name = strings.TrimSuffix(filepath.Base(file), ".1")
		defer func() {
			if closeErr := gz.Close(); closeErr != nil {
				// TODO: If use go 1.20, rewrite like this.
				// err = errors.Join(err, closeErr)
				err = closeErr // overwrite error
			}
		}()

		print.Info("Generate " + out.Name())
		_, err = io.Copy(gz, in)
		if err != nil {
			return err
		}
	}
	return nil
}
