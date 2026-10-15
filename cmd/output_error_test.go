package cmd

import (
	"errors"
	"io"
	"testing"

	"github.com/nao1215/gup/internal/print"
)

type rejectedOutput struct{}

func (rejectedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestVersionReportsOutputFailure(t *testing.T) {
	t.Parallel()
	cmd := newVersionCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(rejectedOutput{})
	if err := cmd.Execute(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure = %v, want closed pipe", err)
	}
}

func TestBugReportReportsOutputFailure(t *testing.T) {
	t.Parallel()
	cmd := newBugReportCmd()
	cmd.SetOut(rejectedOutput{})
	if code := bugReport(cmd, nil, func(string) bool { return false }); code == 0 {
		t.Fatal("failed fallback output returned success")
	}
}

func TestStateLockReportsOutputFailure(t *testing.T) {
	t.Parallel()
	p := print.New(rejectedOutput{}, io.Discard)
	cmd := newCheckCmd()
	code := withStateLock(p, cmd, nil, cmdNameCheck, func() int {
		p.Info("result")
		return 0
	})
	if code == 0 {
		t.Fatal("failed command output returned success")
	}
}
