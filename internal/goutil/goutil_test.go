//nolint:paralleltest,goconst
package goutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/print"
)

// ============================================================================
//  Functions (Methods follow)
// ============================================================================

func TestBinaryPathList_non_existing_path(t *testing.T) {
	dummyPath := filepath.Join("non", "existing", "path")
	list, err := BinaryPathList(filepath.Clean(dummyPath))

	// Require to be error
	if err == nil {
		t.Fatalf("non-existing path should return error. got: nil")
	}

	// Assert to be nil
	if list != nil {
		t.Errorf("it should return nil on error. got: %v", list)
	}

	// Assert to contain expected error msg
	wantContain := dummyPath
	got := err.Error()
	if !strings.Contains(got, wantContain) {
		t.Errorf("it should return error with message '%v'. got: %v", wantContain, got)
	}
}

// Unit test for [BUG Report] Ignore .DS_Store files on macOS #81
// https://github.com/nao1215/gup/issues/81
func TestBinaryPathList_exclusion(t *testing.T) {
	dummyPath := "testdata"
	got, err := BinaryPathList(filepath.Clean(dummyPath))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(dummyPath, "normal.txt")}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("value is mismatch (-want +got):\n%s", diff)
	}
}

func TestGetLatestVer_unknown_module(t *testing.T) {
	out, err := GetLatestVer(".")

	// Require to be error
	if err == nil {
		t.Fatalf("GetLatestVer() should return error. got: nil")
	}

	// Assert to be empty
	if out != "" {
		t.Errorf("GetLatestVer() should return empty string on error. got: %v", out)
	}
}

func TestGetVerWithContext_unknown_module(t *testing.T) {
	out, err := GetVerWithContext(context.Background(), ".", "master")

	// Require to be error
	if err == nil {
		t.Fatalf("GetVerWithContext() should return error. got: nil")
	}

	// Assert to be empty
	if out != "" {
		t.Errorf("GetVerWithContext() should return empty string on error. got: %v", out)
	}
}

func TestGetVerWithContext_golden(t *testing.T) {
	// Backup and defer restore
	oldGoExe := goExe
	defer func() {
		goExe = oldGoExe
	}()

	// Mock the `go` to `echo` command so that the command succeeds and the
	// requested ref is echoed back as the resolved version.
	goExe = "echo"

	out, err := GetVerWithContext(context.Background(), "github.com/nao1215/gup", "master")
	if err != nil {
		t.Fatalf("GetVerWithContext() should not return error. got: %v", err)
	}
	if !strings.Contains(out, "github.com/nao1215/gup@master") {
		t.Errorf("GetVerWithContext() should query the @master ref. got: %v", out)
	}
}

func TestDetectModulePathMismatch(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantDeclared string
		wantRequired string
		wantOK       bool
	}{
		{
			name: "detect mismatch",
			err: errors.New(`go: github.com/cosmtrek/air@latest: version constraints conflict:
	github.com/cosmtrek/air@v1.52.2: parsing go.mod:
	module declares its path as: github.com/air-verse/air
	        but was required as: github.com/cosmtrek/air`),
			wantDeclared: "github.com/air-verse/air",
			wantRequired: "github.com/cosmtrek/air",
			wantOK:       true,
		},
		{
			name:         "nil error",
			err:          nil,
			wantDeclared: "",
			wantRequired: "",
			wantOK:       false,
		},
		{
			name:         "not mismatch",
			err:          errors.New("some other error"),
			wantDeclared: "",
			wantRequired: "",
			wantOK:       false,
		},
		{
			name: "same path is not mismatch",
			err: errors.New(`module declares its path as: github.com/example/tool
but was required as: github.com/example/tool`),
			wantDeclared: "",
			wantRequired: "",
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			declared, required, ok := DetectModulePathMismatch(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("DetectModulePathMismatch() ok = %v, want %v", ok, tt.wantOK)
			}
			if declared != tt.wantDeclared {
				t.Errorf("declared path = %q, want %q", declared, tt.wantDeclared)
			}
			if required != tt.wantRequired {
				t.Errorf("required path = %q, want %q", required, tt.wantRequired)
			}
		})
	}
}

func TestGetPackageInformation_unknown_module(t *testing.T) {
	// Backup and defer restore STDERR via print package
	oldPrintStderr := print.Stderr
	defer func() {
		print.Stderr = oldPrintStderr
	}()

	// Capture stderr
	var tmpBuff bytes.Buffer

	print.Stderr = &tmpBuff

	result, _ := GetPackageInformation([]string{"unknown-module"})

	// Require to be empty
	if len(result) != 0 {
		t.Fatalf("GetPackageInformation() should return empty Package slice on error. got: %v", result)
	}

	// Assert to contain the expected error message
	wantContain := "could not read Go build info"
	got := tmpBuff.String()
	if !strings.Contains(got, wantContain) {
		t.Errorf("it should print error message '%v'. got: %v", wantContain, got)
	}
}

