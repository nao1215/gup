package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHasTarget covers the rule that decides whether the OS classification is
// consulted at all. Getting it wrong is silent: the runner drops the classified
// spec list, atago falls back to searching the whole repository, and the Windows
// leg runs every spec including the ones e2e/os_matrix.tsv excludes -- reporting
// failures the classification exists to prevent, or passing for the wrong reason.
func TestHasTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	spec := filepath.Join(dir, "example.atago.yaml")
	if err := os.WriteFile(spec, []byte("version: \"1\"\n"), 0o600); err != nil {
		t.Fatalf("failed to write the spec file: %v", err)
	}

	tests := map[string]struct {
		args []string
		want bool
	}{
		"no arguments":                {args: nil, want: false},
		"boolean flag only":           {args: []string{"--verbose"}, want: false},
		"flag with an attached value": {args: []string{"--filter=update"}, want: false},
		"flag with a separate value":  {args: []string{"--filter", "update"}, want: false},
		"several flags and their values": {
			args: []string{"--parallel", "4", "--report", "json"},
			want: false,
		},
		// The case both earlier rules got wrong: the value of --filter happens to
		// name a directory that exists (the repository really does have an `e2e`
		// directory), so "is a path" is not enough to call it a target either.
		"flag value that names an existing directory": {
			args: []string{"--filter", dir},
			want: false,
		},
		"flag value that names an existing file": {
			args: []string{"--report", spec},
			want: false,
		},
		"boolean flag followed by a target": {
			args: []string{"--update-snapshots", spec},
			want: true,
		},
		// `--` ends the options: what follows is a target however it is spelled.
		"terminator then a target": {args: []string{"--", spec}, want: true},
		"terminator then an oddly named target": {
			args: []string{"--", "--weird.atago.yaml"},
			want: true,
		},
		"terminator with nothing after it": {args: []string{"--"}, want: false},
		"flags then a terminator":          {args: []string{"--verbose", "--"}, want: false},
		"an existing spec file":            {args: []string{spec}, want: true},
		"an existing directory":            {args: []string{dir}, want: true},
		"a flag then a target":             {args: []string{"--verbose", spec}, want: true},
		"a flag value then a target":       {args: []string{"--filter", "update", spec}, want: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := hasTarget(tt.args); got != tt.want {
				t.Errorf("hasTarget(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
