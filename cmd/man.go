package cmd

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manpageCreateMode is the base mode requested for a newly created man page,
// matching the 0666 os.Create opened with before the atomic write. The process
// umask is applied to it (see applyUmask) so restrictive umasks are honored.
const manpageCreateMode = 0o666

func newManCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "man",
		Short:             "Generate man-pages under /usr/share/man/man1 (need root privilege)",
		Long:              `Generate man-pages under /usr/share/man/man1 (need root privilege)`,
		Example:           "  sudo gup man",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(man(printerFor(cmd), cmd, args))
		},
	}
	return cmd
}

func man(p *print.Printer, _ *cobra.Command, _ []string) int {
	for _, dst := range manPaths(os.Getenv("MANPATH")) {
		if err := generateManpages(p, dst); err != nil {
			p.Err(fmt.Errorf("can not generate man-pages in %s: %w", dst, err))
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
	for _, p := range filepath.SplitList(manpathEnv) {
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

func generateManpages(p *print.Printer, dst string) error {
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
			err = errors.Join(err, removeErr)
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

	return copyManpages(p, manFiles, dst)
}

func copyManpages(p *print.Printer, manFiles []string, dst string) error {
	dst = filepath.Clean(dst)

	// Ensure the target man1 directory exists before writing. A valid custom
	// MANPATH whose man1 directory is absent should be created rather than
	// causing a failure; an unwritable target surfaces as a clear error (#344).
	if err := os.MkdirAll(dst, fileutil.FileModeCreatingDir); err != nil {
		return fmt.Errorf("can not create man directory %s: %w", dst, err)
	}

	for _, file := range manFiles {
		if err := copyOneManpage(p, filepath.Clean(file), dst); err != nil {
			return err
		}
	}
	return nil
}

func copyOneManpage(p *print.Printer, file, dst string) (err error) {
	outputPath := filepath.Clean(filepath.Join(dst, filepath.Base(file)+".gz"))
	defer func() {
		if err == nil {
			p.Info("Generate " + outputPath)
		}
	}()

	in, err := os.Open(file) //nolint:gosec // file comes from filepath.Glob, already cleaned by caller
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	return writeManpageAtomically(outputPath, in, strings.TrimSuffix(filepath.Base(file), ".1"))
}

// writeManpageAtomically gzips src into outputPath via a temp file in the same
// directory followed by an atomic rename, so a failed or interrupted write can
// never truncate or corrupt an existing man page; the destination is only
// replaced once the new content is fully written. A symlinked destination is
// preserved by resolving the link to its target first (matching how the previous
// os.Create followed the link, and consistent with config_file.go).
func writeManpageAtomically(outputPath string, src io.Reader, gzName string) (err error) {
	resolvedPath, err := fileutil.ResolveSymlinkTarget(outputPath)
	if err != nil {
		return fmt.Errorf("can not resolve man page path %s: %w", outputPath, err)
	}
	outputPath = resolvedPath
	// Reject a directory destination before staging any temp file, so a mistaken
	// path cannot replace a directory tree via the rename-replace flow (mirrors the
	// guard in cmd/config_file.go).
	if fileutil.IsDir(outputPath) {
		return fmt.Errorf("%s is a directory, not a file", outputPath)
	}
	dir := filepath.Dir(outputPath)

	// os.CreateTemp creates the temp file with mode 0600. Reproduce os.Create's
	// previous behavior instead: for a new file use 0666 with the process umask
	// applied (so a restrictive umask such as 077 still yields 0600, not a wider
	// 0644), and when replacing an existing file preserve that file's own mode.
	mode := applyUmask(manpageCreateMode)
	if info, statErr := os.Stat(outputPath); statErr == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("can not stat man page %s: %w", outputPath, statErr)
	}

	out, err := os.CreateTemp(dir, filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := out.Name()
	defer func() {
		if out != nil {
			if closeErr := out.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	gz := gzip.NewWriter(out)
	gz.Name = gzName
	if _, err = io.Copy(gz, src); err != nil {
		return err
	}
	if err = gz.Close(); err != nil {
		return err
	}
	if err = out.Chmod(mode); err != nil {
		return fmt.Errorf("can not set permissions on man page %s: %w", outputPath, err)
	}
	if err = out.Sync(); err != nil {
		return fmt.Errorf("can not sync man page %s: %w", outputPath, err)
	}
	if err = out.Close(); err != nil {
		out = nil
		return fmt.Errorf("can not close man page %s: %w", outputPath, err)
	}
	out = nil

	if err = renameWithReplace(tmpPath, outputPath); err != nil {
		return fmt.Errorf("can not finalize man page %s: %w", outputPath, err)
	}
	return nil
}
