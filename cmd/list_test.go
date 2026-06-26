package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Test_list_succeeds_without_go_command reproduces the bug where 'gup list'
// failed with "you didn't install golang" when the 'go' command was absent, even
// though listing only reads local build info from $GOBIN and never invokes the Go
// toolchain.
func Test_list_succeeds_without_go_command(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	p, buf := newTestPrinter()
	if got := list(p, newListCmd(), []string{}); got != 0 {
		t.Fatalf("list() without go = %d, want 0; output:\n%s", got, buf.String())
	}
	if strings.Contains(buf.String(), "you didn't install golang") {
		t.Errorf("list must not require the go command:\n%s", buf.String())
	}
}

func Test_list_gobin_is_empty(t *testing.T) {
	type args struct {
		args []string
	}
	tests := []struct {
		name   string
		gobin  string
		args   args
		want   int
		stderr []string
	}{
		{
			// An empty-but-valid environment is a normal first-run condition,
			// not an error (#350): list succeeds with an informational note.
			name:  "gobin is empty",
			gobin: filepath.Join("testdata", "empty_dir"),
			args:  args{},
			want:  0,
			stderr: []string{
				emptyEnvMessage,
				"",
			},
		},
		{
			// A $GOBIN directory that does not exist yet is a normal first-run
			// condition (#350): list treats it like an empty environment (exit 0
			// + note) instead of failing with a read-dir error.
			name:  testGobinEmpty,
			gobin: testNoExistDir,
			args:  args{},
			want:  0,
			stderr: []string{
				emptyEnvMessage,
				"",
			},
		},
	}

	if err := os.Mkdir(filepath.Join("testdata", "empty_dir"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOBIN", tt.gobin)

			p, buf := newTestPrinter()

			if got := list(p, newListCmd(), tt.args.args); got != tt.want {
				t.Errorf("list() = %v, want %v", got, tt.want)
			}
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.stderr, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}

	err := os.Remove(filepath.Join("testdata", "empty_dir"))
	if err != nil {
		t.Fatal(err)
	}
}