// TestGetPackageInformation_goVersionFailure verifies the fix for issue #296:
// when "go version" cannot be read, GetPackageInformation reports that the Go
// version is unavailable (second return value false) and warns exactly once,
// instead of silently stamping "unknown" and forcing every binary to reinstall.
func TestGetPackageInformation_goVersionFailure(t *testing.T) {
	oldGoExe := goExe
	oldPrintStderr := print.Stderr
	defer func() {
		goExe = oldGoExe
		print.Stderr = oldPrintStderr
	}()
	// Point the go command at a binary that does not exist so "go version"
	// fails on every OS (unlike "false", which is absent on Windows).
	goExe = "gup-nonexistent-go-command-for-test"

	var stderr bytes.Buffer
	print.Stderr = &stderr

	nameDir := "check_success"
	nameBin := "gal"
	if runtime.GOOS == "windows" {
		nameDir = "check_success_for_windows"
		nameBin = "gal.exe"
	}
	pathBin := filepath.Join("..", "..", "cmd", "testdata", nameDir, nameBin)

	pkgs, goVersionAvailable := GetPackageInformation([]string{pathBin})

	// The Go-version detection failure is reported, not swallowed.
	if goVersionAvailable {
		t.Error("goVersionAvailable should be false when 'go version' fails")
	}
	// Packages are still returned so module-version checks can proceed.
	if len(pkgs) != 1 {
		t.Fatalf("GetPackageInformation() returned %d packages, want 1", len(pkgs))
	}
	// Exactly one warning explains why the Go-version comparison was skipped.
	if c := strings.Count(stderr.String(), "skipping Go-version comparison"); c != 1 {
		t.Errorf("want exactly one Go-version warning, got %d: %q", c, stderr.String())
	}

	// Acceptance criterion (1): an otherwise up-to-date binary must not be
	// flagged for update. Without the fix, the Go version is "unknown" so
	// IsGoUpToDate() is false and, with ignore-go-update off, the Go-version
	// term would force a reinstall. The fix makes callers treat a detection
	// failure as ignore-go-update, which disables that term.
	p := pkgs[0]
	p.Version.Latest = p.Version.Current // module side is up-to-date
	if p.IsGoUpToDate() {
		t.Fatal("precondition: IsGoUpToDate() should be false when the Go version is unknown")
	}
	ignoreGoUpdate := !goVersionAvailable
	if goVersionForcesUpdate := !ignoreGoUpdate && !p.IsGoUpToDate(); goVersionForcesUpdate {
		t.Error("Go-version comparison must be disabled on detection failure so it cannot force a reinstall")
	}
}

func TestGetPackageVersion_golden(t *testing.T) {
	// Backaup and defer restore
	oldKeyGoBin := keyGoBin
	defer func() {
		keyGoBin = oldKeyGoBin
	}()

	// Prepare the path of go binary module for testing
	nameDirCheckSuccess := "check_success"
	nameFileBin := "gal"
	want := "v1.1.1"

	if runtime.GOOS == "windows" {
		nameDirCheckSuccess = "check_success_for_windows"
		nameFileBin = "gal.exe"
		want = "(devel)"
	}

	pathDirBin := filepath.Join("..", "..", "cmd", "testdata", nameDirCheckSuccess)

	// Mock the search directory path of go module binaries
	t.Setenv("GOBINTMP", pathDirBin)
	keyGoBin = "GOBINTMP"

	// Get the package version of specified module
	got := GetPackageVersion(nameFileBin)

	// Require to get the expected version of go module binary
	if want != got {
		t.Fatalf("GetPackageVersion() should return %v. got: %v", want, got)
	}
}

func TestGetPackageVersion_getting_error_from_gobin(t *testing.T) {
	// Set env variable to temporary value and defer restore on Cleanup
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	// Backup and defer restore
	oldKeyGoPath := keyGoPath
	defer func() {
		keyGoPath = oldKeyGoPath
	}()

	// Mock the value
	keyGoPath = t.Name()

	// Setting GOBIN, GOPATH and build.Default.GOPATH to empty string
	// should be an error internally and return "unknown" as a version.
	got := GetPackageVersion(".")

	want := "unknown"
	if want != got {
		t.Errorf("GetPackageVersion() should return %v. got: %v", want, got)
	}
}

func TestGetPackageVersion_package_has_no_version_info(t *testing.T) {
	t.Setenv(keyGoBin, filepath.Join(t.TempDir(), "bin"))
	t.Setenv(keyGoPath, "")

	// Backup and defer restore
	OldGoExe := goExe
	defer func() {
		goExe = OldGoExe
	}()

	// Mock the `go` to `echo` command to print instead of executing.
	// This will succeed executing via `exec.Command` but the output will not
	// contain package version as expected. Thus, it will return "unknown" as
	// a result.
	goExe = "echo"

	want := "unknown"
	got := GetPackageVersion("go")
	if want != got {
		t.Errorf("GetPackageVersion() should return %v. got: %v", want, got)
	}
}

func TestGoBin_gobin_and_gopath_is_empty(t *testing.T) {
	// Set env variable to temporary value and defer restore on Cleanup
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	// Backup and defer restore
	oldKeyGoPath := keyGoPath
	defer func() {
		keyGoPath = oldKeyGoPath
	}()

	// Mock the value
	keyGoPath = t.Name()

	wantContain := "$GOPATH is not set"
	result, got := GoBin()

	// Require to be error
	if got == nil {
		t.Fatalf("it should return error but got nil: %v", result)
	}

	// Assert to be empty
	if result != "" {
		t.Errorf("it should return empty string on error. got: %v", result)
	}

	// Assert to contain the expected error message
	if !strings.Contains(got.Error(), wantContain) {
		t.Errorf("it should return error with message '%v'. got: %v", wantContain, got)
	}
}

func TestGoBin_golden(t *testing.T) {
	// Set env variable to temporary value and defer restore on Cleanup
	t.Setenv(keyGoBin, t.Name())

	want := t.Name()
	got, err := GoBin()

	// Require to be no error
	if err != nil {
		t.Fatalf("GoBin() should return no error. got: %v", err)
	}

	// Assert to be equal
	if want != got {
		t.Errorf("GoBin() should return %v. got: %v", want, got)
	}
}

