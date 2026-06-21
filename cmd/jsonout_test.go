//nolint:paralleltest // tests mutate global state (print.Stdout)
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
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
	out := captureCheckOutput(t, func() int {
		if err := encodeJSONPackages(nil); err != nil {
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

	out := captureCheckOutput(t, func() int {
		if err := encodeJSONPackages(recs); err != nil {
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

// readJSON runs fn (capturing stdout) and decodes the captured output as a
// slice of jsonPackage records, failing the test if it is not valid JSON.
func readJSON(t *testing.T, fn func() int) []jsonPackage {
	t.Helper()
	orgStdout := print.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw

	fn()

	_ = pw.Close()
	print.Stdout = orgStdout

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()

	var recs []jsonPackage
	if err := json.Unmarshal(buf.Bytes(), &recs); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, buf.String())
	}
	return recs
}

func Test_doCheck_jsonOutput(t *testing.T) {
	origLatest := getLatestVerCtx
	origRef := getVerByRefCtx
	t.Cleanup(func() {
		getLatestVerCtx = origLatest
		getVerByRefCtx = origRef
	})
	getLatestVerCtx = func(_ context.Context, _ string) (string, error) {
		return testVersionTwo, nil
	}
	getVerByRefCtx = func(_ context.Context, _ string, _ string) (string, error) {
		return testVersionNine, nil
	}

	pkgs := []goutil.Package{
		newCheckPkg("uptodate", testVersionTwo, goutil.UpdateChannelLatest), // == latest -> up-to-date
		newCheckPkg("outdated", testVersionOne, goutil.UpdateChannelLatest), // < latest -> update-available
	}

	recs := readJSON(t, func() int {
		return doCheckJSON(pkgs, 1, 0, true)
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
	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	t.Cleanup(func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
	})
	// tool: outdated -> will be updated; uptodate: already current -> up-to-date
	getLatestVer = func(string) (string, error) { return testVersionTwo, nil }
	installLatest = func(string) error { return nil }

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
	out := captureCheckOutput(t, func() int {
		result, _, _ = updateWithChannels(pkgs, false, false, 1, true, channelMap, 0, true)
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
	origLatest := getLatestVerCtx
	t.Cleanup(func() { getLatestVerCtx = origLatest })
	getLatestVerCtx = func(_ context.Context, _ string) (string, error) {
		return "", errors.New("module not found")
	}

	pkgs := []goutil.Package{
		newCheckPkg("brokentool", testVersionOne, goutil.UpdateChannelLatest),
	}

	recs := readJSON(t, func() int {
		return doCheckJSON(pkgs, 1, 0, true)
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

	origGetLatest := getLatestVer
	t.Cleanup(func() { getLatestVer = origGetLatest })
	getLatestVer = func(string) (string, error) { return testVersionNine, nil }

	cmd := newCheckCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}

	recs := readJSON(t, func() int {
		return check(cmd, []string{})
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

	recs := readJSON(t, func() int {
		return list(cmd, []string{})
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

// Test_gup_jsonFlag exercises gup() with --json (and --dry-run) so the
// flag-parsing and JSON-dispatch branches in gup()/updateWithChannels are
// covered without performing real installs.
func Test_gup_jsonFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	t.Cleanup(func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
	})
	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	installLatest = func(string) error { return nil }

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("failed to set dry-run flag: %v", err)
	}

	var got int
	recs := readJSON(t, func() int {
		got = gup(cmd, []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("gup() = %d, want 0", got)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one JSON record from update --json")
	}
}
