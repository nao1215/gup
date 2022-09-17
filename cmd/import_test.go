package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func Test_runImport(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name string
		args args
		home string
		want int
	}{
		{
			name: "not exist config file",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "/",
			want: 1,
		},
		{
			name: "argument parse error",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "",
			want: 1,
		},
		{
			name: "config file is empty",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "./testdata/empty_conf",
			want: 1,
		},
		{
			name: "can not read config",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			home: "./testdata/can_not_read_conf",
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "argument parse error" {
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
			}

			oldHome := os.Getenv("HOME")
			if err := os.Setenv("HOME", tt.home); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Setenv("HOME", oldHome); err != nil {
					t.Fatal(err)
				}
			}()

			if got := runImport(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("runImport() = %v, want %v", got, tt.want)
			}
		})
	}
}
