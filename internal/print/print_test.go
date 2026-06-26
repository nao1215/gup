// Package print defines functions to accept colored standard output and user input
package print

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	testPrintMessage = "Print message"
	testMessage      = "test message"
	testNoCheck      = "no check"
)

func TestPrinter_Info(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{name: testPrintMessage, msg: testMessage, want: []string{testMessage, ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			New(buf, buf).Info(tt.msg)
			got := strings.Split(buf.String(), "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrinter_Warn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{name: testPrintMessage, msg: testMessage, want: []string{"gup:WARN : test message", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			New(buf, buf).Warn(tt.msg)
			got := strings.Split(buf.String(), "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrinter_Err(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{name: testPrintMessage, msg: testMessage, want: []string{"gup:ERROR: test message", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			New(buf, buf).Err(tt.msg)
			got := strings.Split(buf.String(), "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrinter_Hint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{name: testPrintMessage, msg: testMessage, want: []string{"gup:HINT : test message", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			New(buf, buf).Hint(tt.msg)
			got := strings.Split(buf.String(), "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrinter_Fatal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		msg      string
		want     []string
		exitcode int
	}{
		{name: testPrintMessage, msg: testMessage, want: []string{"gup:FATAL: test message", ""}, exitcode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			p := New(buf, buf)
			exitCode := 0
			p.exit = func(code int) { exitCode = code }

			p.Fatal(tt.msg)
			got := strings.Split(buf.String(), "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
			if exitCode != tt.exitcode {
				t.Errorf("value is mismatch. want=%d got=%d", exitCode, tt.exitcode)
			}
		})
	}
}

//nolint:paralleltest // mutates the process-global os.Stdin via mockStdin
func TestPrinter_Question(t *testing.T) {
	tests := []struct {
		name  string
		ask   string
		input string
		want  bool
	}{
		{name: "user input 'y'", ask: testNoCheck, input: "y", want: true},
		{name: "user input 'yes'", ask: testNoCheck, input: "yes", want: true},
		{name: "user input 'n'", ask: testNoCheck, input: "n", want: false},
		{name: "user input 'no'", ask: testNoCheck, input: "no", want: false},
		{name: "user input 'yes' after 'a'", ask: testNoCheck, input: "a\nyes", want: true},
		{name: "user only input enter", ask: testNoCheck, input: "\nyes", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcDefer, err := mockStdin(t, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			defer funcDefer()

			buf := &bytes.Buffer{}
			got, err := New(buf, buf).Question(tt.ask)
			if err != nil {
				t.Errorf("Question() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Question() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPrinter_ConcurrentWrites verifies a Printer shared across goroutines (as
// happens when goutil reports unreadable binaries from parallel.Run workers)
// serializes its writes: under -race this fails if the methods drop the lock,
// and every emitted line stays whole.
func TestPrinter_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	p := New(buf, buf)

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			p.Warn(testMessage)
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != goroutines {
		t.Fatalf("got %d lines, want %d (writes interleaved?)", len(lines), goroutines)
	}
	for _, l := range lines {
		if l != "gup:WARN : test message" {
			t.Fatalf("interleaved/garbled warning line: %q", l)
		}
	}
}

func TestPrinter_Question_ScanlnErr(t *testing.T) {
	t.Parallel()
	t.Run("scanln returns error", func(t *testing.T) {
		t.Parallel()
		buf := &bytes.Buffer{}
		p := New(buf, buf)
		wantErr := errors.New("some error")
		p.scanln = func(_ ...any) (n int, err error) {
			return -1, wantErr
		}

		got, err := p.Question(testNoCheck)
		if got != false {
			t.Errorf("Question() = %v, want %v", got, false)
		}
		// A scanln read failure must surface as an error so callers can tell it
		// apart from a deliberate "no".
		if !errors.Is(err, wantErr) {
			t.Errorf("Question() error = %v, want %v", err, wantErr)
		}
	})
}

// mockStdin is a helper function that lets the test pretend dummyInput as os.Stdin.
// It will return a function for `defer` to clean up after the test.
func mockStdin(t *testing.T, dummyInput string) (funcDefer func(), err error) {
	t.Helper()

	oldOsStdin := os.Stdin
	var tmpFile *os.File
	var e error
	if runtime.GOOS != "windows" {
		tmpFile, e = os.CreateTemp(t.TempDir(), strings.ReplaceAll(t.Name(), "/", ""))
	} else {
		// See https://github.com/golang/go/issues/51442
		tmpFile, e = os.CreateTemp(os.TempDir(), strings.ReplaceAll(t.Name(), "/", ""))
	}
	if e != nil {
		return nil, e
	}

	content := []byte(dummyInput)

	if _, err := tmpFile.Write(content); err != nil {
		return nil, err
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, err
	}

	// Set stdin to the temp file
	os.Stdin = tmpFile

	return func() {
		// clean up
		os.Stdin = oldOsStdin
		_ = os.Remove(tmpFile.Name())
	}, nil
}
