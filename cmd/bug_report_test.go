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

func TestBug(t *testing.T) {
	b := bytes.NewBufferString("")

	copyRootCmd := newRootCmd()

	copyRootCmd.SetOut(b)
	copyRootCmd.SetArgs([]string{"bug-report", "--help"})

	err := copyRootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	wantBytes, err := os.ReadFile(filepath.Join("testdata", "bug_report", "bug_report.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(strings.TrimSpace(string(gotBytes)), strings.TrimSpace(string(wantBytes))); diff != "" {
		t.Errorf("value is mismatch (-want +got):\n%s", diff)
	}
}
