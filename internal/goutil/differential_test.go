package goutil

import (
	"bufio"
	"context"
	"fmt"
	goversion "go/version"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/print"
	"pgregory.net/rapid"
)

// goVersionM is what `go version -m` reports for one file: the toolchain that
// built it, the main package's import path and the main module with its
// version. ok is false when the go command could not read Go build information
// from the file.
type goVersionM struct {
	ok         bool
	goVersion  string
	importPath string
	modulePath string
	version    string
}

// readGoVersionM runs `go version -m` on bin and parses the header line and the
// path and mod lines of its output. It is the reference implementation the
// differential tests compare gup's own debug/buildinfo reading against.
func readGoVersionM(t *testing.T, goCmd, bin string) goVersionM {
	t.Helper()
	out, err := exec.CommandContext(context.Background(), goCmd, "version", "-m", bin).Output() //nolint:gosec // test-controlled paths
	if err != nil {
		// The go command exits non-zero for a file without Go build info.
		return goVersionM{}
	}
	var got goVersionM
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "\t") {
			_, header, found := strings.Cut(line, ": ")
			if !found {
				t.Fatalf("unexpected `go version -m` header for %s: %q", bin, line)
			}
			got.ok = true
			got.goVersion, _, _ = strings.Cut(header, " ")
			continue
		}
		fields := strings.Split(strings.TrimPrefix(line, "\t"), "\t")
		switch fields[0] {
		case "path":
			got.importPath = fields[1]
		case "mod":
			got.modulePath = fields[1]
			if len(fields) > 2 {
				got.version = fields[2]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

// wantPackage is the Package gup must report for a binary, derived from the
// `go version -m` output alone: a binary is managed when it records a main
// module and is not a standard library command (cmd/...).
func wantPackage(bin string, ref goVersionM) (Package, bool) {
	if !ref.ok || ref.modulePath == "" || ref.importPath == "cmd" || strings.HasPrefix(ref.importPath, "cmd/") {
		return Package{}, false
	}
	pkg := Package{
		Name:       filepath.Base(bin),
		ImportPath: ref.importPath,
		ModulePath: ref.modulePath,
		Version:    NewVersion(),
		GoVersion:  NewVersion(),
	}
	pkg.Version.Current = ref.version
	pkg.GoVersion.Current = ref.goVersion
	pkg.GoVersion.Latest = "go-latest"
	return pkg, true
}

// differentialBinaries lists the files the differential test reads: the
// committed ELF and PE fixtures, files that are not Go binaries, the Go
// toolchain's own commands, and freshly built binaries for every release GOOS,
// so Mach-O is covered as well.
func differentialBinaries(t *testing.T, goCmd string) []string {
	t.Helper()
	fixtures := filepath.Join("..", "cmd", "testdata")
	bins := []string{
		filepath.Join(fixtures, "check_success", "gal"),
		filepath.Join(fixtures, "check_success", "posixer"),
		filepath.Join(fixtures, "check_success", "subaru"),
		filepath.Join(fixtures, "check_success_for_windows", "gal.exe"),
		filepath.Join(fixtures, "check_success_for_windows", "posixer.exe"),
		filepath.Join(fixtures, "check_fail", "dummy"),
		filepath.Join(fixtures, "text", "dummy.txt"),
		filepath.Join(fixtures, "gup_config", "nix.json"),
	}

	goroot, err := exec.CommandContext(context.Background(), goCmd, "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	// The toolchain commands share the suffix of the go command LookPath found
	// (".exe" on Windows).
	exe := filepath.Ext(goCmd)
	for _, name := range []string{"go", "gofmt"} {
		bins = append(bins, filepath.Join(strings.TrimSpace(string(goroot)), "bin", name+exe))
	}

	if testing.Short() {
		return bins
	}
	// A module outside GOPATH built with `go build` records its main module
	// with the version "(devel)" or a VCS-less pseudo version, which gup treats
	// as a managed binary like any other.
	mod := t.TempDir()
	writeFile(t, filepath.Join(mod, "go.mod"), "module example.com/difftool\n\ngo 1.22\n")
	writeFile(t, filepath.Join(mod, "cmd", "difftool", "main.go"), "package main\n\nfunc main() {}\n")
	out := t.TempDir()
	// The GOOS values .goreleaser.yml builds: ELF, Mach-O and PE.
	for _, goos := range strings.Fields("linux darwin windows") {
		bin := filepath.Join(out, "difftool-"+goos)
		build := exec.CommandContext(context.Background(), goCmd, "build", "-trimpath", "-o", bin, "./cmd/difftool") //nolint:gosec // fixed arguments
		build.Dir = mod
		build.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0", "GOFLAGS=-mod=mod", "GOWORK=off")
		if msg, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", goos, err, msg)
		}
		bins = append(bins, bin)
	}
	return bins
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCollectPackageInformation_agreesWithGoVersionM compares what gup reads
// from a binary's embedded build information with what `go version -m`
// reports for the same file. gup decides from that information whether a
// binary is managed, which module `gup update` reinstalls and which version and
// Go toolchain `gup check` compares, so a disagreement with the go command is a
// bug in gup: a tool missing from `gup list`, a toolchain command reinstalled,
// or a version compared against the wrong value.
func TestCollectPackageInformation_agreesWithGoVersionM(t *testing.T) {
	t.Parallel()

	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on PATH")
	}

	for _, bin := range differentialBinaries(t, goCmd) {
		t.Run(filepath.Base(bin), func(t *testing.T) {
			t.Parallel()

			ref := readGoVersionM(t, goCmd, bin)
			want, wantManaged := wantPackage(bin, ref)

			got := collectPackageInformation(print.New(io.Discard, io.Discard), []string{bin}, "go-latest")
			switch {
			case !wantManaged && len(got) != 0:
				t.Fatalf("gup manages %s, but `go version -m` reports %+v", bin, ref)
			case wantManaged && len(got) != 1:
				t.Fatalf("gup skips %s, but `go version -m` reports %+v", bin, ref)
			case wantManaged:
				if diff := cmp.Diff(want, got[0]); diff != "" {
					t.Errorf("gup and `go version -m` disagree on %s (-go version -m +gup):\n%s", bin, diff)
				}
			}
		})
	}
}

// goReleaseName draws the name of a Go toolchain release as a binary records
// it in its build information: go1.N, go1.N.P, go1.NrcK or go1.NbetaK before Go
// 1.21, and go1.N.P (P >= 0) or go1.NrcK from Go 1.21 on. Go 1.21 renamed the
// first release of each minor from go1.N to go1.N.0, so the bare go1.N only
// names a language version from then on and no binary records it.
func goReleaseName(t *rapid.T, label string) string {
	minor := rapid.IntRange(0, 40).Draw(t, label+" minor")
	switch rapid.IntRange(0, 3).Draw(t, label+" form") {
	case 0:
		return fmt.Sprintf("go1.%drc%d", minor, rapid.IntRange(1, 3).Draw(t, label+" rc"))
	case 1:
		if minor < 21 {
			return fmt.Sprintf("go1.%dbeta%d", minor, rapid.IntRange(1, 2).Draw(t, label+" beta"))
		}
		return fmt.Sprintf("go1.%d.0", minor)
	case 2:
		if minor < 21 {
			return fmt.Sprintf("go1.%d", minor)
		}
		return fmt.Sprintf("go1.%d.0", minor)
	default:
		return fmt.Sprintf("go1.%d.%d", minor, rapid.IntRange(1, 12).Draw(t, label+" patch"))
	}
}

// TestGoVersionUpToDate_agreesWithGoVersionCompare compares gup's Go toolchain
// comparison, which decides whether `gup check` reports a binary as built with
// an old Go and whether `gup update` rebuilds it, with go/version.Compare, the
// standard library's implementation of the go command's own ordering. Release
// candidates and betas sort before the release they precede, and a toolchain
// suffix such as " X:nodwarf5" does not change the order.
func TestGoVersionUpToDate_agreesWithGoVersionCompare(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		current := goReleaseName(t, "current")
		available := goReleaseName(t, "available")
		want := goversion.Compare(current, available) >= 0

		suffix := rapid.SampledFrom([]string{"", " X:nodwarf5", "-X:loopvar"}).Draw(t, "toolchain suffix")
		if got := goVersionUpToDate(current+suffix, available); got != want {
			t.Fatalf("goVersionUpToDate(%q, %q) = %v, go/version.Compare says %v", current+suffix, available, got, want)
		}
	})
}
