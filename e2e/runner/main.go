// Command runner bootstraps gup's offline end-to-end suite and hands the specs
// to atago.
//
// It builds gup and the test module proxy, starts that proxy, warms a shared
// module cache, points every toolchain variable at a throwaway temp tree, and
// runs the atago specs classified for this operating system. The developer's
// real $HOME, ~/.config/gup and $GOBIN are never touched, and no network access
// is required.
//
// The test DEFINITIONS are the atago YAML under e2e/atago; this program is only
// the environment bootstrap. It is Go rather than the shell script it replaces
// because the suite now runs on Windows too, and a bash bootstrap would make the
// Windows leg depend on Git Bash being installed -- a test that passes because of
// the runner image rather than because of gup. Everything here (process
// launching, temp trees, path handling, the read-only module cache cleanup) is
// what the standard library does portably anyway.
//
// Usage:
//
//	go run ./e2e/runner                 # every spec classified for this OS
//	go run ./e2e/runner --filter update # extra flags are passed through to atago
//	COVER=1 go run ./e2e/runner         # build a coverage-instrumented gup
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nao1215/gup/e2e"
)

// warmModules are the fixture modules the offline proxy serves. Installing them
// once into a shared module cache keeps the measured scenarios free of
// "downloading ..." progress on stderr, which strict stderr assertions would
// otherwise see. The deliberately broken fixtures are warmed too (their failures
// ignored) so only their zips end up cached.
var warmModules = []string{
	"gup.test/uptodate@v1.0.0",
	"gup.test/outdated@v1.0.0",
	"gup.test/outdated@v1.1.0",
	"gup.test/pinnable@v1.0.0",
	"gup.test/pinnable@v1.1.0",
	"gup.test/maintool@main",
	"gup.test/mastertool@master",
	"gup.test/badmaintool@v1.0.0",
	"gup.test/badmaintool@main",
	"gup.test/badmaintool@master",
	"gup.test/moved/cmd/tool@v1.0.0",
	"gup.test/moved@v1.1.0",
	"gup.test/replaced@v1.0.0",
	"gup.test/replaced@v1.1.0",
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		var exit *exitError
		if errors.As(err, &exit) {
			os.Exit(exit.code)
		}
		os.Exit(1)
	}
}

// exitError carries an exit status out of run, so a failing atago run exits with
// atago's own status instead of a generic 1.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func run(args []string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(repoRoot, "e2e")

	if _, err := exec.LookPath("atago"); err != nil {
		return fmt.Errorf("atago is not installed. Install it from https://github.com/nao1215/atago\n"+
			"e2e: e.g. 'go install github.com/nao1215/atago@latest' (CI uses nao1215/setup-atago): %w", err)
	}

	// Anything the caller passed that is not a flag is an explicit target, so
	// `go run ./e2e/runner e2e/atago/pin.atago.yaml` still works; otherwise the
	// classification table decides what runs here.
	atagoArgs := append([]string{}, args...)
	if !hasTarget(args) {
		targets, err := e2e.TargetsFor(e2eDir, runtime.GOOS)
		if err != nil {
			return err
		}
		atagoArgs = append(atagoArgs, targets...)
		fmt.Printf("e2e: %s: running %d of %d spec files (see e2e/%s)\n",
			runtime.GOOS, len(targets), countSpecs(e2eDir), e2e.MatrixFileName)
	}

	tmp, err := os.MkdirTemp("", "gup-e2e-")
	if err != nil {
		return fmt.Errorf("can not create the e2e temp tree: %w", err)
	}
	defer func() { _ = removeAllForce(tmp) }()

	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("can not create the e2e bin directory: %w", err)
	}

	if err := buildBinaries(repoRoot, binDir); err != nil {
		return err
	}

	proxyURL, stopProxy, err := startProxy(binDir, tmp)
	if err != nil {
		return err
	}
	defer stopProxy()

	// Shared, offline toolchain settings inherited by every scenario. Per-scenario
	// HOME/GOBIN/GOPATH isolation comes from each spec's `env:` + ${workdir}.
	toolchainEnv := map[string]string{
		"GOPROXY":     proxyURL,
		"GOSUMDB":     "off",
		"GOFLAGS":     "-mod=mod",
		"GOTOOLCHAIN": "local",
		"GOMODCACHE":  filepath.Join(tmp, "gomodcache"),
		"GOCACHE":     filepath.Join(tmp, "gocache"),
	}
	for k, v := range toolchainEnv {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("can not set %s: %w", k, err)
		}
	}

	fmt.Println("e2e: warming module cache...")
	warmCache(tmp)

	// Put the e2e-built gup first on PATH so the specs exercise that exact binary.
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("can not extend PATH: %w", err)
	}

	fmt.Printf("e2e: GOPROXY=%s\n", proxyURL)
	atago := exec.Command("atago", append([]string{"run"}, atagoArgs...)...) //nolint:gosec // a fixed command with author-supplied spec targets
	atago.Dir = repoRoot
	atago.Stdout = os.Stdout
	atago.Stderr = os.Stderr
	atago.Stdin = os.Stdin
	if err := atago.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return &exitError{code: exit.ExitCode(), err: fmt.Errorf("atago run failed: %w", err)}
		}
		return fmt.Errorf("can not run atago: %w", err)
	}
	return nil
}

