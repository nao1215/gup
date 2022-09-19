package goutil

import (
	"bytes"
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/print"
)

// ============================================================================
//  Functions (Methods follow)
// ============================================================================

func TestBinaryPathList_non_existing_path(t *testing.T) {
	dummyPath := filepath.Join("/non", "existing", "path")
	list, err := BinaryPathList(dummyPath)

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

func Test_extractImportPath_no_import_paths_to_extract(t *testing.T) {
	// Assert to be empty
	got := extractImportPath([]string{})

	if got != "" {
		t.Errorf("extractImportPath() should return empty string. got: %v", got)
	}
}

func Test_extractModulePath_no_module_paths_to_extract(t *testing.T) {
	// Assert to be empty
	got := extractModulePath([]string{})

	if got != "" {
		t.Errorf("extractModulePath() should return empty string. got: %v", got)
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
	wantContain := "can not get package path"
	got := tmpBuff.String()
	if !strings.Contains(got, wantContain) {
		t.Errorf("it should print error message '%v'. got: %v", wantContain, got)
	}
}

func TestGetPackageVersion_getting_error_from_gobin(t *testing.T) {
	// Set env variable to temporary value and defer restore on Cleanup
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	// Backup and defer restore
	oldBuildDefaultGOPATH := build.Default.GOPATH
	defer func() {
		build.Default.GOPATH = oldBuildDefaultGOPATH
	}()

	// Mock the value
	build.Default.GOPATH = ""

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
	oldBuildDefaultGOPATH := build.Default.GOPATH
	defer func() {
		build.Default.GOPATH = oldBuildDefaultGOPATH
	}()

	// Mock the value
	build.Default.GOPATH = ""

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

func Test_goPath_get_from_build_default_gopath(t *testing.T) {
	// Backup and defer restore
	oldKeyGoPath := keyGoPath

	defer func() {
		keyGoPath = oldKeyGoPath
	}()

	// Mock the private global variable.
	// os.Getenv() in the goPath() shuld return empty since it doesn't exist.
	keyGoPath = t.Name()

	// Assert to be equal
	want := os.Getenv("HOME") + "/go"
	got := goPath()

	if want != got {
		t.Errorf("goPath() should return default GOPATH. got: %v, want: %v", got, want)
	}
}

func TestInstall_arg_is_command_line_arguments(t *testing.T) {
	err := Install("command-line-arguments")

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

func TestInstall_golden(t *testing.T) {
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

	err := Install("github.com/nao1215/gup")

	// Require to be no error
	if err != nil {
		t.Fatalf("it should not return error. got: %v", err)
	}
}

func TestIsAlreadyUpToDate_golden(t *testing.T) {
	for i, test := range []struct {
		curr   string
		latest string
		expect bool
	}{
		// Regular cases
		{curr: "v1.9.0", latest: "v1.9.1", expect: false},
		{curr: "v1.9.0", latest: "v1.9.0", expect: true},
		{curr: "v1.9.1", latest: "v1.9.0", expect: true},
		// Irregular cases (untagged versions)
		{
			curr:   "v0.0.0-20220913151710-7c6e287988f3",
			latest: "v0.0.0-20210608161538-9736a4bde949",
			expect: true,
		},
		{
			curr:   "v0.0.0-20210608161538-9736a4bde949",
			latest: "v0.0.0-20220913151710-7c6e287988f3",
			expect: false,
		},
		// Compatibility between go-style semver and pure-semver
		{curr: "v1.9.0", latest: "1.9.1", expect: false},
		{curr: "v1.9.1", latest: "1.9.0", expect: true},
		{curr: "1.9.0", latest: "v1.9.1", expect: false},
		{curr: "1.9.1", latest: "v1.9.0", expect: true},
		// Issue #36
		{curr: "v1.9.1-0.20220908165354-f7355b5d2afa", latest: "v1.9.0", expect: true},
	} {
		verTmp := Version{
			Current: test.curr,
			Latest:  test.latest,
		}

		want := test.expect
		got := IsAlreadyUpToDate(verTmp)

		// Assert to be equal
		if want != got {
			t.Errorf(
				"case #%v failed. got: (\"%v\" >= \"%v\") = %v, want: %v",
				i, test.curr, test.latest, got, want,
			)
		}
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
			expectErrMsg: "invalid argument",
			reasonErr:    "it should be error if field GOBIN is not empty but env key is not set",
			tmpGOBIN:     "dummy", tmpGOPATH: "",
		},
		{
			name:         "case GOPATH is not empty",
			expectErrMsg: "invalid argument",
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
		GOBIN:   "dummy/",
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
			expectErrMsg: "invalid argument",
			reasonErr:    "it should be error if field GOBIN is not empty but env key is not set",
			tmpGOBIN:     "dummy", tmpGOPATH: "",
		},
		{
			name:         "case GOPATH is not empty",
			expectErrMsg: "invalid argument",
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
	}

	// Assert to contain the expected message
	wantContain := "current: v0.0.1, latest: v1.9.1"
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		t.Errorf("got: %v, want: %v", got, wantContain)
	}
}
