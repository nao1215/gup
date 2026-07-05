package cmd

import (
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

// Bare flag-name constants shared by the per-flag error tests below (the
// command flags are registered by long name only). "file", "timeout" and
// "latest" reuse the production constants fileFlagName/timeoutFlagName/
// latestKeyword.
const (
	fnDryRun         = "dry-run"
	fnNotify         = "notify"
	fnJobs           = "jobs"
	fnIgnoreGoUpdate = "ignore-go-update"
	fnJSON           = "json"
	fnQuiet          = "quiet"
	fnExclude        = "exclude"
	fnMain           = "main"
	fnMaster         = "master"
	fnForce          = "force"
	fnOutput         = "output"
)

func TestParseUpdateFlags_defaults(t *testing.T) {
	t.Parallel()
	cmd := newUpdateCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseUpdateFlags(cmd)
	if err != nil {
		t.Fatalf("parseUpdateFlags() error = %v", err)
	}

	want := updateOpts{
		dryRun:         false,
		notify:         false,
		cpus:           clampJobs(runtime.NumCPU()),
		ignoreGoUpdate: false,
		jsonOut:        false,
		quiet:          false,
		timeout:        defaultGoOpTimeout,
		excludePkgList: []string{},
		mainPkgNames:   []string{},
		masterPkgNames: []string{},
		latestPkgNames: []string{},
		confFile:       "",
	}
	if diff := cmp.Diff(want, opts, cmp.AllowUnexported(updateOpts{})); diff != "" {
		t.Errorf("parseUpdateFlags() mismatch (-want +got):\n%s", diff)
	}
}

func TestParseUpdateFlags_values(t *testing.T) {
	t.Parallel()
	cmd := newUpdateCmd()
	args := []string{
		testFlagDryRun,
		testFlagNotify,
		testFlagJobs, "3",
		"--ignore-go-update",
		"--json",
		"--quiet",
		"--timeout", "5m",
		"--exclude", "foo,bar",
		"--main", "m1",
		"--master", "m2",
		"--latest", "l1",
		testFlagFile, "/tmp/gup.json",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseUpdateFlags(cmd)
	if err != nil {
		t.Fatalf("parseUpdateFlags() error = %v", err)
	}

	want := updateOpts{
		dryRun:         true,
		notify:         true,
		cpus:           3,
		ignoreGoUpdate: true,
		jsonOut:        true,
		quiet:          true,
		timeout:        5 * time.Minute,
		excludePkgList: []string{"foo", "bar"},
		mainPkgNames:   []string{"m1"},
		masterPkgNames: []string{"m2"},
		latestPkgNames: []string{"l1"},
		confFile:       "/tmp/gup.json",
	}
	if diff := cmp.Diff(want, opts, cmp.AllowUnexported(updateOpts{})); diff != "" {
		t.Errorf("parseUpdateFlags() mismatch (-want +got):\n%s", diff)
	}
}

// TestParseUpdateFlags_clampsJobs verifies a non-positive --jobs value is
// clamped to 1, matching the pre-refactor behavior in gup().
func TestParseUpdateFlags_clampsJobs(t *testing.T) {
	t.Parallel()
	cmd := newUpdateCmd()
	if err := cmd.ParseFlags([]string{testFlagJobs, "0"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseUpdateFlags(cmd)
	if err != nil {
		t.Fatalf("parseUpdateFlags() error = %v", err)
	}
	if opts.cpus != 1 {
		t.Errorf("parseUpdateFlags() cpus = %d, want 1", opts.cpus)
	}
}

// TestParseUpdateFlags_error verifies that a missing/unregistered flag surfaces
// as an error instead of panicking, so gup() can handle it once.
func TestParseUpdateFlags_error(t *testing.T) {
	t.Parallel()
	if _, err := parseUpdateFlags(&cobra.Command{}); err == nil {
		t.Error("parseUpdateFlags() error = nil, want error for command without flags")
	}
}

func TestParseCheckFlags_defaults(t *testing.T) {
	t.Parallel()
	cmd := newCheckCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseCheckFlags(cmd)
	if err != nil {
		t.Fatalf("parseCheckFlags() error = %v", err)
	}

	want := checkOpts{
		cpus:           clampJobs(runtime.NumCPU()),
		ignoreGoUpdate: false,
		jsonOut:        false,
		quiet:          false,
		timeout:        defaultGoOpTimeout,
		confFile:       "",
	}
	if diff := cmp.Diff(want, opts, cmp.AllowUnexported(checkOpts{})); diff != "" {
		t.Errorf("parseCheckFlags() mismatch (-want +got):\n%s", diff)
	}
}

func TestParseCheckFlags_values(t *testing.T) {
	t.Parallel()
	cmd := newCheckCmd()
	args := []string{
		testFlagJobs, "2",
		"--ignore-go-update",
		"--json",
		"--quiet",
		"--timeout", "90s",
		testFlagFile, "x.json",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseCheckFlags(cmd)
	if err != nil {
		t.Fatalf("parseCheckFlags() error = %v", err)
	}

	want := checkOpts{
		cpus:           2,
		ignoreGoUpdate: true,
		jsonOut:        true,
		quiet:          true,
		timeout:        90 * time.Second,
		confFile:       "x.json",
	}
	if diff := cmp.Diff(want, opts, cmp.AllowUnexported(checkOpts{})); diff != "" {
		t.Errorf("parseCheckFlags() mismatch (-want +got):\n%s", diff)
	}
}

func TestParseCheckFlags_clampsJobs(t *testing.T) {
	t.Parallel()
	cmd := newCheckCmd()
	if err := cmd.ParseFlags([]string{testFlagJobs, "-4"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseCheckFlags(cmd)
	if err != nil {
		t.Fatalf("parseCheckFlags() error = %v", err)
	}
	if opts.cpus != 1 {
		t.Errorf("parseCheckFlags() cpus = %d, want 1", opts.cpus)
	}
}

func TestParseCheckFlags_error(t *testing.T) {
	t.Parallel()
	if _, err := parseCheckFlags(&cobra.Command{}); err == nil {
		t.Error("parseCheckFlags() error = nil, want error for command without flags")
	}
}

// registerUpToUpdate registers the update command's flags, in the exact order
// parseUpdateFlags reads them, stopping just before stopAt. The resulting
// command therefore has every flag parseUpdateFlags consults before stopAt but
// is missing stopAt itself, so the corresponding getFlag* call fails and the
// matching error branch runs.
func registerUpToUpdate(stopAt string) *cobra.Command {
	cmd := &cobra.Command{}
	f := cmd.Flags()
	order := []struct {
		name string
		reg  func()
	}{
		{fnDryRun, func() { f.BoolP(fnDryRun, "n", false, "") }},
		{fnNotify, func() { f.BoolP(fnNotify, "N", false, "") }},
		{fnJobs, func() { f.IntP(fnJobs, "j", 1, "") }},
		{fnIgnoreGoUpdate, func() { f.Bool(fnIgnoreGoUpdate, false, "") }},
		{fnJSON, func() { f.Bool(fnJSON, false, "") }},
		{fnQuiet, func() { f.BoolP(fnQuiet, "q", false, "") }},
		{timeoutFlagName, func() { f.Duration(timeoutFlagName, 0, "") }},
		{fnExclude, func() { f.StringSliceP(fnExclude, "e", nil, "") }},
		{fnMain, func() { f.StringSliceP(fnMain, "m", nil, "") }},
		{fnMaster, func() { f.StringSlice(fnMaster, nil, "") }},
		{latestKeyword, func() { f.StringSlice(latestKeyword, nil, "") }},
		{fileFlagName, func() { f.StringP(fileFlagName, "f", "", "") }},
	}
	for _, o := range order {
		if o.name == stopAt {
			break
		}
		o.reg()
	}
	return cmd
}

// TestParseUpdateFlags_perFlagError drives each getFlag error branch in
// parseUpdateFlags by handing it a command missing exactly one flag.
func TestParseUpdateFlags_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		fnDryRun, fnNotify, fnJobs, fnIgnoreGoUpdate, fnJSON, fnQuiet,
		timeoutFlagName, fnExclude, fnMain, fnMaster, latestKeyword, fileFlagName,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := registerUpToUpdate(name)
			if _, err := parseUpdateFlags(cmd); err == nil {
				t.Errorf("parseUpdateFlags() missing %q: error = nil, want error", name)
			}
		})
	}
}

func registerUpToCheck(stopAt string) *cobra.Command {
	cmd := &cobra.Command{}
	f := cmd.Flags()
	order := []struct {
		name string
		reg  func()
	}{
		{fnJobs, func() { f.IntP(fnJobs, "j", 1, "") }},
		{fnIgnoreGoUpdate, func() { f.Bool(fnIgnoreGoUpdate, false, "") }},
		{fnJSON, func() { f.Bool(fnJSON, false, "") }},
		{fnQuiet, func() { f.BoolP(fnQuiet, "q", false, "") }},
		{timeoutFlagName, func() { f.Duration(timeoutFlagName, 0, "") }},
		{fileFlagName, func() { f.StringP(fileFlagName, "f", "", "") }},
	}
	for _, o := range order {
		if o.name == stopAt {
			break
		}
		o.reg()
	}
	return cmd
}

// TestParseCheckFlags_perFlagError drives each getFlag error branch in
// parseCheckFlags.
func TestParseCheckFlags_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{fnJobs, fnIgnoreGoUpdate, fnJSON, fnQuiet, timeoutFlagName, fileFlagName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := registerUpToCheck(name)
			if _, err := parseCheckFlags(cmd); err == nil {
				t.Errorf("parseCheckFlags() missing %q: error = nil, want error", name)
			}
		})
	}
}