func TestInstall_arg_is_command_line_arguments(t *testing.T) {
	err := InstallLatest("command-line-arguments")

	// Require to be error
	if err == nil {
		t.Fatalf("it should return error but got nil")
	}

	// Assert to contain the expected error message
	wantContain := "is devel-binary copied from local environment"
	got := err.Error()
	if !strings.Contains(got, wantContain) {
		t.Errorf("it should return error with message '%v'. got: %v", wantContain, got)
	}
}

func TestInstallLatest_golden(t *testing.T) {
	// Backup and defer restore
	OldGoExe := goExe
	defer func() {
		goExe = OldGoExe
	}()

	// Mock the `go` to `echo` command to print instead of executing go.
	//
	// This will succeed executing via `exec.Command` and will not execute the
	// actual `go install <package>` command but `echo install <package>`.
	goExe = "echo"

	err := InstallLatest("github.com/nao1215/gup")

	// Require to be no error
	if err != nil {
		t.Fatalf("it should not return error. got: %v", err)
	}
}

func TestInstall_specificVersion_golden(t *testing.T) {
	// Backup and defer restore
	oldGoExe := goExe
	defer func() {
		goExe = oldGoExe
	}()

	// Mock the `go` to `echo` command to print instead of executing go.
	goExe = "echo"

	err := Install("github.com/nao1215/gup", "v1.0.0")
	if err != nil {
		t.Fatalf("it should not return error. got: %v", err)
	}
}

func TestInstallMaster_golden(t *testing.T) {
	// Backup and defer restore
	OldGoExe := goExe
	defer func() {
		goExe = OldGoExe
	}()

	// Mock the `go` to `echo` command to print instead of executing go.
	//
	// This will succeed executing via `exec.Command` and will not execute the
	// actual `go install <package>` command but `echo install <package>`.
	goExe = "echo"

	err := InstallMainOrMaster("github.com/nao1215/gup")

	// Require to be no error
	if err != nil {
		t.Fatalf("it should not return error. got: %v", err)
	}
}

func TestIsUpToDate_golden(t *testing.T) {
	for i, test := range []struct {
		curr     string
		latest   string
		currGo   string
		latestGo string
		expect   bool
	}{
		// Regular cases
		{curr: "v1.9.0", latest: "v1.9.1", currGo: "go1.22.4", latestGo: "go1.22.4", expect: false},
		{curr: "v1.9.0", latest: "v1.9.0", currGo: "go1.22.4", latestGo: "go1.22.4", expect: true},
		{curr: "v1.9.1", latest: "v1.9.0", currGo: "go1.22.4", latestGo: "go1.22.4", expect: true},
		{curr: "v1.9.0", latest: "v1.9.0", currGo: "go1.22.1", latestGo: "go1.22.4", expect: false},
		// Irregular cases (untagged versions)
		{
			curr:     "v0.0.0-20220913151710-7c6e287988f3",
			latest:   "v0.0.0-20210608161538-9736a4bde949",
			currGo:   "go1.22.4",
			latestGo: "go1.22.4",
			expect:   true,
		},
		{
			curr:     "v0.0.0-20210608161538-9736a4bde949",
			latest:   "v0.0.0-20220913151710-7c6e287988f3",
			currGo:   "go1.22.4",
			latestGo: "go1.22.4",
			expect:   false,
		},
		// Compatibility between go-style semver and pure-semver
		{curr: "v1.9.0", latest: "1.9.1", currGo: "go1.22.4", latestGo: "go1.22.4", expect: false},
		{curr: "v1.9.1", latest: "1.9.0", currGo: "go1.22.4", latestGo: "go1.22.4", expect: true},
		{curr: "1.9.0", latest: "v1.9.1", currGo: "go1.22.4", latestGo: "go1.22.4", expect: false},
		{curr: "1.9.1", latest: "v1.9.0", currGo: "go1.22.4", latestGo: "go1.22.4", expect: true},
		// Issue #36
		{curr: "v1.9.1-0.20220908165354-f7355b5d2afa", latest: "v1.9.0", currGo: "go1.22.4", latestGo: "go1.22.4", expect: true},
	} {
		verTmp := Version{
			Current: test.curr,
			Latest:  test.latest,
		}
		goVerTmp := Version{
			Current: test.currGo,
			Latest:  test.latestGo,
		}
		pkg := Package{Version: &verTmp, GoVersion: &goVerTmp}

		want := test.expect
		got := pkg.IsPackageUpToDate() && pkg.IsGoUpToDate()

		// Assert to be equal
		if want != got {
			t.Errorf(
				"case #%v failed. got: (\"%v\" >= \"%v\" / \"%v\" >= \"%v\") = %v, want: %v",
				i, test.curr, test.latest, test.currGo, test.latestGo, got, want,
			)
		}
	}
}

