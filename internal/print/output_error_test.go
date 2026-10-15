package print

import (
	"errors"
	"io"
	"testing"
)

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPrinterRetainsOutputFailure(t *testing.T) {
	t.Parallel()
	p := New(failedOutput{}, io.Discard)
	p.Info("lost output")
	p.Warn("successful diagnostic")
	if !errors.Is(p.OutputError(), io.ErrClosedPipe) || p.ExitCode(0) != 1 || p.ExitCode(2) != 2 {
		t.Fatalf("lost write failure: %v", p.OutputError())
	}
}

func TestQuestionDoesNotReadAfterOutputFailure(t *testing.T) {
	t.Parallel()
	p := New(failedOutput{}, io.Discard)
	p.scanln = func(...any) (int, error) {
		t.Fatal("read an answer to an invisible question")
		return 0, nil
	}
	if yes, err := p.Question("continue?"); yes || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("question = %v, %v", yes, err)
	}
}
