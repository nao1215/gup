package goutil_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
)

// ============================================================================
//  Tests for public functions
// ============================================================================

func ExampleBinaryPathList() {
	// Get list of files in the current directory
	got, err := goutil.BinaryPathList(".")
	if err != nil {
		log.Fatal(err)
	}

	want := []string{
		"examples_test.go",
		"goutil.go",
		"goutil_test.go",
	}

	if cmp.Equal(got, want) {
		fmt.Println("Example BinaryPathList: OK")
	}
	// Output: Example BinaryPathList: OK
}

func ExampleCanUseGoCmd() {
	// If `go` command is available, CanUseGoCmd returns no error
	if err := goutil.CanUseGoCmd(); err != nil {
		log.Fatal(err) // no go command found
	}

	fmt.Println("Example CanUseGoCmd: OK")
	// Output: Example CanUseGoCmd: OK
}

func ExampleGetLatestVer() {
	// Get the latest version of a package
	verLatest, err := goutil.GetLatestVer("github.com/mattn/go-colorable")
	if err != nil {
		log.Fatal(err)
	}

	// As of 2022/09/17, the latest version of go-colorable is v0.1.13
	expectMin := "v0.1.13"

	if strings.Compare(expectMin, verLatest) <= 0 {
		fmt.Println("Example GetLatestVer: OK")
	} else {
		log.Fatalf("latest version is older than expected. expect: %s, latest: %s",
			expectMin, verLatest)
	}
	// Output: Example GetLatestVer: OK
}

func ExampleGetPackageInformation() {
	// Prepare the path of go binary module for the example
	nameDirCheckSuccess := "check_success"
	nameFileBin := "gal"

	if runtime.GOOS == "windows" {
		nameDirCheckSuccess = "check_success_for_windows"
		nameFileBin = "gal.exe" // remember the extension
	}

	pathFileBin := filepath.Join("..", "..", "cmd", "testdata", nameDirCheckSuccess, nameFileBin)

	pkgInfo := goutil.GetPackageInformation([]string{pathFileBin})
	if pkgInfo == nil {
		log.Fatal("example GetPackageInformation failed. The returned package information is nil")
	}

	// Expected package information on Linux and macOS
	want := []string{
		nameFileBin,
		"github.com/nao1215/gal/cmd/gal",
		"github.com/nao1215/gal",
	}

	// Actual package information
	got := []string{
		pkgInfo[0].Name,
		pkgInfo[0].ImportPath,
		pkgInfo[0].ModulePath,
	}

	if cmp.Equal(got, want) {
		fmt.Println("Example GetPackageInformation: OK")
	} else {
		log.Fatalf("example GetPackageInformation failed. got: %#v, want: %#v", got, want)
	}
	// Output: Example GetPackageInformation: OK
}

func ExampleGetPackageVersion_unknown() {
	// GetPackageVersion returns the version of the package installed via `go install`.
	// In this example, we specify a package that is not installed.
	got := goutil.GetPackageVersion("gup_dummy")

	// Non existing binary returns "unknown"
	want := "unknown"

	if got == want {
		fmt.Println("Example GetPackageVersion: OK")
	} else {
		log.Fatalf(
			"example GetPackageVersion failed. unexpected return. got: %s, want: %s",
			got, want,
		)
	}
	// Output: Example GetPackageVersion: OK
}

func ExampleGoBin() {
	pathDirGoBin, err := goutil.GoBin()
	if err != nil {
		log.Fatal(err)
	}

	// By default, GoBin returns the value of GOBIN or GOPATH environment variable.
	// But note that on race condition `os.Getenv()` may return a temporary
	// directory. Such as `/bin` on U*ix environments.
	if pathDirGoBin == "" {
		log.Fatal("example GoBin failed. path to go binary is empty")
	}

	fmt.Println("Example GoBin: OK")
	// Output: Example GoBin: OK
}

func ExampleInstallLatest() {
	// Install installs an executable from a Go package.
	err := goutil.InstallLatest("example.com/unknown_user/unknown_package")

	// If the package is not found or invalid, Install returns an error.
	// In this case it should be an error.
	if err == nil {
		log.Fatal("example Install failed. non existing/invalid package should return error")
	}

	// Error message should contain the package path
	expectMsg := "can't install example.com/unknown_user/unknown_package"

	if strings.Contains(err.Error(), expectMsg) {
		fmt.Println("Example Install: OK")
	} else {
		fmt.Println(err.Error())
	}
	// Output: Example Install: OK
}

