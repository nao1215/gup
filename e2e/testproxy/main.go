// Command testproxy generates a self-contained Go module proxy tree from
// in-repo fixtures and serves it over 127.0.0.1, so the end-to-end tests can
// run real "go install" commands fully offline.
//
// It emulates a real GOPROXY closely enough for gup's behavior under test:
//   - version downloads (.info/.mod/.zip and @v/list, @latest),
//   - branch resolution via @v/<branch>.info (e.g. @main, @master),
//   - a missing branch returns HTTP 404 with an "unknown revision <ref>" body,
//     matching what proxy.golang.org reports, so gup's @main -> @master
//     fallback (which only triggers on branch-not-found) can be exercised.
//
// Usage:
//
//	testproxy -dir <proxyDir> -url-file <path> [-addr 127.0.0.1:0]
//
// It writes the chosen base URL (e.g. http://127.0.0.1:54321) to -url-file and
// then serves until the process is killed.
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// modVersion describes one module version served by the proxy.
type modVersion struct {
	module  string // module path, e.g. "gup.test/outdated"
	version string // semantic or pseudo version
	mainGo  string // contents of main.go for this version
	// pkgSubdir places main.go in a subdirectory of the module (e.g. "cmd/tool"),
	// so the installable import path becomes module+"/"+pkgSubdir. Empty means the
	// module root. A newer version that omits a previously present subdir lets the
	// proxy reproduce the real "found (vX), but does not contain package ..."
	// failure a tool hits after it moves its command (e.g. on a /v2 bump).
	pkgSubdir string
	// replace, when true, adds a replace directive to this version's go.mod, so
	// "go install" of this version fails with the real "go.mod file ... contains
	// one or more replace directives" error.
	replace bool
}

// branchRef maps a VCS ref (branch name) to a concrete version the proxy
// resolves it to. A module with a ref absent from this list reports
// "unknown revision <ref>" for that ref.
type branchRef struct {
	module  string
	ref     string
	version string
}

// fixtureTime is a fixed timestamp; tests must be deterministic and the runtime
// forbids reading the wall clock here anyway.
const fixtureTime = "2024-01-01T00:00:00Z"

func goMod(module string) string {
	return "module " + module + "\n\ngo 1.21\n"
}

// goModWithReplace returns a go.mod carrying a replace directive, which makes
// "go install" of the module reject it with the real "contains one or more
// replace directives" error.
func goModWithReplace(module string) string {
	return goMod(module) + "\nreplace gup.test/replaced/dep => gup.test/replaced/dep v1.0.0\n"
}

// moduleGoMod renders the go.mod for one version, with or without a replace
// directive.
func moduleGoMod(v modVersion) string {
	if v.replace {
		return goModWithReplace(v.module)
	}
	return goMod(v.module)
}

func okMain(msg string) string {
	return "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"" + msg + "\") }\n"
}

// badMain does not compile, so "go install" fails with a build error rather than
// a branch-not-found error.
const badMain = "package main\n\nfunc main() { thisSymbolDoesNotExist() }\n"

