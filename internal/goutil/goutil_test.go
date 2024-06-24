package goutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

func TestGetPackageInformation_unknown_module(t *testing.T) {
	// Backup and defer restore STDERR via print package
	oldPrintStderr := print.Stderr
	defer func() {
		print.Stderr = oldPrintStderr
	}()

	// Capture stderr
	var tmpBuff bytes.Buffer

	print.Stderr = &tmpBuff

	result := GetPackageInformation([]string{"unknown-module"})

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

func TestIsAlreadyUpToDate_golden(t *testing.T) {
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
		got := pkg.IsAlreadyUpToDate()

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
	// Backup and defer restore since EndDryRunMode will change the env
	// variables to the values stored in the struct.
	oldGOBIN := os.Getenv("GOBIN")
	oldGOPATH := os.Getenv("GOPATH")

	defer func() {
		os.Setenv("GOBIN", oldGOBIN)
		os.Setenv("GOPATH", oldGOPATH)
	}()

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
