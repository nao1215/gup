//go:build !int

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBugReport(t *testing.T) {
	t.Parallel()

	t.Run("Check bug-report --help", func(t *testing.T) {
		b := bytes.NewBufferString("")

		copyRootCmd := newRootCmd()

		copyRootCmd.SetOut(b)
		copyRootCmd.SetArgs([]string{"bug-report", "--help"})

		if err := copyRootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		gotBytes, err := io.ReadAll(b)
		if err != nil {
			t.Fatal(err)
		}
		gotBytes = bytes.ReplaceAll(gotBytes, []byte("\r\n"), []byte("\n"))

		wantBytes, err := os.ReadFile(filepath.Join("testdata", "bug_report", "bug_report.txt"))
		if err != nil {
			t.Fatal(err)
		}
		wantBytes = bytes.ReplaceAll(wantBytes, []byte("\r\n"), []byte("\n"))

		if diff := cmp.Diff(strings.TrimSpace(string(gotBytes)), strings.TrimSpace(string(wantBytes))); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	})
}

func Test_bugReport(t *testing.T) {
	t.Parallel()

	cmd := newBugReportCmd()
	cmd.Version = "v0.0.0"

	wantReturnVal := 0
	gotReturnVal := bugReport(cmd, nil, func(s string) bool {
		if !strings.Contains(s, "v0.0.0") {
			t.Errorf("Expected bug report to contain version number 'v0.0.0', but got: %s", s)
		}
		return true
	})
	if gotReturnVal != wantReturnVal {
		t.Errorf("bugReport() = %d; want %d", gotReturnVal, wantReturnVal)
	}
}
