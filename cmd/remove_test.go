package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func Test_remove(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name   string
		args   args
		gobin  string
		want   int
		stderr []string
	}{
		{
			name: "no argument (not specify delete target)",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			gobin: "no use",
			want:  1,
			stderr: []string{
				"gup:ERROR: no command name specified",
				"",
			},
		},
		{
			name: "GOBIN does not exist",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{"test"},
			},
			gobin: "not_exist",
			want:  1,
			stderr: []string{
				"gup:ERROR: no such file or directory: not_exist/test",
				"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldGoBin := os.Getenv("GOBIN")
			if err := os.Setenv("GOBIN", tt.gobin); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Setenv("GOBIN", oldGoBin); err != nil {
					t.Fatal(err)
				}
			}()

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			tt.args.cmd.Flags().BoolP("force", "f", false, "Forcibly remove the file")
			if got := remove(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("remove() = %v, want %v", got, tt.want)
			}
			pw.Close()
			print.Stdout = orgStdout
			print.Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			defer pr.Close()
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.stderr, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
