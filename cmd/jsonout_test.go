//nolint:paralleltest // tests mutate process-global state (GOBIN/XDG env, cwd)
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

const (
	testImportExampleTool     = "example.com/tool"
	testImportExampleUpToDate = "example.com/uptodate"
)

func Test_newJSONPackage(t *testing.T) {
	pkg := goutil.Package{
		Name:       testBinTool,
		ImportPath: testImportExampleTool,
		ModulePath: testImportExampleTool,
		Version:    &goutil.Version{Current: testVersionOne, Latest: testVersionTwo},
		GoVersion:  &goutil.Version{Current: "go1.21.0", Latest: testGoVersion1224},
		// empty channel must normalize to "latest"
	}

	got := newJSONPackage(pkg, statusUpdateAvailable, nil)

	want := jsonPackage{
		Name:               testBinTool,
		ImportPath:         testImportExampleTool,
		ModulePath:         testImportExampleTool,
		Channel:            string(goutil.UpdateChannelLatest),
		CurrentVersion:     testVersionOne,
		LatestVersion:      testVersionTwo,
		CurrentGoVersion:   "go1.21.0",
		InstalledGoVersion: testGoVersion1224,
		Status:             statusUpdateAvailable,
	}
	if got != want {
		t.Errorf("newJSONPackage() = %+v, want %+v", got, want)
	}
}

func Test_newJSONPackage_nilVersionsAndError(t *testing.T) {
	pkg := goutil.Package{
		Name:          "broken",
		ImportPath:    "example.com/broken",
		UpdateChannel: goutil.UpdateChannelMain,
	}

	got := newJSONPackage(pkg, statusError, errors.New("boom"))

	if got.CurrentVersion != "" || got.LatestVersion != "" {
		t.Errorf("expected empty versions for nil Version, got %+v", got)
	}
	if got.CurrentGoVersion != "" || got.InstalledGoVersion != "" {
		t.Errorf("expected empty go versions for nil GoVersion, got %+v", got)
	}
	if got.Channel != string(goutil.UpdateChannelMain) {
		t.Errorf("channel = %q, want main", got.Channel)
	}
	if got.Status != statusError || got.Error != "boom" {
		t.Errorf("status/error = %q/%q, want error/boom", got.Status, got.Error)
	}
}

func Test_encodeJSONPackages_empty(t *testing.T) {
	out := captureCheckOutput(t, func(p *print.Printer) int {
		if err := encodeJSONPackages(p, nil); err != nil {
			t.Fatalf("encodeJSONPackages() error = %v", err)
		}
		return 0
	})
	// Must be a valid empty JSON array, not "null".
	trimmed := strings.TrimSpace(out)
	if trimmed != "[]" {
		t.Fatalf("encodeJSONPackages(nil) = %q, want []", trimmed)
	}
}

func Test_encodeJSONPackages_validJSON(t *testing.T) {
	recs := []jsonPackage{
		newJSONPackage(goutil.Package{
			Name:       "a",
			ImportPath: "example.com/a",
			Version:    &goutil.Version{Current: testVersionOne, Latest: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		}, statusUpToDate, nil),
		newJSONPackage(goutil.Package{
			Name:       "b",
			ImportPath: "example.com/b",
		}, statusError, errors.New("network down")),
	}

	out := captureCheckOutput(t, func(p *print.Printer) int {
		if err := encodeJSONPackages(p, recs); err != nil {
			t.Fatalf("encodeJSONPackages() error = %v", err)
		}
		return 0
	})

	var decoded []jsonPackage
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded %d records, want 2", len(decoded))
	}
	if decoded[0].Status != statusUpToDate || decoded[1].Status != statusError {
		t.Fatalf("unexpected statuses: %q, %q", decoded[0].Status, decoded[1].Status)
	}
	if decoded[1].Error != "network down" {
		t.Fatalf("error field = %q, want 'network down'", decoded[1].Error)
	}
}

