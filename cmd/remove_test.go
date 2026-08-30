//nolint:errcheck,gosec,wastedassign
package cmd

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/quick"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/spf13/cobra"
)

const (
	whitespaceOnly = "   "
	goplsExe       = "gopls.exe"
	dotExe         = ".exe"
	dotUpperExe    = ".EXE"
	abcLiteral     = "abc"
)

func Test_removeLoop(t *testing.T) {
	type args struct {
		gobin  string
		force  bool
		target []string
	}

	type test struct {
		name  string
		args  args
		input string
		want  int
	}

	tests := []test{}
	if runtime.GOOS != goosWindows {
		tests = append(tests, test{
			name: "windows environment and suffix is mismatch",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{testBinPosixer},
			},
			input: "y",
			want:  1,
		})
	}

	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name: "interactive question: input 'y'",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{testBinPosixerExe},
			},
			input: "y",
			want:  0,
		})
		tests = append(tests, test{
			name: testDeleteCancel,
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{testBinPosixerExe},
			},
			input: "n",
			want:  0,
		})
	} else {
		tests = append(tests, test{
			name: "interactive question: input 'y'",
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{testBinPosixer},
			},
			input: "y",
			want:  0,
		})
		tests = append(tests, test{
			name: testDeleteCancel,
			args: args{
				gobin:  filepath.Join("testdata", "delete"),
				force:  false,
				target: []string{testBinPosixer},
			},
			input: "n",
			want:  0,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Join("testdata", "delete"), 0755); err != nil {
				t.Fatal(err)
			}

			src := ""
			dest := ""
			if runtime.GOOS == goosWindows {
				src = filepath.Join("testdata", "check_success_for_windows", testBinPosixerExe)
				dest = filepath.Join("testdata", "delete", testBinPosixerExe)
			} else {
				src = filepath.Join("testdata", "check_success", testBinPosixer)
				dest = filepath.Join("testdata", "delete", testBinPosixer)
			}
			newFile, err := os.Create(dest)
			if err != nil {
				t.Fatal(err)
			}

			oldFile, err := os.Open(src)
			if err != nil {
				t.Fatal(err)
			}

			_, err = io.Copy(newFile, oldFile)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				os.Remove(dest)
			}()
			oldFile.Close()
			newFile.Close()

			funcDefer, err := mockStdin(t, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			defer funcDefer()

			// mockStdin replaces os.Stdin with a regular file, which is not a TTY.
			// Pretend stdin is a terminal so the interactive confirmation path runs.
			origStdinIsTerminal := stdinIsTerminal
			stdinIsTerminal = func() bool { return true }
			defer func() { stdinIsTerminal = origStdinIsTerminal }()

			if runtime.GOOS != goosWindows && tt.name == "windows environment and suffix is mismatch" {
				GOOS = goosWindows
				defer func() { GOOS = runtime.GOOS }()
				t.Setenv("GOEXE", dotExe)
			}

			p, _ := newTestPrinter()
			if got := removeLoop(p, tt.args.gobin, tt.args.force, tt.args.target); got != tt.want {
				t.Errorf("removeLoop() = %v, want %v", got, tt.want)
			}

			if tt.name == testDeleteCancel && !fileutil.IsFile(dest) {
				t.Errorf("input no, however posixer command is deleted")
			}
		})
	}
}

func Test_removeLoop_rejectPathTraversal(t *testing.T) {
	t.Parallel()

	gobin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(gobin, 0o755); err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(filepath.Dir(gobin), "victim")
	if err := os.WriteFile(victim, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{"../victim"}); got != 1 {
		t.Fatalf("removeLoop() = %v, want %v", got, 1)
	}

	if !fileutil.IsFile(victim) {
		t.Fatalf("path traversal should not delete %s", victim)
	}
}

func Test_remove_flagError(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	// missing "force" flag
	p, _ := newTestPrinter()
	got := remove(p, cmd, []string{testBinTool})
	if got != 1 {
		t.Errorf("remove() = %v, want 1", got)
	}
}