func TestVersionUpToDate_golden(t *testing.T) {
	type args struct {
		current   string
		available string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "basic test",
			args: args{
				current:   "1.0.0",
				available: "1.0.1",
			},
			want: false,
		},
		{
			name: "unknown treated as newer",
			args: args{
				current:   "1.0.0",
				available: "unknown",
			},
			want: false,
		},
		{
			name: "differing digits, single older",
			args: args{
				current:   "1.2.0",
				available: "1.11.5",
			},
			want: false,
		},
		{
			name: "same version",
			args: args{
				current:   "1.0.0",
				available: "1.0.0",
			},
			want: true,
		},
		{
			name: "current newer",
			args: args{
				current:   "2.0.0",
				available: "1.0.0",
			},
			want: true,
		},
		{
			name: "current patch newer",
			args: args{
				current:   "1.0.1",
				available: "1.0.0",
			},
			want: true,
		},
		{
			name: "current minor newer",
			args: args{
				current:   "1.1.0",
				available: "1.0.1",
			},
			want: true,
		},
		{
			name: "different lengths, current newer",
			args: args{
				current:   "1.0",
				available: "0.9.9",
			},
			want: true,
		},
		{
			name: "additional test, current older major version",
			args: args{
				current:   "0.9.9",
				available: "1.0.0",
			},
			want: false,
		},
		{
			name: "additional test, current older minor version",
			args: args{
				current:   "1.0.0",
				available: "1.1.0",
			},
			want: false,
		},
		{
			name: "additional test, current older patch version",
			args: args{
				current:   "1.0.0",
				available: "1.0.1",
			},
			want: false,
		},
		{
			name: "additional test, current much older version",
			args: args{
				current:   "1.0.0",
				available: "2.0.0",
			},
			want: false,
		},
		{
			name: "additional test, current much newer version",
			args: args{
				current:   "2.0.0",
				available: "1.0.0",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionUpToDate(tt.args.current, tt.args.available)
			if got != tt.want {
				t.Errorf("versionUpToDate() test_name=%s, got = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func Test_isModuleBinary(t *testing.T) {
	// The argument is the binary's main module path (buildinfo Main.Path), not
	// its import path.
	for _, tt := range []struct {
		name           string
		mainModulePath string
		want           bool
	}{
		{name: "stdlib/toolchain has no main module", mainModulePath: "", want: false},
		{name: "github.com third-party", mainModulePath: "github.com/nao1215/gup", want: true},
		{name: "golang.org third-party", mainModulePath: "golang.org/x/tools", want: true},
		// Dotless hosts are still third-party modules and must not be dropped
		// as if they were the standard library (issue #299).
		{name: "dotless localhost host", mainModulePath: "localhost/tool", want: true},
		{name: "dotless internal registry host", mainModulePath: "registry/team/tool", want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := isModuleBinary(tt.mainModulePath)
			if got != tt.want {
				t.Errorf("isModuleBinary(%q) = %v, want %v", tt.mainModulePath, got, tt.want)
			}
		})
	}
}

func TestGetPackageInformation_std_cmd_filtered(t *testing.T) {
	// Find gofmt binary, which is a standard library command (Path: "cmd/gofmt",
	// no main module). GetPackageInformation should filter it out because it
	// records no main module.
	ctx := context.Background()
	goroot, err := exec.CommandContext(ctx, "go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("could not determine GOROOT: %v", err)
	}
	gofmt := filepath.Join(strings.TrimSpace(string(goroot)), "bin", "gofmt")
	if runtime.GOOS == "windows" {
		gofmt += ".exe"
	}
	if _, err := os.Stat(gofmt); err != nil {
		t.Skipf("gofmt not found at %s: %v", gofmt, err)
	}

	result, _ := GetPackageInformation([]string{gofmt})
	if len(result) != 0 {
		t.Errorf("GetPackageInformation() should filter standard library commands, got: %v", result)
	}
}

func TestNormalizeUpdateChannel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want UpdateChannel
	}{
		{name: "latest", in: "latest", want: UpdateChannelLatest},
		{name: "main", in: "main", want: UpdateChannelMain},
		{name: "master", in: "master", want: UpdateChannelMaster},
		{name: "upper case", in: "MAIN", want: UpdateChannelMain},
		{name: "blank defaults latest", in: "", want: UpdateChannelLatest},
		{name: "unknown defaults latest", in: "snapshot", want: UpdateChannelLatest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeUpdateChannel(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeUpdateChannel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ============================================================================
//  Methods
// ============================================================================

// ----------------------------------------------------------------------------
//	Type: GoPaths
// ----------------------------------------------------------------------------

func TestGoPaths_EndDryRunMode_fail_if_key_not_set(t *testing.T) {
	// Backup and defer restore
	oldKeyGoBin := keyGoBin
	oldKeyGoPath := keyGoPath
	defer func() {
		keyGoBin = oldKeyGoBin
		keyGoPath = oldKeyGoPath
	}()

	// Mock the key name of the environment variable as empty
	keyGoBin = ""
	keyGoPath = ""

	for i, tt := range []struct {
		name         string
		expectErrMsg string
		reasonErr    string
		tmpGOBIN     string
		tmpGOPATH    string
	}{
		{
			name:         "case GOBIN and GOPATH are empty",
			expectErrMsg: "$GOPATH and $GOBIN is not set",
			reasonErr:    "it should be error if both field GOBIN and GOPATH is empty and env key is not set",
			tmpGOBIN:     "", tmpGOPATH: "",
		},
		{
			name:         "case GOBIN is not empty",
			expectErrMsg: "failed to set GOBIN to env variable",
			reasonErr:    "it should be error if field GOBIN is not empty but env key is not set",
			tmpGOBIN:     "dummy", tmpGOPATH: "",
		},
		{
			name:         "case GOPATH is not empty",
			expectErrMsg: "failed to set GOPATH to env variable",
			reasonErr:    "it should be error if field GOPATH is not empty but env key is not set",
			tmpGOBIN:     "", tmpGOPATH: "dummy",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gp := GoPaths{
				GOBIN:  tt.tmpGOBIN,
				GOPATH: tt.tmpGOPATH,
			}

			err := gp.EndDryRunMode()

			// Require to be error
			if err == nil {
				t.Fatalf("case #%v: EndDryRunMode() should return error. got: nil", i+1)
			}

			// Assert to contain the expected error message
			wantContain := tt.expectErrMsg
			got := err.Error()

			if !strings.Contains(got, wantContain) {
				t.Errorf("case #%v: %v. got: %v, want: %v",
					i+1, tt.reasonErr, got, wantContain,
				)
			}
		})
	}
}

func TestGoPaths_EndDryRunMode_fail_to_remove_temp_dir(t *testing.T) {
	// Backup environment variables. t.Setenv restores on cleanup.
	oldGOBIN := os.Getenv("GOBIN")
	oldGOPATH := os.Getenv("GOPATH")
	t.Setenv("GOBIN", oldGOBIN)
	t.Setenv("GOPATH", oldGOPATH)

	gp := GoPaths{
		GOBIN:   "dummy",
		GOPATH:  "",
		TmpPath: ".", // os.RemoveAll(".") will fail
	}

	// Note: This test should cover the removeTmpDir() method as well
	err := gp.EndDryRunMode()

	// Require to be error
	if err == nil {
		t.Fatal("removeTmpDir() should return error removing '.' directory. got: nil")
	}

	// Assert to contain expected error msg
	wantContain := "temporary directory for dry run remains"
	got := err.Error()

	if !strings.Contains(got, wantContain) {
		t.Errorf("removeTmpDir() should return error with message '%v'. got: %v", wantContain, got)
	}
}

// TestGoPaths_EndDryRunMode_removesTmpDir_even_if_restore_fails covers the issue
// #297 fix: when restoring the env variable fails, the temporary directory must
// still be removed instead of being leaked.
func TestGoPaths_EndDryRunMode_removesTmpDir_even_if_restore_fails(t *testing.T) {
	oldKeyGoBin := keyGoBin
	defer func() { keyGoBin = oldKeyGoBin }()
	// An empty key name makes os.Setenv fail, so the env restore errors out.
	keyGoBin = ""

	// A real temporary directory that EndDryRunMode is expected to remove.
	tmpDir := filepath.Join(t.TempDir(), "dryrun")
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		t.Fatalf("failed to set up temp dir: %v", err)
	}

	gp := GoPaths{GOBIN: "dummy", TmpPath: tmpDir}

	err := gp.EndDryRunMode()
	if err == nil {
		t.Fatal("EndDryRunMode() should return error when env restore fails. got: nil")
	}
	if !strings.Contains(err.Error(), "failed to set GOBIN to env variable") {
		t.Errorf("error should report the restore failure. got: %v", err)
	}

	// The temp directory must be gone even though the restore failed.
	if _, statErr := os.Stat(tmpDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("temp dir should be removed even when env restore fails. stat err: %v", statErr)
	}
}

// TestGoPaths_EndDryRunMode_joins_restore_and_remove_errors verifies that when
// both the env restore and the temp-dir removal fail, the returned error
// reports both problems.
func TestGoPaths_EndDryRunMode_joins_restore_and_remove_errors(t *testing.T) {
	oldKeyGoBin := keyGoBin
	defer func() { keyGoBin = oldKeyGoBin }()
	keyGoBin = "" // env restore fails

	gp := GoPaths{GOBIN: "dummy", TmpPath: "."} // os.RemoveAll(".") fails

	err := gp.EndDryRunMode()
	if err == nil {
		t.Fatal("EndDryRunMode() should return error when both restore and removal fail. got: nil")
	}
	got := err.Error()
	if !strings.Contains(got, "failed to set GOBIN to env variable") {
		t.Errorf("joined error should report the restore failure. got: %v", got)
	}
	if !strings.Contains(got, "temporary directory for dry run remains") {
		t.Errorf("joined error should report the removal failure. got: %v", got)
	}
}

// TestGoPaths_StartDryRunMode_removesTmpDir_when_setenv_fails verifies that a
// failed env mutation during StartDryRunMode does not leak the temp directory.
func TestGoPaths_StartDryRunMode_removesTmpDir_when_setenv_fails(t *testing.T) {
	oldKeyGoBin := keyGoBin
	defer func() { keyGoBin = oldKeyGoBin }()
	keyGoBin = "" // makes os.Setenv fail after the temp dir is created

	gp := GoPaths{GOBIN: "dummy"}

	err := gp.StartDryRunMode()
	if err == nil {
		t.Fatal("StartDryRunMode() should return error when env mutation fails. got: nil")
	}
	if gp.TmpPath == "" {
		t.Fatal("StartDryRunMode() should have created a temp dir before failing")
	}
	if _, statErr := os.Stat(gp.TmpPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("temp dir should be removed when StartDryRunMode fails. stat err: %v", statErr)
	}
}

func TestGoPaths_StartDryRunMode_fail_to_get_temp_dir(t *testing.T) {
	// Backup and defer restore
	OldOsMkdirTemp := osMkdirTemp
	defer func() {
		osMkdirTemp = OldOsMkdirTemp
	}()

	expectErrMsg := "forced dummy error"

	// Mock the function to force return error
	osMkdirTemp = func(dir, pattern string) (name string, err error) {
		return "", errors.New(expectErrMsg)
	}

	gp := GoPaths{}

	err := gp.StartDryRunMode()

	// Require to be error
	if err == nil {
		t.Fatalf("if should return error but got nil")
	}

	// Assert to contain expected error msg
	if !strings.Contains(err.Error(), expectErrMsg) {
		t.Errorf(
			"it did not contain the expected error msg. got: %v, want contain: %v",
			err.Error(), expectErrMsg,
		)
	}
}

func TestGoPaths_StartDryRunMode_setsTmpPath(t *testing.T) {
	t.Setenv("GOBIN", t.Name())
	t.Setenv("GOPATH", "")

	gp := NewGoPaths()
	if gp.GOBIN == "" {
		t.Fatal("test setup failed: GOBIN should not be empty")
	}

	if err := gp.StartDryRunMode(); err != nil {
		t.Fatalf("StartDryRunMode() should return no error. got: %v", err)
	}

	if gp.TmpPath == "" {
		t.Fatal("StartDryRunMode() should set TmpPath")
	}

	if got := os.Getenv("GOBIN"); got != gp.TmpPath {
		t.Fatalf("StartDryRunMode() should set GOBIN to TmpPath. got: %s, want: %s", got, gp.TmpPath)
	}

	if _, err := os.Stat(gp.TmpPath); err != nil {
		t.Fatalf("temporary directory should exist while dry-run is active. err: %v", err)
	}

	if err := gp.EndDryRunMode(); err != nil {
		t.Fatalf("EndDryRunMode() should return no error. got: %v", err)
	}

	if got := os.Getenv("GOBIN"); got != t.Name() {
		t.Fatalf("EndDryRunMode() should restore GOBIN. got: %s, want: %s", got, t.Name())
	}

	if _, err := os.Stat(gp.TmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary directory should be removed after EndDryRunMode(). err: %v", err)
	}
}

func TestGoPaths_StartDryRunMode_fail_if_key_not_set(t *testing.T) {
	// Backup and defer restore
	oldKeyGoBin := keyGoBin
	oldKeyGoPath := keyGoPath
	defer func() {
		keyGoBin = oldKeyGoBin
		keyGoPath = oldKeyGoPath
	}()

	// Mock the key name of the environment variable as empty
	keyGoBin = ""
	keyGoPath = ""

	for i, tt := range []struct {
		name         string
		expectErrMsg string
		reasonErr    string
		tmpGOBIN     string
		tmpGOPATH    string
	}{
		{
			name:         "case GOBIN and GOPATH are empty",
			expectErrMsg: "$GOPATH and $GOBIN is not set",
			reasonErr:    "it should be error if both field GOBIN and GOPATH is empty and env key is not set",
			tmpGOBIN:     "", tmpGOPATH: "",
		},
		{
			name:         "case GOBIN is not empty",
			expectErrMsg: "failed to set GOBIN to env variable",
			reasonErr:    "it should be error if field GOBIN is not empty but env key is not set",
			tmpGOBIN:     "dummy", tmpGOPATH: "",
		},
		{
			name:         "case GOPATH is not empty",
			expectErrMsg: "failed to set GOPATH to env variable",
			reasonErr:    "it should be error if field GOPATH is not empty but env key is not set",
			tmpGOBIN:     "", tmpGOPATH: "dummy",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gp := GoPaths{
				GOBIN:  tt.tmpGOBIN,
				GOPATH: tt.tmpGOPATH,
			}

			err := gp.StartDryRunMode()

			// Require to be error
			if err == nil {
				t.Fatalf("case #%v: StartDryRunMode() should return error. got: nil", i+1)
			}

			// Assert to contain the expected error message
			expectContain := tt.expectErrMsg
			got := err.Error()

			if !strings.Contains(got, expectContain) {
				t.Errorf("case #%v: %v. got: %v, want: %v",
					i+1, tt.reasonErr, got, expectContain,
				)
			}
		})
	}
}

// ----------------------------------------------------------------------------
//  Type: Package
// ----------------------------------------------------------------------------

func TestPackage_CurrentToLatestStr_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v1.42.2",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := "up-to-date: v1.42.2"
	got := pkgInfo.CurrentToLatestStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_CurrentToLatestStr_not_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v0.0.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := "v0.0.1 to v1.9.1"
	got := pkgInfo.CurrentToLatestStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_CurrentToLatestStr_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v0.0.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	wantContain := color.YellowString("v0.0.1") + " to " + color.GreenString("v1.9.1")
	got := pkgInfo.CurrentToLatestStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v2.5.0",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := "up-to-date: v2.5.0"
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_not_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v0.0.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := "current: v0.0.1, latest: v1.9.1"
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v0.0.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.4",
			Latest:  "go1.22.4",
		},
	}

	wantContain := "current: " + color.YellowString("v0.0.1") + ", latest: " + color.GreenString("v1.9.1")
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_go_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.99.9",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := regexp.MustCompile(`up-to-date:.* go1\.99\.9`)
	got := pkgInfo.VersionCheckResultStr()

	if !wantContain.MatchString(got) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_go_not_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.1",
			Latest:  "go1.22.4",
		},
	}

	// Assert to contain the expected message
	wantContain := "current: go1.22.1, installed: go1.22.4"
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_VersionCheckResultStr_go_not_up_to_date_color(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		ModulePath: "github.com/dummy_name/dummy/foo",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.1",
			Latest:  "go1.22.4",
		},
	}

	wantContain := "current: " + color.YellowString("go1.22.1") + ", installed: " + color.GreenString("go1.22.4")
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}

