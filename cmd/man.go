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
		Use:     "man",
		Short:   "Generate man-pages under /usr/share/man/man1 (need root privilege)",
		Long:    `Generate man-pages under /usr/share/man/man1 (need root privilege)`,
		Example: "  sudo gup man",
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(man(cmd, args))
		},
	}
	return cmd
}

func man(cmd *cobra.Command, args []string) int { //nolint
	if err := generateManpages(filepath.Join("/", "usr", "share", "man", "man1")); err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not generate man-pages", err))
		return 1
	}
	return 0
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

		out, err := os.Create(filepath.Join(dst, filepath.Base(file)+".gz"))
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
