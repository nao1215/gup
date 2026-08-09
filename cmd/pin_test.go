package cmd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/spf13/cobra"
)

const (
	pinTestTool   = "golangci-lint"
	pinTestImport = "github.com/golangci/golangci-lint/cmd/golangci-lint"
	pinTestGupImp = "github.com/nao1215/gup"
	pinTestV162   = "v1.62.0"
)

func TestParsePinArgs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		args        []string
		wantTarget  string
		wantVersion string
		wantErr     bool
	}{
		{name: "two args", args: []string{pinTestTool, pinTestV162}, wantTarget: pinTestTool, wantVersion: pinTestV162},
		{name: "at form", args: []string{pinTestTool + "@" + pinTestV162}, wantTarget: pinTestTool, wantVersion: pinTestV162},
		{name: "import path at form", args: []string{"github.com/x/y/cmd/z@" + testVersion123}, wantTarget: "github.com/x/y/cmd/z", wantVersion: testVersion123},
		{name: "single arg without version", args: []string{pinTestTool}, wantErr: true},
		{name: "empty version in at form", args: []string{pinTestTool + "@"}, wantErr: true},
		{name: "latest keyword rejected", args: []string{testBinTool, string(goutil.UpdateChannelLatest)}, wantErr: true},
		{name: "main keyword rejected", args: []string{testBinTool, string(goutil.UpdateChannelMain)}, wantErr: true},
		{name: "double version specification", args: []string{testBinTool + "@" + testVersion123, "v2.0.0"}, wantErr: true},
		{name: "empty target", args: []string{"@v1.0.0"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target, version, err := parsePinArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePinArgs(%v) expected error, got target=%q version=%q", tt.args, target, version)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePinArgs(%v) unexpected error: %v", tt.args, err)
			}
			if target != tt.wantTarget || version != tt.wantVersion {
				t.Errorf("parsePinArgs(%v) = (%q, %q), want (%q, %q)", tt.args, target, version, tt.wantTarget, tt.wantVersion)
			}
		})
	}
}

func TestResolvePinTarget(t *testing.T) {
	t.Parallel()
	installed := []goutil.Package{
		{Name: pinTestTool, ImportPath: pinTestImport},
		{Name: "gup", ImportPath: pinTestGupImp},
		{Name: "oldbin", ImportPath: ""}, // built by an old Go version: no import path
	}

	if p, err := resolvePinTarget(installed, pinTestTool); err != nil || p.ImportPath != pinTestImport {
		t.Errorf("by name: err=%v import=%q", err, p.ImportPath)
	}
	if p, err := resolvePinTarget(installed, pinTestImport); err != nil || p.Name != pinTestTool {
		t.Errorf("by import path: err=%v name=%q", err, p.Name)
	}
	if _, err := resolvePinTarget(installed, "not-installed"); err == nil {
		t.Error("an unmanaged tool must be rejected")
	}
	// Pinning a binary with no import path must fail fast: writing an entry with an
	// empty import_path would make ReadConfFile reject the whole config afterward.
	if _, err := resolvePinTarget(installed, "oldbin"); err == nil || !strings.Contains(err.Error(), "import path") {
		t.Errorf("pinning a binary without an import path must be rejected, got %v", err)
	}
}

func TestResolvePinTarget_ambiguousName(t *testing.T) {
	t.Parallel()
	// Two managed tools sharing a binary name must not silently pin the wrong one;
	// the caller is forced to disambiguate with the full import path.
	installed := []goutil.Package{
		{Name: testBinTool, ImportPath: "example.com/a/tool"},
		{Name: testBinTool, ImportPath: "example.com/b/tool"},
	}
	if _, err := resolvePinTarget(installed, testBinTool); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Errorf("ambiguous name must be rejected, got %v", err)
	}
	// The exact import path is still unambiguous.
	if p, err := resolvePinTarget(installed, "example.com/b/tool"); err != nil || p.ImportPath != "example.com/b/tool" {
		t.Errorf("exact import path should resolve: err=%v import=%q", err, p.ImportPath)
	}
}

func TestTargetPackage(t *testing.T) {
	t.Parallel()
	byName := targetPackage(pinTestTool)
	if byName.Name != pinTestTool || byName.ImportPath != "" {
		t.Errorf("targetPackage(name) = %+v", byName)
	}
	byPath := targetPackage("github.com/x/y/cmd/z")
	if byPath.ImportPath != "github.com/x/y/cmd/z" || byPath.Name != "z" {
		t.Errorf("targetPackage(path) = %+v", byPath)
	}
}

// TestParsePinArgs_tooManyArgs covers the default branch that rejects more than
// two arguments.
func TestParsePinArgs_tooManyArgs(t *testing.T) {
	t.Parallel()
	if _, _, err := parsePinArgs([]string{"a", "b", "c"}); err == nil {
		t.Fatal("parsePinArgs() err = nil, want error for three args")
	}
}

// Test_completeFirstArgPathBinaries guards the "one binary argument" contract of
// pin and unpin: cobra keeps calling ValidArgsFunction past the Args validator's
// maximum, so binaries must only be offered where one can still be accepted.
//
//nolint:paralleltest // t.Setenv pins $GOBIN, which forbids parallel subtests
func Test_completeFirstArgPathBinaries(t *testing.T) {
	t.Setenv("GOBIN", helper_makeBinaries(t, testBinGal, testBinSubaru))

	tests := []struct {
		name       string
		args       []string
		toComplete string
		want       []string
	}{
		{name: "first argument completes binaries", args: []string{}, want: []string{testBinGal, testBinSubaru}},
		{name: "first argument honors the typed prefix", args: []string{}, toComplete: "s", want: []string{testBinSubaru}},
		// pin's second argument is a VERSION, and unpin accepts no second
		// argument at all: neither takes another binary name.
		{name: "second argument offers nothing", args: []string{testBinGal}},
		{name: "argument past the maximum offers nothing", args: []string{testBinGal, testVersionOne}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, directive := completeFirstArgPathBinaries(nil, tt.args, tt.toComplete)
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive = %v, want NoFileComp", directive)
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("completions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Test_pinAndUnpinUseFirstArgCompletion keeps the two commands wired to the
// shared completion helper, so a future edit cannot silently regress one of them.
//
//nolint:paralleltest // t.Setenv pins $GOBIN, which forbids parallel subtests
func Test_pinAndUnpinUseFirstArgCompletion(t *testing.T) {
	t.Setenv("GOBIN", helper_makeBinaries(t, testBinGal))

	for _, tt := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: testCmdPin, cmd: newPinCmd()},
		{name: "unpin", cmd: newUnpinCmd()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.ValidArgsFunction == nil {
				t.Fatal("ValidArgsFunction is not registered")
			}
			got, _ := tt.cmd.ValidArgsFunction(tt.cmd, nil, "")
			if diff := cmp.Diff([]string{testBinGal}, got); diff != "" {
				t.Errorf("first argument completions mismatch (-want +got):\n%s", diff)
			}
			if got, _ := tt.cmd.ValidArgsFunction(tt.cmd, []string{testBinGal}, ""); len(got) != 0 {
				t.Errorf("second argument completions = %v, want none", got)
			}
		})
	}
}