func Test_remove_noArgs(t *testing.T) {
	t.Parallel()
	cmd := newRemoveCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("remove without args should fail")
	}
	got := err.Error()
	for _, want := range []string{"requires at least one binary name", "gup remove gopls"} {
		if !strings.Contains(got, want) {
			t.Errorf("error should contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Usage:") {
		t.Errorf("error should be concise, not full help, got:\n%s", got)
	}
}

func Test_removeLoop_forceNonExist(t *testing.T) {
	t.Parallel()
	gobin := t.TempDir()
	p, _ := newTestPrinter()
	got := removeLoop(p, gobin, true, []string{"nonexistent"})
	if got != 1 {
		t.Errorf("removeLoop() = %v, want 1 for non-existent binary", got)
	}
}

// Test_removeLoop_stdinReadFailureIsError reproduces the bug where a failed
// confirmation read (e.g. EOF / a closed stdin pipe) was treated as a user
// "cancel": gup printed "cancel removal" and returned a success exit code, so a
// caller could not tell that the removal had silently not happened. A read
// failure must instead be reported as an error (exit 1) and must not delete the
// target.
//
//nolint:paralleltest // mutates package-level stdinIsTerminal and process os.Stdin
func Test_removeLoop_stdinReadFailureIsError(t *testing.T) {
	gobin := t.TempDir()
	binaryName := testBinPosixer
	if GOOS == goosWindows {
		binaryName += normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
	}
	binaryPath := filepath.Join(gobin, binaryName)
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Pretend stdin is a TTY so the interactive confirmation path runs, then feed
	// it an empty stream so the confirmation read returns EOF.
	origStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = origStdinIsTerminal })

	funcDefer, err := mockStdin(t, "")
	if err != nil {
		t.Fatal(err)
	}
	defer funcDefer()

	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, false, []string{testBinPosixer}); got != 1 {
		t.Fatalf("removeLoop() on stdin read failure = %d, want 1", got)
	}
	if !fileutil.IsFile(binaryPath) {
		t.Fatal("target must not be deleted when the confirmation read fails")
	}
	if strings.Contains(buf.String(), "cancel removal") {
		t.Errorf("must not report 'cancel removal' on a read failure:\n%s", buf.String())
	}
}

func Test_removeLoop_windowsFallbackGoexe(t *testing.T) {
	origGOOS := GOOS
	GOOS = goosWindows
	t.Cleanup(func() { GOOS = origGOOS })
	t.Setenv("GOEXE", "")

	gobin := t.TempDir()
	binaryPath := filepath.Join(gobin, testBinPosixerExe)
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{testBinPosixer}); got != 0 {
		t.Fatalf("removeLoop() = %v, want 0", got)
	}
	if fileutil.IsFile(binaryPath) {
		t.Fatalf("binary should be removed: %s", binaryPath)
	}
}

func Test_removeLoop_windowsSuffixCaseInsensitive(t *testing.T) {
	origGOOS := GOOS
	GOOS = goosWindows
	t.Cleanup(func() { GOOS = origGOOS })
	t.Setenv("GOEXE", dotExe)

	gobin := t.TempDir()
	binaryPath := filepath.Join(gobin, "gopls.EXE")
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{"gopls.EXE"}); got != 0 {
		t.Fatalf("removeLoop() = %v, want 0", got)
	}
	if fileutil.IsFile(binaryPath) {
		t.Fatalf("binary should be removed: %s", binaryPath)
	}
}

func Test_removeLoop_forceTrimmedName(t *testing.T) {
	t.Parallel()

	gobin := t.TempDir()
	binaryName := testBinPosixer
	if GOOS == goosWindows {
		binaryName += normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
	}
	binaryPath := filepath.Join(gobin, binaryName)
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{"  posixer  "}); got != 0 {
		t.Fatalf("removeLoop() = %v, want 0", got)
	}
	if fileutil.IsFile(binaryPath) {
		t.Fatalf("binary should be removed: %s", binaryPath)
	}
}

func Test_isSafeBinaryName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple name", input: testBinMytool, want: true},
		{name: "with extension", input: testBinMytoolExe, want: true},
		{name: "empty", input: "", want: false},
		{name: "whitespace only", input: whitespaceOnly, want: false},
		{name: "leading and trailing whitespace", input: " mytool ", want: false},
		{name: "absolute path", input: "/usr/bin/tool", want: false},
		{name: "forward slash", input: "sub/tool", want: false},
		{name: "backslash", input: `sub\tool`, want: false},
		{name: "contains colon", input: "C:tool", want: false},
		{name: "single dot", input: ".", want: false},
		{name: "double dots", input: "..", want: false},
		{name: "parent traversal", input: "../escape", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isSafeBinaryName(tt.input)
			if got != tt.want {
				t.Errorf("isSafeBinaryName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // mutates package-level stdinIsTerminal
func Test_removeLoop_nonTTYWithoutForceFailsFast(t *testing.T) {
	// Not parallel: this test mutates the package-level stdinIsTerminal,
	// which is also mutated by Test_removeLoop.
	gobin := t.TempDir()
	binaryPath := filepath.Join(gobin, testBinPosixer)
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	target := testBinPosixer
	if GOOS == goosWindows {
		target += normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
		if err := os.Rename(binaryPath, filepath.Join(gobin, target)); err != nil {
			t.Fatal(err)
		}
		binaryPath = filepath.Join(gobin, target)
	}

	origStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = origStdinIsTerminal })

	// Without --force and without a TTY, removeLoop must fail fast (exit 1)
	// and must NOT attempt interactive confirmation nor remove the file.
	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, false, []string{target}); got != 1 {
		t.Fatalf("removeLoop() = %v, want 1 for non-TTY without --force", got)
	}
	if !fileutil.IsFile(binaryPath) {
		t.Fatalf("file must not be removed when confirmation is required but stdin is not a TTY: %s", binaryPath)
	}
}

