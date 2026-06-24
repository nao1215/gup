package cmd

import (
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
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
		"--file", "/tmp/gup.json",
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
		"--file", "x.json",
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