// fixtures returns the module versions and branch refs the proxy serves.
//
// Pseudo-versions use the standard v0.0.0-<utc>-<12 hex> form. Real semantic
// versions stay within v0/v1 because a v2+ module path would need a /v2 suffix.
func fixtures() ([]modVersion, []branchRef, map[string][]string, map[string]string) {
	versions := []modVersion{
		// uptodate: a single version that is also @latest.
		{module: "gup.test/uptodate", version: "v1.0.0", mainGo: okMain("uptodate v1.0.0")},
		// outdated: installed at v1.0.0, newer v1.1.0 available as @latest.
		{module: "gup.test/outdated", version: "v1.0.0", mainGo: okMain("outdated v1.0.0")},
		{module: "gup.test/outdated", version: "v1.1.0", mainGo: okMain("outdated v1.1.0")},
		// maintool: tracked on @main (resolves to a pseudo-version).
		{module: "gup.test/maintool", version: "v0.0.0-20240101000000-00000000000a", mainGo: okMain("maintool main")},
		// mastertool: has only a master branch (no main).
		{module: "gup.test/mastertool", version: "v0.0.0-20240101000000-00000000000b", mainGo: okMain("mastertool master")},
		// badmaintool: installable at v1.0.0; @main resolves but does NOT compile;
		// @master resolves and DOES compile. This proves gup must not fall back to
		// the working @master when @main fails for a non-branch reason.
		// The @main and @master pseudo-versions sort NEWER than v1.0.0 (they are
		// "+1 commit after v1.0.0"), so gup actually attempts the install instead
		// of treating the installed v1.0.0 as up-to-date.
		{module: "gup.test/badmaintool", version: "v1.0.0", mainGo: okMain("badmaintool v1.0.0")},
		{module: "gup.test/badmaintool", version: "v1.0.1-0.20240102000000-00000000000c", mainGo: badMain},
		{module: "gup.test/badmaintool", version: "v1.0.1-0.20240102000000-00000000000d", mainGo: okMain("badmaintool master")},
		// moved: the command lives under cmd/tool at v1.0.0, but the newer
		// @latest (v1.1.0) no longer contains that package. Installing
		// gup.test/moved/cmd/tool@v1.1.0 therefore fails with the real
		// "found (v1.1.0), but does not contain package ..." error a tool emits
		// after relocating its command (e.g. on a major-version bump). This drives
		// the diagnostics e2e for the next-step hint.
		{module: "gup.test/moved", version: "v1.0.0", mainGo: okMain("moved tool v1.0.0"), pkgSubdir: "cmd/tool"},
		{module: "gup.test/moved", version: "v1.1.0", mainGo: okMain("moved v1.1.0")},
		// replaced: installable at v1.0.0, but the newer @latest (v1.1.0) adds a
		// replace directive to go.mod, so "go install" rejects it with the real
		// "contains one or more replace directives" error. Drives the diagnostics
		// e2e for the replace-directive next-step hint.
		{module: "gup.test/replaced", version: "v1.0.0", mainGo: okMain("replaced v1.0.0")},
		{module: "gup.test/replaced", version: "v1.1.0", mainGo: okMain("replaced v1.1.0"), replace: true},
	}
	branches := []branchRef{
		{"gup.test/maintool", "main", "v0.0.0-20240101000000-00000000000a"},
		{"gup.test/mastertool", "master", "v0.0.0-20240101000000-00000000000b"},
		{"gup.test/badmaintool", "main", "v1.0.1-0.20240102000000-00000000000c"},
		{"gup.test/badmaintool", "master", "v1.0.1-0.20240102000000-00000000000d"},
	}
	// @v/list contents per module. Modules tracked only via a branch still need a
	// (possibly empty) list so the go client's deprecation lookup does not 404.
	lists := map[string][]string{
		"gup.test/uptodate":    {"v1.0.0"},
		"gup.test/outdated":    {"v1.0.0", "v1.1.0"},
		"gup.test/maintool":    {},
		"gup.test/mastertool":  {},
		"gup.test/badmaintool": {"v1.0.0"},
		"gup.test/moved":       {"v1.0.0", "v1.1.0"},
		"gup.test/replaced":    {"v1.0.0", "v1.1.0"},
	}
	// @latest version per module. The go client resolves @latest even for a
	// branch install (deprecation lookup), so branch-only modules point @latest
	// at their branch pseudo-version.
	latest := map[string]string{
		"gup.test/uptodate":    "v1.0.0",
		"gup.test/outdated":    "v1.1.0",
		"gup.test/maintool":    "v0.0.0-20240101000000-00000000000a",
		"gup.test/mastertool":  "v0.0.0-20240101000000-00000000000b",
		"gup.test/badmaintool": "v1.0.0",
		"gup.test/moved":       "v1.1.0",
		"gup.test/replaced":    "v1.1.0",
	}
	return versions, branches, lists, latest
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // test fixture dir
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644) //nolint:gosec // test fixture file
}

