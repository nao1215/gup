package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/goutil"
)

// These tests moved from internal/goutil when the version-display logic was
// relocated into the cmd layer; they assert the rendered strings (including
// color codes), so the presentation behavior is preserved.

const (
	rvName   = "foo"
	rvImport = "github.com/dummy_name/dummy"
	rvModule = "github.com/dummy_name/dummy/foo"

	rvV001 = "v0.0.1"
	rvV100 = "v1.0.0"
	rvV110 = "v1.1.0"
	rvV191 = "v1.9.1"

	rvGoCur     = "go1.23.0"
	rvGoOld     = "go1.22.0"
	rvGo1221    = "go1.22.1"
	rvGo1250ND  = "go1.25.0-X:nodwarf5"
	rvGoCurrent = testGoVersion1224     // "go1.22.4"
	rvGo1260ND  = testGoVersionNoDwarf5 // "go1.26.0-X:nodwarf5"
)

func TestCurrentToLatestStr_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: "v1.42.2", Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := "up-to-date: v1.42.2"
	if got := currentToLatestStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestCurrentToLatestStr_not_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV001, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := "v0.0.1 to v1.9.1"
	if got := currentToLatestStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

//nolint:paralleltest // mutates the global color.NoColor
func TestCurrentToLatestStr_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV001, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := color.YellowString(rvV001) + " to " + color.GreenString(rvV191)
	if got := currentToLatestStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestVersionCheckResultStr_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: "v2.5.0", Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := "up-to-date: v2.5.0"
	if got := versionCheckResultStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestVersionCheckResultStr_not_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV001, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := "current: v0.0.1, latest: v1.9.1"
	if got := versionCheckResultStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

//nolint:paralleltest // mutates the global color.NoColor
func TestVersionCheckResultStr_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV001, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGoCurrent},
	}

	wantContain := "current: " + color.YellowString(rvV001) + ", latest: " + color.GreenString(rvV191)
	if got := versionCheckResultStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestVersionCheckResultStr_go_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: "go1.99.9", Latest: rvGoCurrent},
	}

	wantContain := regexp.MustCompile(`up-to-date:.* go1\.99\.9`)
	if got := versionCheckResultStr(pkgInfo); !wantContain.MatchString(got) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestVersionCheckResultStr_go_not_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1221, Latest: rvGoCurrent},
	}

	wantContain := "current: go1.22.1, installed: go1.22.4"
	if got := versionCheckResultStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

//nolint:paralleltest // mutates the global color.NoColor
func TestVersionCheckResultStr_go_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		ModulePath: rvModule,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1221, Latest: rvGoCurrent},
	}

	wantContain := "current: " + color.YellowString(rvGo1221) + ", installed: " + color.GreenString(rvGoCurrent)
	if got := versionCheckResultStr(pkgInfo); !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestCurrentToLatestStr_go_not_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1221, Latest: rvGoCurrent},
	}

	got := currentToLatestStr(pkgInfo)
	if !strings.Contains(got, rvGo1221) || !strings.Contains(got, rvGoCurrent) {
		t.Errorf("expected go version range, got: %v", got)
	}
}

func TestCurrentToLatestStr_go_customBuildTag_color(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1250ND, Latest: rvGo1260ND},
	}

	got := currentToLatestStr(pkgInfo)
	if !strings.Contains(got, rvGo1250ND) {
		t.Fatalf("expected current custom go version in output, got: %q", got)
	}
	if !strings.Contains(got, rvGo1260ND) {
		t.Fatalf("expected latest custom go version in output, got: %q", got)
	}
}

func TestCurrentToLatestStr_both_not_up_to_date(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		Version:    &goutil.Version{Current: rvV001, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1221, Latest: rvGoCurrent},
	}

	got := currentToLatestStr(pkgInfo)
	if !strings.Contains(got, rvV001) || !strings.Contains(got, rvV191) {
		t.Errorf("expected package version range, got: %v", got)
	}
	if !strings.Contains(got, rvGo1221) || !strings.Contains(got, rvGoCurrent) {
		t.Errorf("expected go version range, got: %v", got)
	}
}

// TestCurrentToLatestStr_customBuildTagEqual mirrors the display half of the old
// goutil IsGoUpToDate custom-build-tag test: when a custom Go build tag compares
// equal, the package is rendered as already up-to-date.
func TestCurrentToLatestStr_customBuildTagEqual(t *testing.T) {
	t.Parallel()
	pkgInfo := goutil.Package{
		Name:       rvName,
		ImportPath: rvImport,
		Version:    &goutil.Version{Current: rvV191, Latest: rvV191},
		GoVersion:  &goutil.Version{Current: rvGo1260ND, Latest: rvGo1260ND},
	}

	if got := currentToLatestStr(pkgInfo); !strings.Contains(got, "Already up-to-date") {
		t.Fatalf("currentToLatestStr() = %q, want to include 'Already up-to-date'", got)
	}
}