func TestPackage_CurrentToLatestStr_go_not_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.1",
			Latest:  "go1.22.4",
		},
	}

	got := pkgInfo.CurrentToLatestStr()
	if !strings.Contains(got, "go1.22.1") || !strings.Contains(got, "go1.22.4") {
		t.Errorf("expected go version range, got: %v", got)
	}
}

func TestPackage_CurrentToLatestStr_go_customBuildTag_color(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.25.0-X:nodwarf5",
			Latest:  "go1.26.0-X:nodwarf5",
		},
	}

	got := pkgInfo.CurrentToLatestStr()
	if !strings.Contains(got, "go1.25.0-X:nodwarf5") {
		t.Fatalf("expected current custom go version in output, got: %q", got)
	}
	if !strings.Contains(got, "go1.26.0-X:nodwarf5") {
		t.Fatalf("expected latest custom go version in output, got: %q", got)
	}
}

func TestNormalizeGoVersionForCompare(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "keep semver chars", in: "1.26.0-X.nodwarf5+meta", want: "1.26.0-X.nodwarf5+meta"},
		{name: "replace colon", in: "1.26.0-X:nodwarf5", want: "1.26.0-X.nodwarf5"},
		{name: "replace tilde", in: "1.26.0-X~nodwarf5", want: "1.26.0-X.nodwarf5"},
		{name: "trim spaces", in: " 1.26.0-X:nodwarf5 ", want: "1.26.0-X.nodwarf5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeGoVersionForCompare(tt.in); got != tt.want {
				t.Fatalf("normalizeGoVersionForCompare(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPackage_CurrentToLatestStr_both_not_up_to_date(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		Version: &Version{
			Current: "v0.0.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.22.1",
			Latest:  "go1.22.4",
		},
	}

	got := pkgInfo.CurrentToLatestStr()
	if !strings.Contains(got, "v0.0.1") || !strings.Contains(got, "v1.9.1") {
		t.Errorf("expected package version range, got: %v", got)
	}
	if !strings.Contains(got, "go1.22.1") || !strings.Contains(got, "go1.22.4") {
		t.Errorf("expected go version range, got: %v", got)
	}
}