// registerUpToMigrate registers the flags runMigrate reads, in order, stopping
// before stopAt.
func registerUpToMigrate(stopAt string) *cobra.Command {
	cmd := &cobra.Command{}
	f := cmd.Flags()
	order := []struct {
		name string
		reg  func()
	}{
		{fnDryRun, func() { f.BoolP(fnDryRun, "n", false, "") }},
		{fnNotify, func() { f.BoolP(fnNotify, "N", false, "") }},
		{fnJobs, func() { f.IntP(fnJobs, "j", 1, "") }},
		{fnForce, func() { f.Bool(fnForce, false, "") }},
		{timeoutFlagName, func() { f.Duration(timeoutFlagName, 0, "") }},
	}
	for _, o := range order {
		if o.name == stopAt {
			break
		}
		o.reg()
	}
	return cmd
}

// TestRunMigrate_perFlagError drives each getFlag error branch in runMigrate.
// ensureGoCommandAvailable runs first and succeeds because the go toolchain is
// on PATH under `go test`; the flag reads then fail one at a time.
func TestRunMigrate_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{fnDryRun, fnNotify, fnJobs, fnForce, timeoutFlagName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := registerUpToMigrate(name)
			p, buf := newTestPrinter()
			if got := runMigrate(p, cmd, []string{"before", "after"}); got != 1 {
				t.Errorf("runMigrate() missing %q: exit = %d, want 1", name, got)
			}
			if buf.Len() == 0 {
				t.Errorf("runMigrate() missing %q: expected an error message", name)
			}
		})
	}
}

