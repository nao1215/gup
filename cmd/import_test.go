package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func Test_runImport_Error(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name   string
		args   args
		home   string
		want   int
		stderr []string
	}{
		{
			name: "argument parse error (--dry-run)",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "",
			want: 1,
			stderr: []string{
				"gup:ERROR: can not parse command line argument (--dry-run): flag accessed but not defined: dry-run",
				"",
			},
		},
		{
			name: "argument parse error (--input)",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "",
			want: 1,
			stderr: []string{
				"gup:ERROR: can not parse command line argument (--input): flag accessed but not defined: input",
				"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "argument parse error (--dry-run)" {
				tt.args.cmd.Flags().StringP("input", "i", config.FilePath(), "specify gup.conf file path to import")
			} else if tt.name == "argument parse error (--input)" {
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
			}

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			if got := runImport(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("runImport() = %v, want %v", got, tt.want)
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
