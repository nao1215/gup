//nolint:paralleltest
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_importEmptyExport(t *testing.T) {
	for _, dryRun := range []string{"false", "true"} {
		t.Run("dry-run="+dryRun, func(t *testing.T) {
			setupXDGBase(t)
			gobin := t.TempDir()
			t.Setenv("GOBIN", gobin)
			confPath := filepath.Join(t.TempDir(), "gup.json")
			exportCmd := newExportCmd()
			if err := exportCmd.Flags().Set("file", confPath); err != nil {
				t.Fatal(err)
			}
			p, buf := newTestPrinter()
			if code := export(p, exportCmd, nil); code != 0 {
				t.Fatalf("export() = %d: %s", code, buf.String())
			}
			before, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatal(err)
			}
			original := installByVersionCtx
			installByVersionCtx = func(context.Context, string, string) error {
				t.Error("empty import must not install a package")
				return nil
			}
			t.Cleanup(func() { installByVersionCtx = original })
			cmd := newImportCmd()
			if err := cmd.Flags().Set("file", confPath); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Flags().Set("dry-run", dryRun); err != nil {
				t.Fatal(err)
			}
			buf.Reset()
			// Importing an empty export must not require the Go toolchain.
			t.Setenv("PATH", "")
			if code := runImport(p, cmd, nil); code != 0 {
				t.Fatalf("import of empty export = %d: %s", code, buf.String())
			}
			if !strings.Contains(buf.String(), "nothing to import") || !strings.Contains(buf.String(), confPath) {
				t.Errorf("expected no-work message naming configuration: %s", buf.String())
			}
			after, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Error("import changed the exported configuration")
			}
			entries, err := os.ReadDir(gobin)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 || os.Getenv("GOBIN") != gobin {
				t.Error("empty import changed GOBIN or its contents")
			}
		})
	}
}

func Test_runImport_invalidEmptyConfiguration(t *testing.T) {
	for _, input := range []string{
		`{"schema_version":`,
		`{"schema_version":99,"packages":[]}`,
		`{"schema_version":1,"packages":[{}]}`,
	} {
		t.Run(input, func(t *testing.T) {
			confPath := filepath.Join(t.TempDir(), "gup.json")
			if err := os.WriteFile(confPath, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := newImportCmd()
			if err := cmd.Flags().Set("file", confPath); err != nil {
				t.Fatal(err)
			}
			p, buf := newTestPrinter()
			if code := runImport(p, cmd, nil); code != 1 {
				t.Errorf("invalid configuration import = %d, want 1", code)
			}
			if !strings.Contains(buf.String(), confPath) || strings.Contains(buf.String(), "nothing to import") {
				t.Errorf("expected configuration error naming the file: %s", buf.String())
			}
		})
	}
}
