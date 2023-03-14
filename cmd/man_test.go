package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateManpages(t *testing.T) {
	t.Parallel()

	t.Run("Generate man pages", func(t *testing.T) {
		dst, err := os.MkdirTemp("", "test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			if removeErr := os.RemoveAll(dst); removeErr != nil {
				t.Fatal(removeErr)
			}
		}()

		if err := generateManpages(dst); err != nil {
			t.Fatalf("generateManpages() failed: %v", err)
		}

		manFiles, err := filepath.Glob(filepath.Join(dst, "*.1.gz"))
		if err != nil {
			t.Errorf("Failed to glob man files: %v", err)
		}
		if len(manFiles) == 0 {
			t.Error("No man files found")
		}
	})
}