// hasTarget reports whether the caller named spec files or directories of their
// own, as opposed to passing only atago flags.
//
// A target is recognized by being a path that exists, not by "does not start
// with a dash". atago's flags take their values as separate arguments
// (`--filter update`), so the simpler rule read `update` as a spec path, left
// the classified targets off the command line, and made atago fall back to
// searching the whole repository -- which on Windows would have run every spec
// including the ones e2e/os_matrix.tsv excludes, silently defeating the
// classification the Windows leg depends on.
func hasTarget(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if _, err := os.Stat(arg); err == nil {
			return true
		}
	}
	return false
}

// countSpecs reports how many spec files exist, for the "running N of M" line
// that makes a reduced OS leg visible in the log rather than silent.
func countSpecs(e2eDir string) int {
	specs, err := e2e.SpecFiles(e2eDir)
	if err != nil {
		return 0
	}
	return len(specs)
}

// buildBinaries compiles gup and the offline module proxy into binDir.
//
// COVER=1 builds a coverage-instrumented gup so `make coverage` can collect the
// real CLI's runtime coverage: atago passes the environment (including
// GOCOVERDIR, exported by the caller) through to every spec command, so each gup
// child process writes its own covdata on exit. The default path (COVER unset)
// stays identical, keeping `make e2e` fast.
func buildBinaries(repoRoot, binDir string) error {
	gupArgs := []string{"build"}
	if os.Getenv("COVER") != "" {
		fmt.Println("e2e: COVER=1 -> building coverage-instrumented gup")
		gupArgs = append(gupArgs, "-cover", "-covermode=atomic", "-coverpkg=./...")
	}
	gupArgs = append(gupArgs,
		"-ldflags", "-X github.com/nao1215/gup/internal/cmdinfo.Version=v0.0.0-e2e",
		"-o", filepath.Join(binDir, exeName("gup")), ".")

	fmt.Println("e2e: building gup and the test proxy...")
	if err := goRun(repoRoot, gupArgs...); err != nil {
		return fmt.Errorf("can not build gup: %w", err)
	}
	if err := goRun(repoRoot, "build", "-o", filepath.Join(binDir, exeName("testproxy")), "./e2e/testproxy"); err != nil {
		return fmt.Errorf("can not build the test proxy: %w", err)
	}
	return nil
}

// startProxy launches the offline module proxy and waits for it to report its
// URL, returning that URL and a stop function.
func startProxy(binDir, tmp string) (string, func(), error) {
	urlFile := filepath.Join(tmp, "proxy.url")
	fmt.Println("e2e: starting offline module proxy...")
	proxy := exec.Command(filepath.Join(binDir, exeName("testproxy")), //nolint:gosec // a binary this program just built
		"-dir", filepath.Join(tmp, "proxy"),
		"-url-file", urlFile,
		"-addr", "127.0.0.1:0")
	proxy.Stdout = os.Stdout
	proxy.Stderr = os.Stderr
	if err := proxy.Start(); err != nil {
		return "", nil, fmt.Errorf("can not start the test proxy: %w", err)
	}
	stop := func() {
		_ = proxy.Process.Kill()
		_, _ = proxy.Process.Wait()
	}

	// The proxy binds an ephemeral port and writes the resulting URL, so there is
	// nothing to poll but the file.
	for range 100 {
		if raw, err := os.ReadFile(filepath.Clean(urlFile)); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
			return strings.TrimSpace(string(raw)), stop, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	return "", nil, errors.New("the test proxy did not report its URL in time")
}

// warmCache pre-installs the fixture modules into the shared module cache.
// Failures are ignored on purpose: the deliberately broken fixtures are expected
// to fail here, and warming is an optimization, not a precondition.
func warmCache(tmp string) {
	warmHome := filepath.Join(tmp, "warm")
	gobin := filepath.Join(warmHome, "gobin")
	if err := os.MkdirAll(gobin, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: could not create the warm-up GOBIN: %v\n", err)
		return
	}
	for _, module := range warmModules {
		cmd := exec.Command("go", "install", module) //nolint:gosec // module names are this file's own constants
		cmd.Env = append(os.Environ(),
			"HOME="+warmHome,
			"USERPROFILE="+warmHome,
			"GOBIN="+gobin,
			"GOPATH="+filepath.Join(warmHome, "gopath"),
		)
		_ = cmd.Run()
	}
}

// goRun runs the go tool in dir with the process environment.
func goRun(dir string, args ...string) error {
	cmd := exec.Command("go", args...) //nolint:gosec // args are built by this file
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// exeName appends the platform's executable suffix.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// findRepoRoot walks up from the working directory to the module root, so the
// runner works from anywhere in the tree.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("can not determine the working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find the gup repository root (no go.mod above the working directory)")
		}
		dir = parent
	}
}

// removeAllForce deletes the temp tree, first clearing the read-only bits the Go
// module cache sets. os.RemoveAll cannot delete a read-only file on Windows, and
// a leaked module cache per run adds up fast.
func removeAllForce(root string) error {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a path that cannot be walked is one RemoveAll will report
		}
		mode := fs.FileMode(0o600)
		if d.IsDir() {
			mode = 0o700
		}
		_ = os.Chmod(path, mode)
		return nil
	})
	return os.RemoveAll(root)
}