// readJSON runs fn with a Printer whose stdout and stderr are SEPARATE buffers,
// then decodes only the stdout buffer as a slice of jsonPackage records. Keeping
// the streams separate is the point: it verifies the #291 contract that warnings
// and errors (written to stderr) never contaminate the machine-readable JSON on
// stdout.
func readJSON(t *testing.T, fn func(p *print.Printer) int) []jsonPackage {
	t.Helper()
	var out, errBuf bytes.Buffer
	fn(print.New(&out, &errBuf))

	var recs []jsonPackage
	if err := json.Unmarshal(out.Bytes(), &recs); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nSTDOUT:\n%s\nSTDERR:\n%s", err, out.String(), errBuf.String())
	}
	return recs
}

func Test_doCheck_jsonOutput(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(_ context.Context, _ string) (string, error) {
		return testVersionTwo, nil
	}
	deps.getVerByRef = func(_ context.Context, _ string, _ string) (string, error) {
		return testVersionNine, nil
	}

	pkgs := []goutil.Package{
		newCheckPkg("uptodate", testVersionTwo, goutil.UpdateChannelLatest), // == latest -> up-to-date
		newCheckPkg("outdated", testVersionOne, goutil.UpdateChannelLatest), // < latest -> update-available
	}

	recs := readJSON(t, func(p *print.Printer) int {
		return doCheckJSON(deps, p, pkgs, 1, 0, true)
	})

	got := map[string]string{}
	for _, r := range recs {
		got[r.Name] = r.Status
	}
	if got["uptodate"] != statusUpToDate {
		t.Errorf("uptodate status = %q, want %q", got["uptodate"], statusUpToDate)
	}
	if got["outdated"] != statusUpdateAvailable {
		t.Errorf("outdated status = %q, want %q", got["outdated"], statusUpdateAvailable)
	}
}

// Test_doCheckJSON_stableInputOrder verifies the #365 contract end-to-end for
// check --json: even when the latest-version lookup resolves earlier packages
// last (inverting completion order), the emitted JSON array stays in input
// order.
func Test_doCheckJSON_stableInputOrder(t *testing.T) {
	const total = 6
	pkgs := make([]goutil.Package, total)
	sleepByModule := make(map[string]time.Duration, total)
	for i := range total {
		p := newCheckPkg(fmt.Sprintf("tool-%d", i), testVersionTwo, goutil.UpdateChannelLatest)
		pkgs[i] = p
		// Earlier packages resolve slower so they complete last.
		sleepByModule[p.ModulePath] = time.Duration(total-i) * 3 * time.Millisecond
	}

	deps := testDeps()
	deps.getLatestVer = func(_ context.Context, modulePath string) (string, error) {
		time.Sleep(sleepByModule[modulePath])
		return testVersionTwo, nil
	}

	recs := readJSON(t, func(p *print.Printer) int {
		return doCheckJSON(deps, p, pkgs, total, 0, true)
	})

	if len(recs) != total {
		t.Fatalf("got %d records, want %d", len(recs), total)
	}
	for i, r := range recs {
		if want := fmt.Sprintf("tool-%d", i); r.Name != want {
			t.Errorf("records[%d].Name = %q, want %q (stable input order)", i, r.Name, want)
		}
	}
}

func Test_listJSONRecords(t *testing.T) {
	pkgs := []goutil.Package{
		{
			Name:          testBinTool,
			ImportPath:    testImportExampleTool,
			ModulePath:    testImportExampleTool,
			Version:       &goutil.Version{Current: testVersion123},
			UpdateChannel: goutil.UpdateChannelMaster,
		},
	}

	recs := listJSONRecords(pkgs)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	r := recs[0]
	if r.Status != statusInstalled {
		t.Errorf("status = %q, want %q", r.Status, statusInstalled)
	}
	if r.CurrentVersion != testVersion123 {
		t.Errorf("current_version = %q, want %s", r.CurrentVersion, testVersion123)
	}
	if r.Channel != string(goutil.UpdateChannelMaster) {
		t.Errorf("channel = %q, want master", r.Channel)
	}
	// list does not query latest/go versions.
	if r.LatestVersion != "" || r.CurrentGoVersion != "" {
		t.Errorf("expected empty latest/go versions for list, got %+v", r)
	}
}