//nolint:paralleltest // mutates package-level stdinIsTerminal
func Test_removeLoop_nonTTYWithForceStillRemoves(t *testing.T) {
	// Not parallel: this test mutates the package-level stdinIsTerminal,
	// which is also mutated by Test_removeLoop.
	gobin := t.TempDir()
	target := testBinPosixer
	if GOOS == goosWindows {
		target += normalizeExecSuffix(GOOS, os.Getenv("GOEXE"))
	}
	binaryPath := filepath.Join(gobin, target)
	if err := os.WriteFile(binaryPath, []byte("dummy"), 0o700); err != nil {
		t.Fatal(err)
	}

	origStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = origStdinIsTerminal })

	// --force must skip confirmation regardless of TTY state.
	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{target}); got != 0 {
		t.Fatalf("removeLoop() = %v, want 0 for non-TTY with --force", got)
	}
	if fileutil.IsFile(binaryPath) {
		t.Fatalf("file must be removed with --force: %s", binaryPath)
	}
}

// Test_isSafeBinaryName_propertyStaysInGobin is a property-based test asserting
// the security invariant: any name accepted by isSafeBinaryName can only resolve
// to a file located directly inside $GOBIN, i.e.
//
//	isSafeBinaryName(s) == true  =>  filepath.Dir(filepath.Join(gobin, s)) == gobin
//
// This is the last line of defense against path traversal in `gup remove`.
func Test_isSafeBinaryName_propertyStaysInGobin(t *testing.T) {
	t.Parallel()

	gobin := filepath.Clean(t.TempDir())

	invariant := func(s string) bool {
		if !isSafeBinaryName(s) {
			return true // property only constrains accepted names
		}
		return filepath.Dir(filepath.Join(gobin, s)) == gobin
	}

	if err := quick.Check(invariant, &quick.Config{MaxCount: 100000}); err != nil {
		t.Errorf("isSafeBinaryName property violated: %v", err)
	}

	// Adversarial inputs that quick may not easily generate.
	adversarial := []string{
		"",                  // empty
		whitespaceOnly,      // whitespace only
		" tool ",            // surrounding whitespace
		".",                 // current dir
		"..",                // parent dir
		"../escape",         // parent traversal
		"../../escape",      // deeper traversal
		"/abs/path",         // absolute
		`C:\Windows\tool`,   // windows absolute
		"C:tool",            // windows drive-relative (colon)
		"sub/tool",          // forward slash
		`sub\tool`,          // backslash
		"tool\x00.exe",      // embedded NUL
		"tool\nname",        // embedded newline (control char)
		"tool\tname",        // embedded tab (control char)
		"a",                 // exact-length minimal name (len 1)
		"e",                 // len(s) shorter than typical suffix
		"gopls",             // plain valid name
		goplsExe,            // valid name with extension
		"ｇｏｐｌｓ",             // unicode full-width look-alike
		"tool/../escape",    // traversal hidden mid-string
		"./tool",            // dot-slash prefix
		"\u202e" + "exe.sh", // right-to-left override look-alike
	}
	for _, s := range adversarial {
		if !invariant(s) {
			t.Errorf("invariant violated for adversarial input %q: accepted name resolves outside gobin", s)
		}
	}
}

func Test_hasSuffixFold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		s      string
		suffix string
		want   bool
	}{
		{name: "exact match same case", s: dotExe, suffix: dotExe, want: true},
		{name: "case-insensitive match", s: "gopls.EXE", suffix: dotExe, want: true},
		{name: "case-insensitive match reversed", s: goplsExe, suffix: dotUpperExe, want: true},
		{name: "suffix present lowercase", s: "tool.exe", suffix: dotExe, want: true},
		{name: "no suffix match", s: "tool.bin", suffix: dotExe, want: false},
		{name: "s shorter than suffix", s: "ex", suffix: dotExe, want: false},
		{name: "s one char shorter than suffix", s: ".ex", suffix: dotExe, want: false},
		{name: "empty suffix matches anything", s: testBinTool, suffix: "", want: true},
		{name: "empty suffix and empty s", s: "", suffix: "", want: true},
		{name: "empty s nonempty suffix", s: "", suffix: dotExe, want: false},
		{name: "equal length match", s: abcLiteral, suffix: abcLiteral, want: true},
		{name: "equal length mismatch", s: abcLiteral, suffix: "xyz", want: false},
		{name: "suffix in middle only", s: ".exe.bin", suffix: dotExe, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasSuffixFold(tt.s, tt.suffix); got != tt.want {
				t.Errorf("hasSuffixFold(%q, %q) = %v, want %v", tt.s, tt.suffix, got, tt.want)
			}
		})
	}
}

