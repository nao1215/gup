package goutil_test

import (
	"fmt"
	"log"
	"os"
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
	pkgInfo := goutil.GetPackageInformation([]string{"../../cmd/testdata/check_success/gal"})
	if pkgInfo == nil {
		log.Fatal("example GetPackageInformation failed. The returned package information is nil")
	}

	want := []string{
		"gal",
		"github.com/nao1215/gal/cmd/gal",
		"github.com/nao1215/gal",
	}
	got := []string{
		pkgInfo[0].Name,
		pkgInfo[0].ImportPath,
		pkgInfo[0].ModulePath,
	}

	if cmp.Equal(got, want) {
		fmt.Println("Example GetPackageInformation: OK")
	} else {
		log.Fatalf("example GetPackageInformation failed. got: %v, want: %v", got, want)
	}
	// Output: Example GetPackageInformation: OK
}

func ExampleGetPackageVersion() {
	// GetPackageVersion returns the version of the package installed via `go install`.
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

func ExampleGoVersionWithOptionM() {
	// GoVersionWithOptionM returns the embedded module version information of
	// the executable. `gal` in this case.
	modInfo, err := goutil.GoVersionWithOptionM("../../cmd/testdata/check_success/gal")
	if err != nil {
		log.Fatal(err)
	}

	for _, info := range modInfo {
		expectContains := "github.com/nao1215/gal"
		if strings.Contains(info, expectContains) {
			fmt.Println("Example GoVersionWithOptionM: OK")

			break
		}
	}

	// Output: Example GoVersionWithOptionM: OK
}

func ExampleInstall() {
	// Install installs an executable from a Go package.
	err := goutil.Install("example.com/unknown_user/unknown_package")

	// If the package is not found or invalid, Install returns an error
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

func ExampleIsAlreadyUpToDate() {
	// Create Version object with Current and Latest package version
	ver := goutil.Version{
		Current: "v1.9.0",
		Latest:  "v1.9.1",
	}

	// Check if Current is already up to date (expected: false)
	if goutil.IsAlreadyUpToDate(ver) {
		fmt.Println("Example IsAlreadyUpToDate: already up to date.")
	} else {
		fmt.Println("Example IsAlreadyUpToDate: outdated. Newer latest version exists.")
	}

	// Output: Example IsAlreadyUpToDate: outdated. Newer latest version exists.
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
//  Type: Package
// ----------------------------------------------------------------------------

func ExamplePackage_CurrentToLatestStr() {
	// Set the paths of the target binary
	packages := goutil.GetPackageInformation([]string{"../../cmd/testdata/check_success/gal"})
	if len(packages) == 0 {
		log.Fatal("example GetPackageInformation failed. The returned package information is nil")
	}

	// test with the first package found
	pkgInfo := packages[0]

	wantContain := "Already up-to-date"
	got := pkgInfo.CurrentToLatestStr()

	if !strings.Contains(got, wantContain) {
		log.Fatalf(
			"example Package.CurrentToLatestStr failed. \nwant contain: %s\n got: %s",
			wantContain, got,
		)
	}

	fmt.Println("Example Package.CurrentToLatestStr: OK")
	// Output: Example Package.CurrentToLatestStr: OK
}

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

func ExamplePackage_VersionCheckResultStr() {
	packages := goutil.GetPackageInformation([]string{"../../cmd/testdata/check_success/gal"})
	if len(packages) == 0 {
		log.Fatal("example GetPackageInformation failed. The returned package information is nil")
	}

	// test with the first package found
	pkgInfo := packages[0]

	wantContain := "Already up-to-date"
	got := pkgInfo.VersionCheckResultStr()

	if !strings.Contains(got, wantContain) {
		log.Fatalf(
			"example Package.VersionCheckResultStr failed. \nwant contain: %s\n got: %s",
			wantContain, got,
		)
	}

	fmt.Println("Example Package.VersionCheckResultStr: OK")
	// Output: Example Package.VersionCheckResultStr: OK
}
