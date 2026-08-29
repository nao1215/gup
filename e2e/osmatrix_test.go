package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestOSMatrix_classifiesEverySpec is the reason the table exists. gup's E2E
// suite ran on Linux only; when it grew macOS and Windows legs, the risk was not
// that a spec would be excluded -- some genuinely cannot run there -- but that
// the exclusions would be invisible and would quietly grow. A new spec now
// either runs everywhere or has a written reason on the record.
func TestOSMatrix_classifiesEverySpec(t *testing.T) {
	t.Parallel()

	matrix, err := LoadMatrix(".")
	if err != nil {
		t.Fatalf("LoadMatrix() error: %v", err)
	}
	specs, err := SpecFiles(".")
	if err != nil {
		t.Fatalf("SpecFiles() error: %v", err)
	}

	classified := make(map[string]bool, len(matrix))
	for _, entry := range matrix {
		classified[entry.Spec] = true
	}

	for _, spec := range specs {
		if !classified[spec] {
			t.Errorf("%s is not classified in %s; add a row saying which operating systems it runs on, and why not the others",
				spec, MatrixFileName)
		}
	}
	for _, entry := range matrix {
		if !slices.Contains(specs, entry.Spec) {
			t.Errorf("%s classifies %s, which no longer exists", MatrixFileName, entry.Spec)
		}
	}
}

// TestOSMatrix_everyOSRunsSomething guards against a leg that silently became a
// no-op: a matrix where nothing is classified for Windows would make the Windows
// job pass by running nothing at all.
func TestOSMatrix_everyOSRunsSomething(t *testing.T) {
	t.Parallel()

	for _, goos := range SupportedOS {
		targets, err := TargetsFor(".", goos)
		if err != nil {
			t.Errorf("TargetsFor(%q) error: %v", goos, err)
			continue
		}
		if len(targets) == 0 {
			t.Errorf("no spec runs on %s", goos)
		}
		for _, target := range targets {
			if _, err := os.Stat(target); err != nil {
				t.Errorf("the %s target %s does not exist: %v", goos, target, err)
			}
		}
	}
}

// TestOSMatrix_linuxRunsEverySpec pins the baseline. Linux is where the suite
// was written and where every spec is expected to pass; a spec excluded there is
// almost certainly a mistake rather than a portability decision, and would
// otherwise reduce the coverage the other legs are compared against.
func TestOSMatrix_linuxRunsEverySpec(t *testing.T) {
	t.Parallel()

	matrix, err := LoadMatrix(".")
	if err != nil {
		t.Fatalf("LoadMatrix() error: %v", err)
	}
	for _, entry := range matrix {
		if !entry.RunsOn("linux") {
			t.Errorf("%s is excluded from Linux, the baseline every other leg is measured against", entry.Spec)
		}
	}
}

// TestOSMatrix_excludedSpecsExplainThemselves asserts the reasons are real
// sentences rather than a placeholder that satisfies the parser. A one-word
// reason tells the next person nothing about whether the exclusion is still
// justified.
func TestOSMatrix_excludedSpecsExplainThemselves(t *testing.T) {
	t.Parallel()

	matrix, err := LoadMatrix(".")
	if err != nil {
		t.Fatalf("LoadMatrix() error: %v", err)
	}
	const minReasonWords = 5
	for _, entry := range matrix {
		if len(entry.OS) == len(SupportedOS) {
			continue
		}
		if words := len(strings.Fields(entry.Reason)); words < minReasonWords {
			t.Errorf("%s excludes an OS with a %d-word reason (%q); say what actually stops it from running there",
				entry.Spec, words, entry.Reason)
		}
	}
}

// TestLoadMatrix_rejectsMalformedTables covers the parser's own guards: a table
// that silently accepted a typo'd OS name, a duplicate row, or a missing reason
// would let exactly the drift this file prevents back in.
func TestLoadMatrix_rejectsMalformedTables(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		table string
		want  string
	}{
		"unknown OS": {
			table: "a.atago.yaml\tlinux,plan9\treason enough words here\n",
			want:  "unknown OS",
		},
		"duplicate row": {
			table: "a.atago.yaml\tlinux,darwin,windows\na.atago.yaml\tlinux,darwin,windows\n",
			want:  "classified twice",
		},
		"missing reason": {
			table: "a.atago.yaml\tlinux\n",
			want:  "gives no reason",
		},
		"no OS listed": {
			table: "a.atago.yaml\t\treason enough words here\n",
			want:  "lists no operating system",
		},
		"missing column": {
			table: "a.atago.yaml\n",
			want:  "want <spec>",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, MatrixFileName), []byte(tt.table), 0o600); err != nil {
				t.Fatalf("failed to write the table: %v", err)
			}
			_, err := LoadMatrix(dir)
			if err == nil {
				t.Fatalf("LoadMatrix() accepted a malformed table: %q", tt.table)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("LoadMatrix() error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}