func Test_updateWithChannels_jsonOutput(t *testing.T) {
	deps := testDeps()
	// tool: outdated -> will be updated; uptodate: already current -> up-to-date
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionTwo, nil }
	deps.installLatest = func(context.Context, string) error { return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportExampleTool,
			ModulePath: testImportExampleTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
		{
			Name:       "uptodate",
			ImportPath: testImportExampleUpToDate,
			ModulePath: testImportExampleUpToDate,
			Version:    &goutil.Version{Current: testVersionTwo},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}
	channelMap := map[string]goutil.UpdateChannel{
		testBinTool: goutil.UpdateChannelLatest,
		"uptodate":  goutil.UpdateChannelLatest,
	}

	var recs []jsonPackage
	var result int
	out := captureCheckOutput(t, func(p *print.Printer) int {
		result, _, _ = updateWithChannels(deps, p, pkgs, false, false, 1, true, channelMap, nil, 0, true, false)
		return result
	})

	if err := json.Unmarshal([]byte(out), &recs); err != nil {
		t.Fatalf("update --json output is not valid JSON: %v\n%s", err, out)
	}
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	got := map[string]string{}
	for _, r := range recs {
		got[r.Name] = r.Status
	}
	if got["tool"] != statusUpdated {
		t.Errorf("tool status = %q, want %q", got["tool"], statusUpdated)
	}
	if got["uptodate"] != statusUpToDate {
		t.Errorf("uptodate status = %q, want %q", got["uptodate"], statusUpToDate)
	}
}

func Test_doCheck_jsonOutput_errorRecord(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(_ context.Context, _ string) (string, error) {
		return "", errors.New("module not found")
	}

	pkgs := []goutil.Package{
		newCheckPkg("brokentool", testVersionOne, goutil.UpdateChannelLatest),
	}

	recs := readJSON(t, func(p *print.Printer) int {
		return doCheckJSON(deps, p, pkgs, 1, 0, true)
	})
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0].Status != statusError {
		t.Errorf("status = %q, want %q", recs[0].Status, statusError)
	}
	if recs[0].Error == "" {
		t.Errorf("expected non-empty error field")
	}
}

// Test_check_jsonFlag exercises the command entry point with --json so the
// flag-parsing and JSON-dispatch branches in check() are covered.
func Test_check_jsonFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }

	cmd := newCheckCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}

	recs := readJSON(t, func(p *print.Printer) int {
		return check(deps, p, cmd, []string{})
	})
	if len(recs) == 0 {
		t.Fatal("expected at least one JSON record from check --json")
	}
	for _, r := range recs {
		if r.Status == "" {
			t.Errorf("record %q has empty status", r.Name)
		}
	}
}

// Test_list_jsonFlag exercises list() with --json end to end.
func Test_list_jsonFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newListCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}

	recs := readJSON(t, func(p *print.Printer) int {
		return list(p, cmd, []string{})
	})
	if len(recs) == 0 {
		t.Fatal("expected at least one JSON record from list --json")
	}
	for _, r := range recs {
		if r.Status != statusInstalled {
			t.Errorf("list record %q status = %q, want %q", r.Name, r.Status, statusInstalled)
		}
	}
}

