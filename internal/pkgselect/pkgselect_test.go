package pkgselect

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
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

func TestExtractByTargets(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{
		{Name: "test1"},
		{Name: nameTest2},
		{Name: "test3"},
	}

	t.Run("returns the matching package", func(t *testing.T) {
		t.Parallel()
		got := ExtractByTargets(pkgs, []string{nameTest2}, func(string) {})
		want := []goutil.Package{{Name: nameTest2}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns an empty slice when nothing matches", func(t *testing.T) {
		t.Parallel()
		got := ExtractByTargets(pkgs, []string{"test4"}, func(string) {})
		if diff := cmp.Diff([]goutil.Package{}, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns the input unchanged when no targets are given", func(t *testing.T) {
		t.Parallel()
		got := ExtractByTargets(pkgs, nil, func(string) {})
		if diff := cmp.Diff(pkgs, got); diff != "" {
			t.Fatalf("value is mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestExtractByTargets_warnsOncePerMissingTarget(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{{Name: "present"}}

	var warnings []string
	got := ExtractByTargets(pkgs, []string{nameMissing, nameMissing}, func(msg string) {
		warnings = append(warnings, msg)
	})
	if len(got) != 0 {
		t.Fatalf("ExtractByTargets() should return no packages, got: %+v", got)
	}

	want := []string{"not found '" + nameMissing + "' package in $GOPATH/bin or $GOBIN"}
	if diff := cmp.Diff(want, warnings); diff != "" {
		t.Fatalf("warnings mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractByTargets_windowsCaseInsensitive(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != goosWindows {
		t.Skip("windows only")
	}

	pkgs := []goutil.Package{
		{Name: "gopls.exe"},
		{Name: "dlv.exe"},
	}
	got := ExtractByTargets(pkgs, []string{"GOPLS"}, func(string) {})
	want := []goutil.Package{{Name: "gopls.exe"}}
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

func TestPackageInfoByTargets_filtersToTarget(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("..", "..", "cmd", "testdata", "check_success"))

	pkgs, _, err := PackageInfoByTargets([]string{"gal"})
	if err != nil {
		t.Fatalf("PackageInfoByTargets() error = %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("PackageInfoByTargets() returned %d packages, want 1: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].Name != "gal" {
		t.Fatalf("PackageInfoByTargets() name = %q, want gal", pkgs[0].Name)
	}
}