// mockStdin is a helper function that lets the test pretend dummyInput as os.Stdin.
// It will return a function for `defer` to clean up after the test.
func mockStdin(t *testing.T, dummyInput string) (funcDefer func(), err error) {
	t.Helper()

	oldOsStdin := os.Stdin
	var tmpFile *os.File
	var e error
	if runtime.GOOS != goosWindows {
		tmpFile, e = os.CreateTemp(t.TempDir(), strings.ReplaceAll(t.Name(), "/", ""))
	} else {
		// See https://github.com/golang/go/issues/51442
		tmpFile, e = os.CreateTemp(os.TempDir(), strings.ReplaceAll(t.Name(), "/", ""))
	}
	if e != nil {
		return nil, e
	}

	content := []byte(dummyInput)

	if _, err := tmpFile.Write(content); err != nil {
		return nil, err
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, err
	}

	// Set stdin to the temp file
	os.Stdin = tmpFile

	return func() {
		// clean up
		os.Stdin = oldOsStdin
		os.Remove(tmpFile.Name())
	}, nil
}

// TestRemoveLoop_removeFailure covers the branch that reports an os.Remove
// failure: the target exists but its parent directory is read-only, so the
// unlink is denied.
//
//nolint:paralleltest // relies on process-wide filesystem state
func TestRemoveLoop_removeFailure(t *testing.T) {
	skipIfDirWriteFaultUnsupported(t)
	gobin := t.TempDir()
	bin := filepath.Join(gobin, testBinTool)
	if err := os.WriteFile(bin, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(gobin, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gobin, 0o700) })

	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{testBinTool}); got != 1 {
		t.Fatalf("removeLoop() = %d, want 1 on remove failure", got)
	}
	if buf.Len() == 0 {
		t.Error("removeLoop() expected an error message")
	}
}