// Test_list_jsonFlag_ambiguousConfigFailsFast verifies the #364 contract: list
// --json uses the same config-resolution rules as check/update/import, so when
// both the user-level config and ./gup.json exist and no --file is given, it
// fails fast with the ambiguity error instead of silently picking one. An
// explicit --file resolves the ambiguity and annotates the saved channel.
func Test_list_jsonFlag_ambiguousConfigFailsFast(t *testing.T) {
	gobin, err := filepath.Abs(filepath.Join("testdata", "check_success"))
	if err != nil {
		t.Fatal(err)
	}
	setupXDGBase(t)
	chdirToTemp(t)
	t.Setenv("GOBIN", gobin)

	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	canonical := `{"schema_version":1,"packages":[{"name":"posixer","import_path":"` + testImportPathPosixer + `","version":"v0.1.0","channel":"main"}]}` + "\n"
	local := `{"schema_version":1,"packages":[{"name":"posixer","import_path":"` + testImportPathPosixer + `","version":"v0.1.0","channel":"master"}]}` + "\n"
	if err := os.WriteFile(config.FilePath(), []byte(canonical), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalFilePath(), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}

	// No --file: the ambiguous config must make list --json fail fast.
	ambiguousCmd := newListCmd()
	if err := ambiguousCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	var ambiguousGot int
	out := captureCheckOutput(t, func(p *print.Printer) int {
		ambiguousGot = list(p, ambiguousCmd, []string{})
		return ambiguousGot
	})
	if ambiguousGot != 1 {
		t.Errorf("list --json = %d, want 1 on ambiguous config", ambiguousGot)
	}
	if !strings.Contains(out, "multiple gup.json") || !strings.Contains(out, "--file") {
		t.Errorf("expected ambiguity error mentioning --file, got: %s", out)
	}

	// Explicit --file resolves the ambiguity and annotates the saved channel.
	cmd := newListCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	if err := cmd.Flags().Set("file", config.LocalFilePath()); err != nil {
		t.Fatalf("failed to set file flag: %v", err)
	}

	var got int
	recs := readJSON(t, func(p *print.Printer) int {
		got = list(p, cmd, []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("list --json --file should succeed in ambiguous repo state, got %d", got)
	}
	foundPosixer := false
	for _, r := range recs {
		if r.ImportPath == testImportPathPosixer {
			foundPosixer = true
			if r.Channel != string(goutil.UpdateChannelMaster) {
				t.Errorf("posixer channel = %q, want master (from explicit --file ./gup.json)", r.Channel)
			}
		}
	}
	if !foundPosixer {
		t.Fatal("expected posixer record in list --json output")
	}
}

// Test_emptyEnv_jsonEmitsEmptyArray verifies the #350 contract: in an empty
// environment, list/check/update --json emit a valid empty JSON array and exit 0
// instead of printing an error.
func Test_emptyEnv_jsonEmitsEmptyArray(t *testing.T) {
	emptyGobin := t.TempDir()

	t.Run("list --json", func(t *testing.T) {
		t.Setenv("GOBIN", emptyGobin)
		cmd := newListCmd()
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatal(err)
		}
		var got int
		recs := readJSON(t, func(p *print.Printer) int { got = list(p, cmd, nil); return got })
		if got != 0 {
			t.Fatalf("list --json on empty env = %d, want 0", got)
		}
		if len(recs) != 0 {
			t.Fatalf("list --json on empty env should be [], got %d records", len(recs))
		}
	})

	t.Run("check --json", func(t *testing.T) {
		t.Setenv("GOBIN", emptyGobin)
		cmd := newCheckCmd()
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatal(err)
		}
		var got int
		recs := readJSON(t, func(p *print.Printer) int { got = check(defaultDependencies(), p, cmd, nil); return got })
		if got != 0 {
			t.Fatalf("check --json on empty env = %d, want 0", got)
		}
		if len(recs) != 0 {
			t.Fatalf("check --json on empty env should be [], got %d records", len(recs))
		}
	})

	t.Run("update --json", func(t *testing.T) {
		t.Setenv("GOBIN", emptyGobin)
		cmd := newUpdateCmd()
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatal(err)
		}
		var got int
		recs := readJSON(t, func(p *print.Printer) int { got = gup(defaultDependencies(), p, cmd, nil); return got })
		if got != 0 {
			t.Fatalf("update --json on empty env = %d, want 0", got)
		}
		if len(recs) != 0 {
			t.Fatalf("update --json on empty env should be [], got %d records", len(recs))
		}
	})
}

// Test_gup_jsonFlag exercises gup() with --json (and --dry-run) so the
// flag-parsing and JSON-dispatch branches in gup()/updateWithChannels are
// covered without performing real installs.
func Test_gup_jsonFlag(t *testing.T) {
	deps := helper_stubUpdateForJSON(t, func(context.Context, string) error { return nil })

	cmd := helper_newJSONDryRunUpdateCmd(t)

	var got int
	recs := readJSON(t, func(p *print.Printer) int {
		got = gup(deps, p, cmd, []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("gup() = %d, want 0", got)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one JSON record from update --json")
	}
}

// helper_stubUpdateForJSON returns dependencies whose update operations are
// stubbed (latest == v9.9.9, the given install func) and points $GOBIN at the
// check_success fixtures so gup() can run --json end-to-end without real
// installs. The caller passes the returned deps to gup(), and readJSON fails the
// test if STDOUT is not valid JSON, which is exactly the contamination this
// issue (#291) guards against.
func helper_stubUpdateForJSON(t *testing.T, install func(context.Context, string) error) dependencies {
	t.Helper()
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	deps.installLatest = install
	return deps
}

// helper_newJSONDryRunUpdateCmd returns an update command with --json and
// --dry-run already set, so JSON-mode regression tests don't repeat the flag
// wiring (and perform no real installs).
func helper_newJSONDryRunUpdateCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("failed to set dry-run flag: %v", err)
	}
	return cmd
}