func ExamplePackage_IsUpToDate() {
	// Create Version object with Current and Latest package and Go versions
	ver := goutil.Version{
		Current: "v1.9.0",
		Latest:  "v1.9.1",
	}
	goVer := goutil.Version{
		Current: "go1.21.1",
		Latest:  "go1.22.4",
	}
	pkg := goutil.Package{Version: &ver, GoVersion: &goVer}

	// Check if Current is already up to date (expected: false)
	if pkg.IsPackageUpToDate() && pkg.IsGoUpToDate() {
		fmt.Println("Example IspToDate: already up to date.")
	} else {
		fmt.Println("Example IsUpToDate: outdated. Newer latest version or installed Go toolchain exists.")
	}

	// Output: Example IsUpToDate: outdated. Newer latest version or installed Go toolchain exists.
}

func ExampleNewGoPaths() {
	// Instantiate GoPaths object
	gp := goutil.NewGoPaths()

	// By default, NewGoPaths returns a GoPaths object with the value of GOBIN
	// or GOPATH environment variable of Go. But note that on race condition
	// `os.Getenv()` may return a temporary directory.
	if gp.GOBIN == "" && gp.GOPATH == "" {
		log.Fatal("example NewGoPaths failed. both GOBIN and GOPATH are empty")
	}

	fmt.Println("Example NewGoPaths: OK")
	// Output: Example NewGoPaths: OK
}

func ExampleNewVersion() {
	// Instantiate Version object
	ver := goutil.NewVersion()

	// By default, Current and Latest fields are empty
	if ver.Current != "" {
		log.Fatal("example NewVersion failed. the field Current is not empty")
	}

	if ver.Latest != "" {
		log.Fatal("example NewVersion failed. the field Latest is not empty")
	}

	fmt.Println("Example NewVersion: OK")
	// Output: Example NewVersion: OK
}

// ============================================================================
//  Tests for public methods
// ============================================================================

// ----------------------------------------------------------------------------
//  Type: GoPaths
// ----------------------------------------------------------------------------

func ExampleGoPaths_StartDryRunMode() {
	gh := goutil.NewGoPaths()

	// StartDryRunMode starts dry run mode. In dry run mode, GoPaths will temporarily
	// change the OS env variables of GOBIN or GOPATH. The original values will be
	// restored when the `EndDryRunMode` method is called.
	if err := gh.StartDryRunMode(); err != nil {
		log.Fatalf("example GoPaths.StartDryRunMode failed to start dry mode: %s", err.Error())
	}

	onDryRunMode := []string{
		os.Getenv("GOBIN"),
		os.Getenv("GOPATH"),
	}

	// End dry run mode.
	if err := gh.EndDryRunMode(); err != nil {
		log.Fatalf("example GoPaths.StartDryRunMode failed to end dry mode: %s", err.Error())
	}

	offDryRunMode := []string{
		os.Getenv("GOBIN"),
		os.Getenv("GOPATH"),
	}

	if cmp.Equal(onDryRunMode, offDryRunMode) {
		log.Fatal("example GoPaths.StartDryRunMode failed. dry run mode did not change to temp dir")
	}

	fmt.Println("Example GoPaths.StartDryRunMode: OK")
	// Output: Example GoPaths.StartDryRunMode: OK
}

// ----------------------------------------------------------------------------
//
//	Type: Package
//
// ----------------------------------------------------------------------------
func ExamplePackage_SetLatestVer() {
	packages := goutil.GetPackageInformation([]string{"../../cmd/testdata/check_success/gal"})
	if len(packages) == 0 {
		log.Fatal("example GetPackageInformation failed. The returned package information is nil")
	}

	// test with the first package found
	pkgInfo := packages[0]

	// By default, the Latest field of Package object is empty
	before := pkgInfo.Version.Latest

	// Execute method and update the Version.Latest field
	pkgInfo.SetLatestVer()

	// After calling SetLatestVer, the Latest field should be updated with the latest
	// version or `unknown` if the latest version is not found.
	after := pkgInfo.Version.Latest

	// Require the field to be updated
	if before == after {
		log.Fatalf(
			"example Package.SetLatestVer failed. The latest version is not updated. before: %s, after: %s",
			before, after,
		)
	}

	fmt.Println("Example Package.SetLatestVer: OK")
	// Output: Example Package.SetLatestVer: OK
}