func TestPackage_IsGoUpToDate_customBuildTag(t *testing.T) {
	pkgInfo := Package{
		Name:       "foo",
		ImportPath: "github.com/dummy_name/dummy",
		Version: &Version{
			Current: "v1.9.1",
			Latest:  "v1.9.1",
		},
		GoVersion: &Version{
			Current: "go1.26.0-X:nodwarf5",
			Latest:  "go1.26.0-X:nodwarf5",
		},
	}

	if !pkgInfo.IsGoUpToDate() {
		t.Fatal("custom Go build tag with ':' should be treated as up-to-date when equal")
	}

	if got := pkgInfo.CurrentToLatestStr(); !strings.Contains(got, "Already up-to-date") {
		t.Fatalf("CurrentToLatestStr() = %q, want to include 'Already up-to-date'", got)
	}
}

func TestInstallMainOrMaster_mainFailsGenericError_noMasterFallback(t *testing.T) {
	oldGoExe := goExe
	defer func() { goExe = oldGoExe }()

	// "false" exits non-zero with no "unknown revision main" on stderr, i.e. a
	// generic (non-branch-not-found) failure. Per #340 gup must surface the
	// @main error and must NOT fall back to @master.
	goExe = "false"

	err := InstallMainOrMaster("github.com/example/tool")
	if err == nil {
		t.Fatal("expected error when @main fails")
	}
	if !strings.Contains(err.Error(), "can't install github.com/example/tool") {
		t.Errorf("error should surface the @main install failure, got: %v", err)
	}
	if strings.Contains(err.Error(), "cannot update with @master or @main") {
		t.Errorf("a generic @main failure must not trigger the @master fallback, got: %v", err)
	}
}