// Test_gup_jsonFlag_excludeKeepsStdoutPure verifies that --json keeps STDOUT
// pure JSON even when --exclude would print a human-readable "Exclude ..." line
// in normal mode. This is the core regression for issue #291: before the fix,
// excludePkgs wrote that line to STDOUT (print.Info) and broke JSON parsing.
func Test_gup_jsonFlag_excludeKeepsStdoutPure(t *testing.T) {
	deps := helper_stubUpdateForJSON(t, func(context.Context, string) error { return nil })

	cmd := helper_newJSONDryRunUpdateCmd(t)
	if err := cmd.Flags().Set("exclude", testBinPosixer); err != nil {
		t.Fatalf("failed to set exclude flag: %v", err)
	}

	var got int
	recs := readJSON(t, func(p *print.Printer) int {
		got = gup(deps, p, cmd, []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("gup() = %d, want 0", got)
	}
	if len(recs) == 0 {
		t.Fatal("expected JSON records for the non-excluded packages")
	}
	for _, r := range recs {
		if strings.Contains(r.Name, testBinPosixer) {
			t.Errorf("excluded package %q must not appear in JSON output", r.Name)
		}
	}
}

// Test_gup_jsonFlag_missingFlagTargetKeepsStdoutPure verifies that a "not found"
// warning for an unknown --main target (emitted via print.Warn to STDERR) does
// not contaminate the JSON written to STDOUT, while the valid packages are still
// reported. This pins STDOUT purity for the missing-targets edge case.
func Test_gup_jsonFlag_missingFlagTargetKeepsStdoutPure(t *testing.T) {
	deps := helper_stubUpdateForJSON(t, func(context.Context, string) error { return nil })

	cmd := helper_newJSONDryRunUpdateCmd(t)
	if err := cmd.Flags().Set("main", "doesnotexist"); err != nil {
		t.Fatalf("failed to set main flag: %v", err)
	}

	var got int
	recs := readJSON(t, func(p *print.Printer) int {
		got = gup(deps, p, cmd, []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("gup() = %d, want 0", got)
	}
	if len(recs) == 0 {
		t.Fatal("expected JSON records even when an unknown --main target is given")
	}
}

// Test_gup_jsonFlag_partialFailureKeepsStdoutPure verifies that when one package
// fails to install, its error (emitted via print.Err to STDERR) does not
// contaminate STDOUT and the JSON still reports a per-package error status.
func Test_gup_jsonFlag_partialFailureKeepsStdoutPure(t *testing.T) {
	deps := helper_stubUpdateForJSON(t, func(_ context.Context, importPath string) error {
		if strings.Contains(importPath, testBinPosixer) {
			return errors.New("install failed")
		}
		return nil
	})

	cmd := helper_newJSONDryRunUpdateCmd(t)

	var got int
	recs := readJSON(t, func(p *print.Printer) int {
		got = gup(deps, p, cmd, []string{})
		return got
	})
	if got != 1 {
		t.Fatalf("gup() = %d, want 1 (one package failed)", got)
	}

	statusByName := map[string]string{}
	for _, r := range recs {
		statusByName[r.Name] = r.Status
	}
	if statusByName[testBinPosixer] != statusError {
		t.Errorf("%s status = %q, want %q", testBinPosixer, statusByName[testBinPosixer], statusError)
	}
	for name, status := range statusByName {
		if name != testBinPosixer && status == statusError {
			t.Errorf("package %q unexpectedly reported error status", name)
		}
	}
}