// registerUpToImport registers the flags runImport reads, in order, stopping
// before stopAt.
func registerUpToImport(stopAt string) *cobra.Command {
	cmd := &cobra.Command{}
	f := cmd.Flags()
	order := []struct {
		name string
		reg  func()
	}{
		{fnDryRun, func() { f.BoolP(fnDryRun, "n", false, "") }},
		{fileFlagName, func() { f.StringP(fileFlagName, "f", "", "") }},
		{fnNotify, func() { f.BoolP(fnNotify, "N", false, "") }},
		{fnJobs, func() { f.IntP(fnJobs, "j", 1, "") }},
		{timeoutFlagName, func() { f.Duration(timeoutFlagName, 0, "") }},
	}
	for _, o := range order {
		if o.name == stopAt {
			break
		}
		o.reg()
	}
	return cmd
}

// TestRunImport_perFlagError drives the getFlag error branches in runImport.
func TestRunImport_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{fnDryRun, fileFlagName, fnNotify, fnJobs, timeoutFlagName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := registerUpToImport(name)
			p, buf := newTestPrinter()
			if got := runImport(p, cmd, nil); got != 1 {
				t.Errorf("runImport() missing %q: exit = %d, want 1", name, got)
			}
			if buf.Len() == 0 {
				t.Errorf("runImport() missing %q: expected an error message", name)
			}
		})
	}
}

// TestList_perFlagError drives the getFlag error branches in list. list first
// reads local build info (which succeeds), then the json and file flags.
func TestList_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{fnJSON, fileFlagName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{}
			f := cmd.Flags()
			if name != fnJSON {
				f.Bool(fnJSON, false, "")
			}
			p, buf := newTestPrinter()
			if got := list(p, cmd, nil); got != 1 {
				t.Errorf("list() missing %q: exit = %d, want 1", name, got)
			}
			if buf.Len() == 0 {
				t.Errorf("list() missing %q: expected an error message", name)
			}
		})
	}
}

// TestExport_perFlagError drives the getFlag error branches in export.
func TestExport_perFlagError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{fnOutput, fileFlagName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{}
			f := cmd.Flags()
			if name != fnOutput {
				f.BoolP(fnOutput, "o", false, "")
			}
			p, buf := newTestPrinter()
			if got := export(p, cmd, nil); got != 1 {
				t.Errorf("export() missing %q: exit = %d, want 1", name, got)
			}
			if buf.Len() == 0 {
				t.Errorf("export() missing %q: expected an error message", name)
			}
		})
	}
}

// TestRunPin_fileFlagError drives the getFlagString(fileFlagName) error branch in
// runPin. parsePinArgs succeeds on valid args, then the missing file flag fails.
func TestRunPin_fileFlagError(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{} // no --file flag registered
	p, buf := newTestPrinter()
	if got := runPin(p, cmd, []string{testBinTool, testVersionOne}); got != 1 {
		t.Errorf("runPin() exit = %d, want 1", got)
	}
	if buf.Len() == 0 {
		t.Error("runPin() expected an error message")
	}
}

// TestRunUnpin_fileFlagError drives the getFlagString(fileFlagName) error branch in
// runUnpin.
func TestRunUnpin_fileFlagError(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{} // no --file flag registered
	p, buf := newTestPrinter()
	if got := runUnpin(p, cmd, []string{testBinTool}); got != 1 {
		t.Errorf("runUnpin() exit = %d, want 1", got)
	}
	if buf.Len() == 0 {
		t.Error("runUnpin() expected an error message")
	}
}