// Test_removeLoop_refusesTheLockFile is the regression test for `gup remove
// .gup.lock --force`.
//
// The lock guarding $GOBIN lives in $GOBIN, and `gup remove` deletes files from
// $GOBIN by name, so the name had to be reserved rather than merely hidden. The
// consequence of not reserving it is not a lost file: the kernel lock lives on
// an open descriptor and survives the unlink, so the command running the removal
// keeps working, while the NEXT gup finds the name free, creates its own file
// there and locks that instead - two commands rewriting one $GOBIN, each
// certain it has it to itself.
func Test_removeLoop_refusesTheLockFile(t *testing.T) {
	t.Parallel()

	gobin := t.TempDir()
	lock := filepath.Join(gobin, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Spellings that reach the same file: as typed, padded, and (on the
	// case-insensitive filesystems macOS and Windows use) upper-cased.
	for _, name := range []string{lockfile.DirLockName, "  " + lockfile.DirLockName + "  ", ".GUP.LOCK"} {
		p, buf := newTestPrinter()
		if got := removeLoop(p, gobin, true, []string{name}); got != 1 {
			t.Errorf("removeLoop(%q) = %d, want 1", name, got)
		}
		if !fileutil.IsFile(lock) {
			t.Fatalf("removeLoop(%q) deleted gup's own lock file", name)
		}
		if out := buf.String(); !strings.Contains(out, "gup's own lock file") {
			t.Errorf("removeLoop(%q) output %q does not explain the refusal", name, out)
		}
	}
}

// Test_removeLoop_refusesTheLockFileOnWindowsSpellings covers the same refusal
// through the Windows path, where the argument gets $GOEXE appended before it is
// looked up. A check placed only after that step would let `.gup.lock` through
// as `.gup.lock.exe`; one placed only before it would miss a $GOEXE that
// composes the reserved name.
func Test_removeLoop_refusesTheLockFileOnWindowsSpellings(t *testing.T) {
	original := GOOS
	GOOS = goosWindows
	t.Cleanup(func() { GOOS = original })

	gobin := t.TempDir()
	lock := filepath.Join(gobin, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOEXE", dotExe)
	p, _ := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{lockfile.DirLockName}); got != 1 {
		t.Errorf("removeLoop() = %d, want 1", got)
	}

	// A $GOEXE that turns an innocuous argument INTO the reserved name.
	t.Setenv("GOEXE", ".lock")
	p, _ = newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{".gup"}); got != 1 {
		t.Errorf("removeLoop() = %d, want 1", got)
	}
	if !fileutil.IsFile(lock) {
		t.Fatal("removeLoop() deleted gup's own lock file")
	}
}

// Test_removeLoop_stillRemovesOtherDotFiles keeps the refusal narrow. Only the
// one name gup keeps for itself is reserved; a tool that happens to start with a
// dot is still the user's to remove.
func Test_removeLoop_stillRemovesOtherDotFiles(t *testing.T) {
	// Pinned so the file this plants carries the same suffix removeLoop will look
	// for: on Windows an argument without one gets $GOEXE appended.
	t.Setenv("GOEXE", "")
	gobin := t.TempDir()
	const arg = ".gup.lock.bak"
	onDisk := arg
	if GOOS == goosWindows {
		onDisk += exeSuffix
	}
	other := filepath.Join(gobin, onDisk)
	if err := os.WriteFile(other, []byte("not the lock"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{arg}); got != 0 {
		t.Errorf("removeLoop() = %d, want 0; output = %q", got, buf.String())
	}
	if fileutil.IsFile(other) {
		t.Error("removeLoop() refused a file that is not gup's lock")
	}
}

// Test_remove_refusesTheLockFileThroughTheRealCLI drives the whole command -
// argument parsing, the state lock the command itself takes, and the removal
// loop - because the bug was only reachable that way: the lock `gup remove`
// holds while it runs is the very file the argument names.
func Test_remove_refusesTheLockFileThroughTheRealCLI(t *testing.T) {
	setupXDGBase(t)
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)
	t.Setenv(gupLockWaitEnv, "200ms")

	code := 0
	originalExit := OsExit
	OsExit = func(c int) { code = c }
	t.Cleanup(func() { OsExit = originalExit })

	out, err := runRootWithBuffer([]string{testCmdGup, "remove", "--force", lockfile.DirLockName})
	if err != nil {
		t.Fatalf("gup remove returned %v, want the command's own status", err)
	}
	if code != 1 {
		t.Errorf("gup remove --force %s exited %d, want 1", lockfile.DirLockName, code)
	}
	if !strings.Contains(out, "gup's own lock file") {
		t.Errorf("gup remove output %q does not explain the refusal", out)
	}
	// The command took that lock while it ran, so the file is there - and still
	// there, which is the whole point.
	if !fileutil.IsFile(filepath.Join(gobin, lockfile.DirLockName)) {
		t.Error("gup remove deleted the lock file it was holding")
	}
}

// gupLockWaitEnv shortens the lock's acquisition timeout so a test that is not
// about waiting does not wait.
const gupLockWaitEnv = "GUP_LOCK_WAIT"

// Test_removeLoop_refusesAnotherNameForTheLockFile covers the half of the
// refusal that no amount of string folding reaches.
//
// The name checks in removeLoop compare what the user typed against the one name
// gup keeps for itself. A file has more names than that: a hard link is a second
// name by construction, Windows answers to an 8.3 alias like GUPLOC~1.LOC, and
// Win32 strips trailing dots and spaces before the filesystem sees the name at
// all. So the last question asked before the delete is not "is this name
// reserved" but "is this file the lock file", which the filesystem answers.
func Test_removeLoop_refusesAnotherNameForTheLockFile(t *testing.T) {
	// Pinned so the file this plants carries the suffix removeLoop looks for.
	t.Setenv("GOEXE", "")
	gobin := t.TempDir()
	lock := filepath.Join(gobin, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	const arg = "innocent"
	onDisk := arg
	if GOOS == goosWindows {
		onDisk += exeSuffix
	}
	alias := filepath.Join(gobin, onDisk)
	if err := os.Link(lock, alias); err != nil {
		t.Skipf("this filesystem does not support hard links: %v", err)
	}

	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{arg}); got != 1 {
		t.Errorf("removeLoop(%q) = %d, want 1", arg, got)
	}
	if !fileutil.IsFile(lock) {
		t.Fatal("removeLoop() deleted gup's own lock file through a second name for it")
	}
	if !fileutil.IsFile(alias) {
		t.Error("removeLoop() unlinked a second name for the lock file")
	}
	if out := buf.String(); !strings.Contains(out, "gup's own lock file") {
		t.Errorf("removeLoop() output %q does not explain the refusal", out)
	}
}
