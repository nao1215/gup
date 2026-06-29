package pkgselect

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

const (
	goosWindows = "windows"

	pkg1        = "pkg1"
	pkg2        = "pkg2"
	pkg3        = "pkg3"
	nameTest2   = "test2"
	nameMissing = "missing"
)

// excludeNotice builds the notice Exclude emits for a dropped binary, matching
// the message asserted in the table below without repeating the literal.
func excludeNotice(name string) string {
	return "Exclude '" + name + "' from the update target"
}

func TestFilterBinaryPaths(t *testing.T) {
	t.Parallel()

	binList := []string{
		filepath.Join("tmp", "gopls"),
		filepath.Join("tmp", "dlv"),
		filepath.Join("tmp", "air"),
	}

	t.Run("matches a trimmed target and ignores unknown ones", func(t *testing.T) {
		t.Parallel()
		got := FilterBinaryPaths(binList, []string{"  dlv  ", nameMissing})
		want := []string{filepath.Join("tmp", "dlv")}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns the whole list when no targets are given", func(t *testing.T) {
		t.Parallel()
		got := FilterBinaryPaths(binList, nil)
		if diff := cmp.Diff(binList, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns an empty list when every target is blank", func(t *testing.T) {
		t.Parallel()
		got := FilterBinaryPaths(binList, []string{"   ", ""})
		if diff := cmp.Diff([]string{}, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestFilterBinaryPaths_windowsExe(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != goosWindows {
		t.Skip("windows only")
	}

	binList := []string{
		filepath.Join("tmp", "gopls.exe"),
		filepath.Join("tmp", "dlv.exe"),
	}
	got := FilterBinaryPaths(binList, []string{"GOPLS"})
	want := []string{filepath.Join("tmp", "gopls.exe")}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("value is mismatch (-want +got):\n%s", diff)
	}
}

func TestMissingTargets(t *testing.T) {
	t.Parallel()

	binList := []string{
		filepath.Join("bin", "test1"),
		filepath.Join("bin", nameTest2),
		filepath.Join("bin", "test3"),
	}

	t.Run("returns nothing when every target matches a binary", func(t *testing.T) {
		t.Parallel()
		if got := MissingTargets(binList, []string{nameTest2, "test1"}); len(got) != 0 {
			t.Fatalf("MissingTargets() = %v, want none", got)
		}
	})

	t.Run("returns targets that match no binary, in order", func(t *testing.T) {
		t.Parallel()
		got := MissingTargets(binList, []string{"test4", nameTest2, "test5"})
		want := []string{"test4", "test5"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns nothing when no targets are given", func(t *testing.T) {
		t.Parallel()
		if got := MissingTargets(binList, nil); got != nil {
			t.Fatalf("MissingTargets() = %v, want nil", got)
		}
	})

	// A binary present in $GOBIN but absent from the resolved packages (e.g. its
	// build info can't be read, or it was not installed by 'go install') must NOT
	// be reported as missing: it is present, just unmanageable. MissingTargets
	// keys off the binary paths, so naming such a binary yields no "not found"
	// entry even though GetPackageInformation would drop it.
	t.Run("present-but-unmanageable binary is not reported missing", func(t *testing.T) {
		t.Parallel()
		bins := []string{filepath.Join("bin", "broken")}
		if got := MissingTargets(bins, []string{"broken"}); len(got) != 0 {
			t.Fatalf("MissingTargets() = %v, want none (binary exists on disk)", got)
		}
	})
}

func TestMissingTargets_dedupAndTrim(t *testing.T) {
	t.Parallel()

	binList := []string{filepath.Join("bin", "present")}
	got := MissingTargets(binList, []string{" " + nameMissing, nameMissing})
	want := []string{nameMissing}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("value is mismatch (-want +got):\n%s", diff)
	}
}

func TestWarnMissing(t *testing.T) {
	t.Parallel()

	var warnings []string
	WarnMissing([]string{nameMissing}, func(msg string) { warnings = append(warnings, msg) })

	want := []string{"not found '" + nameMissing + "' package in $GOPATH/bin or $GOBIN"}
	if diff := cmp.Diff(want, warnings); diff != "" {
		t.Fatalf("warnings mismatch (-want +got):\n%s", diff)
	}
}

func TestMissingTargets_windowsCaseInsensitive(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != goosWindows {
		t.Skip("windows only")
	}

	binList := []string{
		filepath.Join("bin", "gopls.exe"),
		filepath.Join("bin", "dlv.exe"),
	}
	// "GOPLS" matches "gopls.exe" case-insensitively (so it is not missing),
	// while "MISSING" matches nothing.
	got := MissingTargets(binList, []string{"GOPLS", "MISSING"})
	want := []string{"MISSING"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("value is mismatch (-want +got):\n%s", diff)
	}
}

func TestExclude(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{
		{Name: pkg1},
		{Name: pkg2},
		{Name: pkg3},
	}

	tests := []struct {
		name        string
		excludeList []string
		want        []goutil.Package
		wantNotices []string
	}{
		{
			name:        "drops the named packages",
			excludeList: []string{pkg1, pkg3},
			want:        []goutil.Package{{Name: pkg2}},
			wantNotices: []string{excludeNotice(pkg1), excludeNotice(pkg3)},
		},
		{
			name:        "drops every package",
			excludeList: []string{pkg1, pkg2, pkg3},
			want:        []goutil.Package{},
			wantNotices: []string{excludeNotice(pkg1), excludeNotice(pkg2), excludeNotice(pkg3)},
		},
		{
			name:        "keeps every package when the excluded one is absent",
			excludeList: []string{"pkg4"},
			want:        []goutil.Package{{Name: pkg1}, {Name: pkg2}, {Name: pkg3}},
			wantNotices: nil,
		},
		{
			name:        "trims exclude names before matching",
			excludeList: []string{" pkg1", "pkg3 "},
			want:        []goutil.Package{{Name: pkg2}},
			wantNotices: []string{excludeNotice(pkg1), excludeNotice(pkg3)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var notices []string
			got := Exclude(pkgs, tt.excludeList, func(msg string) {
				notices = append(notices, msg)
			})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("packages mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantNotices, notices); diff != "" {
				t.Errorf("notices mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Exclude must not invoke the notify callback when no package is excluded, so a
// no-op (e.g. JSON mode) caller never has to special-case it.
func TestExclude_silentWhenNothingExcluded(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{{Name: pkg1}}
	called := false
	Exclude(pkgs, nil, func(string) { called = true })
	if called {
		t.Fatal("notify should not be called when nothing is excluded")
	}
}

func TestBinaryPaths(t *testing.T) {
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)

	for _, name := range []string{"gopls", "dlv"} {
		if err := os.WriteFile(filepath.Join(gobin, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := BinaryPaths()
	if err != nil {
		t.Fatalf("BinaryPaths() error = %v", err)
	}
	want := []string{
		filepath.Join(gobin, "dlv"),
		filepath.Join(gobin, "gopls"),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("value is mismatch (-want +got):\n%s", diff)
	}
}

// A first-run environment has no $GOBIN directory yet. Treating that as a
// read-dir error would make list/check/update/export fail instead of behaving
// like a normal empty installed-tool set, so BinaryPaths must return an empty
// list and no error when the directory does not exist.
func TestBinaryPaths_missingGOBINIsEmpty(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("GOBIN", missingDir)

	got, err := BinaryPaths()
	if err != nil {
		t.Fatalf("BinaryPaths() with missing $GOBIN should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("BinaryPaths() with missing $GOBIN should be empty, got: %v", got)
	}
}

func TestPackageInfoByTargets_filtersToTarget(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("..", "..", "cmd", "testdata", "check_success"))

	pkgs, missing, _, err := PackageInfoByTargets(print.New(io.Discard, io.Discard), []string{"gal"})
	if err != nil {
		t.Fatalf("PackageInfoByTargets() error = %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("PackageInfoByTargets() returned %d packages, want 1: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].Name != "gal" {
		t.Fatalf("PackageInfoByTargets() name = %q, want gal", pkgs[0].Name)
	}
	if len(missing) != 0 {
		t.Fatalf("PackageInfoByTargets() missing = %v, want none", missing)
	}
}

// A target that names a binary present on disk but whose build info can't be
// read must not be reported as missing (it exists, just can't be managed). This
// is the regression for the "not found" mislabeling: check_fail/dummy is a
// non-Go file, so GetPackageInformation drops it, yet it is present.
func TestPackageInfoByTargets_presentButUnreadableIsNotMissing(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("..", "..", "cmd", "testdata", "check_fail"))

	pkgs, missing, _, err := PackageInfoByTargets(print.New(io.Discard, io.Discard), []string{"dummy"})
	if err != nil {
		t.Fatalf("PackageInfoByTargets() error = %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("PackageInfoByTargets() returned %d packages, want 0: %+v", len(pkgs), pkgs)
	}
	if len(missing) != 0 {
		t.Fatalf("PackageInfoByTargets() missing = %v, want none (dummy exists on disk)", missing)
	}
}