func infoJSON(version string) string {
	b, _ := json.Marshal(map[string]string{"Version": version, "Time": fixtureTime})
	return string(b)
}

// writeVersion writes the .info/.mod/.zip for one module version.
func writeVersion(dir string, v modVersion) error {
	vdir := filepath.Join(dir, v.module, "@v")
	if err := writeFile(filepath.Join(vdir, v.version+".info"), infoJSON(v.version)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(vdir, v.version+".mod"), moduleGoMod(v)); err != nil {
		return err
	}
	zipPath := filepath.Join(vdir, v.version+".zip")
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil { //nolint:gosec // test fixture dir
		return err
	}
	zf, err := os.Create(zipPath) //nolint:gosec // test fixture path
	if err != nil {
		return err
	}
	defer func() { _ = zf.Close() }()
	zw := zip.NewWriter(zf)
	prefix := v.module + "@" + v.version + "/"
	mainGoPath := "main.go"
	if v.pkgSubdir != "" {
		mainGoPath = v.pkgSubdir + "/main.go"
	}
	for name, content := range map[string]string{"go.mod": moduleGoMod(v), mainGoPath: v.mainGo} {
		w, werr := zw.Create(prefix + name)
		if werr != nil {
			return werr
		}
		if _, werr := w.Write([]byte(content)); werr != nil {
			return werr
		}
	}
	return zw.Close()
}

// generate writes the whole proxy tree under dir.
func generate(dir string) error {
	versions, branches, lists, latest := fixtures()
	for _, v := range versions {
		if err := writeVersion(dir, v); err != nil {
			return err
		}
	}
	for _, b := range branches {
		if err := writeFile(filepath.Join(dir, b.module, "@v", b.ref+".info"), infoJSON(b.version)); err != nil {
			return err
		}
	}
	for module, vs := range lists {
		content := ""
		if len(vs) > 0 {
			content = strings.Join(vs, "\n") + "\n"
		}
		if err := writeFile(filepath.Join(dir, module, "@v", "list"), content); err != nil {
			return err
		}
	}
	for module, v := range latest {
		if err := writeFile(filepath.Join(dir, module, "@latest"), infoJSON(v)); err != nil {
			return err
		}
	}
	return nil
}

// proxyHandler serves the static tree, emulating a real proxy's branch-not-found
// error for a missing @v/<ref>.info so the go client reports "unknown revision".
func proxyHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		path := filepath.Join(root, clean)
		// Keep the resolved path within root (defense in depth for a test server).
		if !strings.HasPrefix(path, filepath.Clean(root)) {
			http.NotFound(w, r)
			return
		}
		if data, err := os.ReadFile(path); err == nil { //nolint:gosec // path constrained to root
			_, _ = w.Write(data)
			return
		}
		if strings.HasSuffix(clean, ".info") {
			ref := strings.TrimSuffix(filepath.Base(clean), ".info")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, "unknown revision %s", ref)
			return
		}
		http.NotFound(w, r)
	}
}

func main() {
	dir := flag.String("dir", "", "directory to generate the proxy tree into (required)")
	urlFile := flag.String("url-file", "", "file to write the served base URL into (required)")
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	flag.Parse()

	if *dir == "" || *urlFile == "" {
		log.Fatal("testproxy: -dir and -url-file are required")
	}
	if err := generate(*dir); err != nil {
		log.Fatalf("testproxy: generate: %v", err)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("testproxy: listen: %v", err)
	}
	baseURL := "http://" + ln.Addr().String()
	if err := os.WriteFile(*urlFile, []byte(baseURL), 0o644); err != nil { //nolint:gosec // test url file
		log.Fatalf("testproxy: write url file: %v", err)
	}
	log.Printf("testproxy serving %s from %s", baseURL, *dir)
	if err := http.Serve(ln, proxyHandler(*dir)); err != nil { //nolint:gosec // long-lived test server
		log.Fatalf("testproxy: serve: %v", err)
	}
}