func TestVersionUpToDate_invalidVersion(t *testing.T) {
	// invalid versions should return false
	if versionUpToDate("not-a-version", "1.0.0") {
		t.Error("invalid current version should return false")
	}
	if versionUpToDate("1.0.0", "not-a-version") {
		t.Error("invalid available version should return false")
	}
}

func TestGetPackageInformation_emptyList(t *testing.T) {
	result, _ := GetPackageInformation([]string{})
	if result != nil {
		t.Errorf("expected nil for empty list, got %v", result)
	}
}

const timeoutTestImportPath = "github.com/nao1215/posixer"

// expiredContext returns a context whose deadline is already in the past.
func expiredContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	t.Cleanup(cancel)
	return ctx
}

// canceledContext returns a context that has already been canceled.
func canceledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestInstallWithContext_Timeout(t *testing.T) {
	err := InstallWithContext(expiredContext(t), timeoutTestImportPath, "latest")
	if err == nil {
		t.Fatal("InstallWithContext should fail when the context deadline is exceeded")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should report a timeout, got: %v", err)
	}
	// The timeout error must name the runnable command and the --timeout hint
	// so users can rerun or relax the limit (issue #318).
	if !strings.Contains(err.Error(), "go install "+timeoutTestImportPath+"@latest") {
		t.Errorf("error should include the runnable go install command, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--timeout") {
		t.Errorf("error should hint at the --timeout flag, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should wrap context.DeadlineExceeded, got: %v", err)
	}
}

func TestInstallWithContext_Cancel(t *testing.T) {
	err := InstallWithContext(canceledContext(t), timeoutTestImportPath, "latest")
	if err == nil {
		t.Fatal("InstallWithContext should fail when the context is canceled")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("error should report a cancellation, got: %v", err)
	}
}

func TestGetLatestVerWithContext_Timeout(t *testing.T) {
	_, err := GetLatestVerWithContext(expiredContext(t), timeoutTestImportPath)
	if err == nil {
		t.Fatal("GetLatestVerWithContext should fail when the context deadline is exceeded")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should report a timeout, got: %v", err)
	}
	// The version-check timeout uses "go list" (not "go install") and hints at
	// the --timeout flag (issue #318).
	if !strings.Contains(err.Error(), "go list -m "+timeoutTestImportPath+"@latest") {
		t.Errorf("error should include the runnable go list command, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--timeout") {
		t.Errorf("error should hint at the --timeout flag, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should wrap context.DeadlineExceeded, got: %v", err)
	}
}

func TestGetLatestVerWithContext_Cancel(t *testing.T) {
	_, err := GetLatestVerWithContext(canceledContext(t), timeoutTestImportPath)
	if err == nil {
		t.Fatal("GetLatestVerWithContext should fail when the context is canceled")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("error should report a cancellation, got: %v", err)
	}
}

// errorCauseAfterPrefix returns the part of the error message after the
// "<prefix>:\n" header, trimmed. It is empty when the error carries no cause.
func errorCauseAfterPrefix(t *testing.T, err error, prefix string) string {
	t.Helper()
	return strings.TrimSpace(strings.TrimPrefix(err.Error(), prefix+":"))
}

// TestInstallWithContext_EmptyStderrFallback covers issue #298: when the go
// subprocess fails without writing to stderr (as a killed process does), the
// error must still name a cause (from err.Error()) instead of being empty.
func TestInstallWithContext_EmptyStderrFallback(t *testing.T) {
	oldGoExe := goExe
	defer func() { goExe = oldGoExe }()
	// A missing command fails on every OS while leaving stderr empty, and with
	// context.Background() ctx.Err() is nil so the fallback branch is reached.
	goExe = "gup-nonexistent-go-command-for-test"

	err := InstallWithContext(context.Background(), timeoutTestImportPath, "latest")
	if err == nil {
		t.Fatal("InstallWithContext should fail when the go command is missing")
	}
	if !strings.Contains(err.Error(), "can't install") {
		t.Errorf("error should report the install failure, got: %v", err)
	}
	prefix := fmt.Sprintf("can't install %s", timeoutTestImportPath)
	if errorCauseAfterPrefix(t, err, prefix) == "" {
		t.Errorf("error should include a non-empty cause when stderr is empty, got: %v", err)
	}
}

// TestGetVerWithContext_EmptyStderrFallback is the GetVerWithContext counterpart
// of TestInstallWithContext_EmptyStderrFallback (issue #298).
func TestGetVerWithContext_EmptyStderrFallback(t *testing.T) {
	oldGoExe := goExe
	defer func() { goExe = oldGoExe }()
	goExe = "gup-nonexistent-go-command-for-test"

	_, err := GetVerWithContext(context.Background(), timeoutTestImportPath, "latest")
	if err == nil {
		t.Fatal("GetVerWithContext should fail when the go command is missing")
	}
	if !strings.Contains(err.Error(), "can't check") {
		t.Errorf("error should report the version-check failure, got: %v", err)
	}
	prefix := fmt.Sprintf("can't check %s", timeoutTestImportPath)
	if errorCauseAfterPrefix(t, err, prefix) == "" {
		t.Errorf("error should include a non-empty cause when stderr is empty, got: %v", err)
	}
}

// benchBinarySources are real Go binaries (with build info) used to populate
// synthetic GOBIN directories for benchmarks.
func benchBinarySources(b *testing.B) []string {
	b.Helper()
	base := filepath.Join("..", "..", "cmd", "testdata", "check_success")
	srcs := []string{
		filepath.Join(base, "gal"),
		filepath.Join(base, "posixer"),
		filepath.Join(base, "subaru"),
	}
	for _, s := range srcs {
		if _, err := os.Stat(s); err != nil {
			b.Skipf("benchmark fixtures unavailable: %v", err)
		}
	}
	return srcs
}

// benchSetupGobin copies real Go binaries into a fresh temp dir until it holds
// n files, returning the directory path.
func benchSetupGobin(b *testing.B, n int) string {
	b.Helper()
	srcs := benchBinarySources(b)
	dir := b.TempDir()
	for i := 0; i < n; i++ {
		data, err := os.ReadFile(srcs[i%len(srcs)])
		if err != nil {
			b.Fatal(err)
		}
		dst := filepath.Join(dir, fmt.Sprintf("bin%04d", i))
		//nolint:gosec // dst is under b.TempDir(); not user-controlled.
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

func BenchmarkGetPackageInformation(b *testing.B) {
	for _, n := range []int{3, 30, 150} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dir := benchSetupGobin(b, n)
			list, err := BinaryPathList(dir)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = GetPackageInformation(list)
			}
		})
	}
}

func BenchmarkBinaryPathList(b *testing.B) {
	for _, n := range []int{3, 30, 150} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dir := benchSetupGobin(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := BinaryPathList(dir); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGetInstalledGoVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := GetInstalledGoVersion(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoBin(b *testing.B) {
	b.Setenv("GOBIN", b.TempDir())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := GoBin(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetPackageInformationWithoutGoVersion(b *testing.B) {
	for _, n := range []int{3, 30, 150} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dir := benchSetupGobin(b, n)
			list, err := BinaryPathList(dir)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = GetPackageInformationWithoutGoVersion(list)
			}
		})
	}
}
