// Package cmd define subcommands provided by the gup command
package cmd

import (
	"os"
	"testing"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "success",
			args: []string{""},
		},
		{
			name: "fail",
			args: []string{"no-exist-subcommand", "--no-exist-option"},
		},
	}
	for _, tt := range tests {
		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})
	}
}
