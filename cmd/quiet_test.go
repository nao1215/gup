//nolint:paralleltest // these tests mutate global stub variables
package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
)

const (
	quietImportUpdated  = "github.com/example/updated"
	quietImportUpToDate = "github.com/example/uptodate"
	quietNameUpdated    = "updated"
	quietNameUpToDate   = "uptodate"
)

// quietMixedPkgs returns one package that needs an update and one that is
// already up to date, so the quiet-mode filtering and summary can be exercised.
func quietMixedPkgs() []goutil.Package {
	return []goutil.Package{
		{
			Name:       quietNameUpdated,
			ImportPath: quietImportUpdated,
			ModulePath: quietImportUpdated,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
		{
			Name:       quietNameUpToDate,
			ImportPath: quietImportUpToDate,
			ModulePath: quietImportUpToDate,
			Version:    &goutil.Version{Current: testVersionNine},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}
}

func Test_updateWithChannels_quiet(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	deps.installLatest = func(context.Context, string) error { return nil }

	pkgs := quietMixedPkgs()
	channelMap := map[string]goutil.UpdateChannel{
		quietNameUpdated:  goutil.UpdateChannelLatest,
		quietNameUpToDate: goutil.UpdateChannelLatest,
	}

	var got int
	out := captureCheckOutput(t, func() int {
		got, _, _ = updateWithChannels(deps, pkgs, false, false, 1, true, channelMap, nil, 0, false, true)
		return got
	})
	if got != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", got)
	}

	if strings.Contains(out, "update binary under") {
		t.Errorf("quiet mode should suppress the header, got:\n%s", out)
	}
	if strings.Contains(out, quietImportUpToDate) {
		t.Errorf("quiet mode should not print up-to-date binaries, got:\n%s", out)
	}
	if !strings.Contains(out, quietImportUpdated) {
		t.Errorf("quiet mode should print the updated binary, got:\n%s", out)
	}
	if !strings.Contains(out, "gup: 1 updated, 1 up-to-date, 0 failed") {
		t.Errorf("quiet mode should print a summary, got:\n%s", out)
	}
}

func Test_updateWithChannels_quiet_failed(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	deps.installLatest = func(context.Context, string) error { return errors.New("install failed") }

	pkgs := []goutil.Package{
		{
			Name:       quietNameUpdated,
			ImportPath: quietImportUpdated,
			ModulePath: quietImportUpdated,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}
	channelMap := map[string]goutil.UpdateChannel{quietNameUpdated: goutil.UpdateChannelLatest}

	var got int
	out := captureCheckOutput(t, func() int {
		got, _, _ = updateWithChannels(deps, pkgs, false, false, 1, true, channelMap, nil, 0, false, true)
		return got
	})
	if got != 1 {
		t.Fatalf("updateWithChannels() = %d, want 1 (one package failed)", got)
	}
	// The failure is counted in the summary and the error is still surfaced
	// (captureCheckOutput merges STDOUT and STDERR).
	if !strings.Contains(out, "gup: 0 updated, 0 up-to-date, 1 failed") {
		t.Errorf("quiet summary should count the failure, got:\n%s", out)
	}
	if !strings.Contains(out, "install failed") {
		t.Errorf("the failure cause should still be printed, got:\n%s", out)
	}
}

func Test_doCheck_quiet(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }

	pkgs := quietMixedPkgs()

	var got int
	out := captureCheckOutput(t, func() int {
		got = doCheck(deps, pkgs, 1, 0, true, true)
		return got
	})
	if got != 0 {
		t.Fatalf("doCheck() = %d, want 0", got)
	}

	if strings.Contains(out, "check binary under") {
		t.Errorf("quiet mode should suppress the header, got:\n%s", out)
	}
	if strings.Contains(out, quietImportUpToDate) {
		t.Errorf("quiet mode should not print up-to-date binaries, got:\n%s", out)
	}
	if !strings.Contains(out, quietImportUpdated) {
		t.Errorf("quiet mode should print the update-available binary, got:\n%s", out)
	}
	// The "$ gup update <names>" hint is kept (it is the actionable output).
	if !strings.Contains(out, "$ gup update "+quietNameUpdated) {
		t.Errorf("quiet mode should keep the update hint, got:\n%s", out)
	}
	if !strings.Contains(out, "gup: 1 update available, 1 up-to-date, 0 failed") {
		t.Errorf("quiet mode should print a summary, got:\n%s", out)
	}
}

// Test_updateWithChannels_jsonQuiet verifies that --json takes precedence over
// --quiet: STDOUT stays a valid JSON array with no summary line mixed in.
func Test_updateWithChannels_jsonQuiet(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	deps.installLatest = func(context.Context, string) error { return nil }

	pkgs := quietMixedPkgs()
	channelMap := map[string]goutil.UpdateChannel{
		quietNameUpdated:  goutil.UpdateChannelLatest,
		quietNameUpToDate: goutil.UpdateChannelLatest,
	}

	recs := readJSON(t, func() int {
		got, _, _ := updateWithChannels(deps, pkgs, false, false, 1, true, channelMap, nil, 0, true, true)
		return got
	})
	if len(recs) != 2 {
		t.Fatalf("expected 2 JSON records, got %d", len(recs))
	}
}
