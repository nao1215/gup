// Package print defines functions to accept colored standard output and user input
package print

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestInfo(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Print message",
			args: args{
				msg: "test message",
			},
			want: []string{"gup:INFO : test message", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgStdout := Stdout
			orgStderr := Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			Stdout = pw
			Stderr = pw

			Info(tt.args.msg)
			pw.Close()
			Stdout = orgStdout
			Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWarn(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Print message",
			args: args{
				msg: "test message",
			},
			want: []string{"gup:WARN : test message", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgStdout := Stdout
			orgStderr := Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			Stdout = pw
			Stderr = pw

			Warn(tt.args.msg)
			pw.Close()
			Stdout = orgStdout
			Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestErr(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Print message",
			args: args{
				msg: "test message",
			},
			want: []string{"gup:ERROR: test message", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgStdout := Stdout
			orgStderr := Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			Stdout = pw
			Stderr = pw

			Err(tt.args.msg)
			pw.Close()
			Stdout = orgStdout
			Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
func TestFatal(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name     string
		args     args
		want     []string
		exitcode int
	}{
		{
			name: "Print message",
			args: args{
				msg: "test message",
			},
			want:     []string{"gup:FATAL: test message", ""},
			exitcode: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgStdout := Stdout
			orgStderr := Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			Stdout = pw
			Stderr = pw

			orgOsExit := OsExit
			exitCode := 0
			OsExit = func(code int) {
				exitCode = code
			}
			defer func() { OsExit = orgOsExit }()

			Fatal(tt.args.msg)
			pw.Close()
			Stdout = orgStdout
			Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
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
func TestQuestion(t *testing.T) {
	type args struct {
		ask string
	}
	tests := []struct {
		name  string
		args  args
		input string
		want  bool
	}{
		{
			name:  "user input 'y'",
			args:  args{"no check"},
			input: "y",
			want:  true,
		},
		{
			name:  "user input 'yes'",
			args:  args{"no check"},
			input: "yes",
			want:  true,
		},
		{
			name:  "user input 'n'",
			args:  args{"no check"},
			input: "n",
			want:  false,
		},
		{
			name:  "user input 'no'",
			args:  args{"no check"},
			input: "no",
			want:  false,
		},
		{
			name:  "user input 'yes' after 'a'",
			args:  args{"no check"},
			input: "a\nyes",
			want:  true,
		},
		{
			name:  "user only input enter",
			args:  args{"no check"},
			input: "\nyes",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcDefer, err := mockStdin(t, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			defer funcDefer()

			if got := Question(tt.args.ask); got != tt.want {
				t.Errorf("Question() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuestion_FmtScanlnErr(t *testing.T) {
	t.Run("fmt.Scanln() return error", func(t *testing.T) {
		orgFmtScanln := FmtScanln
		FmtScanln = func(a ...any) (n int, err error) {
			return -1, errors.New("some error")
		}
		defer func() { FmtScanln = orgFmtScanln }()

		if got := Question("no check"); got != false {
			t.Errorf("Question() = %v, want %v", got, false)
		}
	})
}

// mockStdin is a helper function that lets the test pretend dummyInput as os.Stdin.
// It will return a function for `defer` to clean up after the test.
func mockStdin(t *testing.T, dummyInput string) (funcDefer func(), err error) {
	t.Helper()

	oldOsStdin := os.Stdin
	tmpFile, err := os.CreateTemp(t.TempDir(), time.Now().GoString())

	if err != nil {
		return nil, err
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
		os.Remove(tmpFile.Name())
	}, nil
}
