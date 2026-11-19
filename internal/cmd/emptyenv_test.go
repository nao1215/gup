package cmd

import (
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/print"
)

// TestHandleEmptyEnvironment locks in the shared "no manageable binaries"
// behavior that update and check delegate to.
//
//nolint:paralleltest // captureCheckOutput drives a shared buffer-backed Printer
func TestHandleEmptyEnvironment(t *testing.T) {
	t.Run("explicit selection is a usage error", func(t *testing.T) {
		var code int
		out := captureCheckOutput(t, func(p *print.Printer) int {
			code = handleEmptyEnvironment(p, "", false, true, "boom: nothing selected")
			return code
		})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(out, "boom: nothing selected") {
			t.Errorf("output = %q, want it to contain the usage error", out)
		}
	})

	t.Run("first-run note on empty environment", func(t *testing.T) {
		var code int
		out := captureCheckOutput(t, func(p *print.Printer) int {
			code = handleEmptyEnvironment(p, "", false, false, "unused")
			return code
		})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(out, emptyEnvMessage) {
			t.Errorf("output = %q, want the empty-environment note", out)
		}
	})

	t.Run("json mode emits an empty array", func(t *testing.T) {
		var code int
		out := captureCheckOutput(t, func(p *print.Printer) int {
			code = handleEmptyEnvironment(p, "", true, false, "unused")
			return code
		})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if strings.TrimSpace(out) != "[]" {
			t.Errorf("output = %q, want an empty JSON array", out)
		}
	})

	t.Run("explicit --file pointing at a directory fails fast", func(t *testing.T) {
		dir := t.TempDir()
		var code int
		_ = captureCheckOutput(t, func(p *print.Printer) int {
			code = handleEmptyEnvironment(p, dir, false, false, "unused")
			return code
		})
		if code != 1 {
			t.Errorf("exit code = %d, want 1 for an invalid --file", code)
		}
	})
}