func TestPinnedResultStr(t *testing.T) {
	t.Parallel()

	// Satisfied pin (version matches, Go current): "pinned <ver>".
	satisfied := goutil.Package{
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: rvV100,
		Version:       &goutil.Version{Current: rvV100},
		GoVersion:     &goutil.Version{Current: rvGoCur, Latest: rvGoCur},
	}
	if got := pinnedResultStr(satisfied); !strings.Contains(got, "pinned") || !strings.Contains(got, rvV100) {
		t.Errorf("satisfied pinnedResultStr() = %q, want it to mention pinned v1.0.0", got)
	}
	if got := pinnedResultStr(satisfied); strings.Contains(got, "installed") || strings.Contains(got, "go1.2") {
		t.Errorf("satisfied pinnedResultStr() = %q, want no version/Go delta", got)
	}

	// Version mismatch: "pinned <ver>, installed <other>".
	mismatch := goutil.Package{
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: rvV100,
		Version:       &goutil.Version{Current: rvV110},
		GoVersion:     &goutil.Version{Current: rvGoCur, Latest: rvGoCur},
	}
	if got := pinnedResultStr(mismatch); !strings.Contains(got, "installed") || !strings.Contains(got, rvV110) {
		t.Errorf("mismatch pinnedResultStr() = %q, want it to mention the installed version", got)
	}

	// Version matches but built with an older Go: a pending rebuild is surfaced.
	staleGo := goutil.Package{
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: rvV100,
		Version:       &goutil.Version{Current: rvV100},
		GoVersion:     &goutil.Version{Current: rvGoOld, Latest: rvGoCur},
	}
	if got := pinnedResultStr(staleGo); !strings.Contains(got, rvGoOld) || !strings.Contains(got, rvGoCur) {
		t.Errorf("stale-Go pinnedResultStr() = %q, want it to show the Go transition", got)
	}

	// Missing installed version falls back to "unknown".
	noCurrent := goutil.Package{
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: rvV100,
		Version:       &goutil.Version{Current: ""},
		GoVersion:     &goutil.Version{Current: rvGoCur, Latest: rvGoCur},
	}
	if got := pinnedResultStr(noCurrent); !strings.Contains(got, "unknown") {
		t.Errorf("no-current pinnedResultStr() = %q, want it to mention unknown", got)
	}
}

// TestHideIgnoredGoDelta verifies the seam that suppresses an ignored Go
// toolchain delta before rendering. When ignoreGoUpdate is set (and not in JSON
// mode), a Go-only delta is collapsed so currentToLatestStr/versionCheckResultStr
// render a clean "Already up-to-date" line. It must be a no-op when the delta is
// not being ignored or when emitting JSON, and tolerate a nil GoVersion.
func TestHideIgnoredGoDelta(t *testing.T) {
	t.Parallel()

	t.Run("suppresses Go-only delta when ignored", func(t *testing.T) {
		t.Parallel()
		p := goutil.Package{
			Name:       rvName,
			ImportPath: rvImport,
			ModulePath: rvModule,
			Version:    &goutil.Version{Current: rvV100, Latest: rvV100},
			GoVersion:  &goutil.Version{Current: rvGoCurrent, Latest: rvGo1260ND},
		}
		hideIgnoredGoDelta(&p, true, false)
		if p.GoVersion.Latest != p.GoVersion.Current {
			t.Fatalf("GoVersion.Latest = %q, want collapsed to %q", p.GoVersion.Latest, p.GoVersion.Current)
		}
		if got := currentToLatestStr(p); !strings.Contains(got, "up-to-date") {
			t.Fatalf("currentToLatestStr after suppression = %q, want it to read up-to-date", got)
		}
		if strings.Contains(versionCheckResultStr(p), rvGo1260ND) {
			t.Fatalf("versionCheckResultStr still shows the ignored Go latest %q", rvGo1260ND)
		}
	})

	t.Run("no-op when not ignoring", func(t *testing.T) {
		t.Parallel()
		p := goutil.Package{
			Version:   &goutil.Version{Current: rvV100, Latest: rvV100},
			GoVersion: &goutil.Version{Current: rvGoCurrent, Latest: rvGo1260ND},
		}
		hideIgnoredGoDelta(&p, false, false)
		if p.GoVersion.Latest != rvGo1260ND {
			t.Fatalf("GoVersion.Latest = %q, want unchanged %q when not ignoring", p.GoVersion.Latest, rvGo1260ND)
		}
	})

	t.Run("no-op in JSON mode", func(t *testing.T) {
		t.Parallel()
		p := goutil.Package{
			Version:   &goutil.Version{Current: rvV100, Latest: rvV100},
			GoVersion: &goutil.Version{Current: rvGoCurrent, Latest: rvGo1260ND},
		}
		hideIgnoredGoDelta(&p, true, true)
		if p.GoVersion.Latest != rvGo1260ND {
			t.Fatalf("GoVersion.Latest = %q, want unchanged %q in JSON mode", p.GoVersion.Latest, rvGo1260ND)
		}
	})

	t.Run("tolerates nil GoVersion", func(t *testing.T) {
		t.Parallel()
		p := goutil.Package{Version: &goutil.Version{Current: rvV100, Latest: rvV100}}
		hideIgnoredGoDelta(&p, true, false) // must not panic
	})
}
