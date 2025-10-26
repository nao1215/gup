package cmd

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/gorky/file"
)

func Test_removeLoop(t *testing.T) {
	type args struct {
		gobin  string
		force  bool
		target []string
	}

	type test struct {
		name  string
		args  args
		input string
		want  int
	}

	tests := []test{
		{
			name: "windows environment and suffix is mismatch",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{"posixer"},
			},
			input: "y",
			want:  1,
		},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, test{
			name: "interactive question: input 'y'",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{"posixer.exe"},
			},
			input: "y",
			want:  0,
		})
		tests = append(tests, test{
			name: "delete cancel",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{"posixer.exe"},
			},
			input: "n",
			want:  0,
		})
	} else {
		tests = append(tests, test{
			name: "interactive question: input 'y'",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{"posixer"},
			},
			input: "y",
			want:  0,
		})
		tests = append(tests, test{
			name: "delete cancel",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{"posixer"},
			},
			input: "n",
			want:  0,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Join("testdata", "delete"), 0755); err != nil {
				t.Fatal(err)
			}

			src := ""
			dest := ""
			if runtime.GOOS == "windows" {
				src = filepath.Join("testdata", "check_success_for_windows", "posixer.exe")
				dest = filepath.Join("testdata", "delete", "posixer.exe")
			} else {
				src = filepath.Join("testdata", "check_success", "posixer")
				dest = filepath.Join("testdata", "delete", "posixer")
			}
			newFile, err := os.Create(dest)
			if err != nil {
				t.Fatal(err)
			}

			oldFile, err := os.Open(src)
			if err != nil {
				t.Fatal(err)
			}

			_, err = io.Copy(newFile, oldFile)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				os.Remove(dest)
			}()
			oldFile.Close()
			newFile.Close()

			funcDefer, err := mockStdin(t, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			defer funcDefer()

			if runtime.GOOS != "windows" && tt.name == "windows environment and suffix is mismatch" {
				GOOS = "windows"
				defer func() { GOOS = runtime.GOOS }()
				t.Setenv("GOEXE", ".exe")
			}

			if got := removeLoop(tt.args.gobin, tt.args.force, tt.args.target); got != tt.want {
				t.Errorf("removeLoop() = %v, want %v", got, tt.want)
			}

			if tt.name == "delete cancel" && !file.IsFile(dest) {
				t.Errorf("input no, however posixer command is deleted")
			}
		})
	}
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
		os.Remove(tmpFile.Name())
	}, nil
}
