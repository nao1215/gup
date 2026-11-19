package cmd

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
)

// Package names reused across the parallel tests, extracted to constants to
// satisfy goconst (repeated string literals).
const (
	pkgFail = "fail"
	pkgSlow = "slow"
	pkgOK1  = "ok-1"
	pkgOK2  = "ok-2"
)

func TestCountFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		total int
		want  string
	}{
		{1, "[%1d/%1d]"},
		{9, "[%1d/%1d]"},
		{10, "[%2d/%2d]"},
		{100, "[%3d/%3d]"},
	}
	for _, c := range cases {
		if got := countFormat(c.total); got != c.want {
			t.Errorf("countFormat(%d) = %q, want %q", c.total, got, c.want)
		}
	}
}

// makePkgs builds n packages with deterministic, unique names.
func makePkgs(n int) []goutil.Package {
	pkgs := make([]goutil.Package, n)
	for i := range pkgs {
		pkgs[i] = goutil.Package{Name: "pkg-" + strconv.Itoa(i)}
	}
	return pkgs
}

// TestExecutePackages_TimeoutIsolation drives the full executePackages engine
// (the path used by update/check/import/migrate) with a non-zero timeout and a
// mix of outcomes, asserting the exit code is 1 (a package failed) while the
// successful packages are still reported. The existing tests never exercise
// executePackages with a non-zero timeout end-to-end.
func TestExecutePackages_TimeoutIsolation(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{
		{Name: pkgOK1},
		{Name: pkgOK2},
		{Name: pkgSlow},
	}

	p, _ := newTestPrinter()
	var succeeded int64
	code, results := executePackages(p, pkgs, 2, 20*time.Millisecond,
		func(ctx context.Context, p goutil.Package) updateResult {
			if p.Name == pkgSlow {
				<-ctx.Done()
				return updateResult{pkg: p, err: ctx.Err()}
			}
			atomic.AddInt64(&succeeded, 1)
			return updateResult{pkg: p, status: statusUpdated}
		},
		func(_ string, _ updateResult) {})

	if code != 1 {
		t.Errorf("executePackages exit code = %d, want 1 (one package timed out)", code)
	}
	if len(results) != len(pkgs) {
		t.Fatalf("received %d results, want %d", len(results), len(pkgs))
	}
	if got := atomic.LoadInt64(&succeeded); got != 2 {
		t.Errorf("succeeded packages = %d, want 2 (timeout isolated from the rest)", got)
	}

	var timedOut int
	for _, r := range results {
		if errors.Is(r.err, context.DeadlineExceeded) {
			timedOut++
		}
	}
	if timedOut != 1 {
		t.Errorf("timed-out packages = %d, want exactly 1", timedOut)
	}
}

// TestExecutePackages_StableInputOrder verifies the #365 contract: even when
// workers finish in the reverse of input order, executePackages returns results
// in the original input order, so the JSON records derived from them are
// deterministic across runs. The worker sleeps longer for earlier packages so
// completion order is inverted relative to input order.
func TestExecutePackages_StableInputOrder(t *testing.T) {
	t.Parallel()

	const total = 8
	pkgs := makePkgs(total)

	// Earlier input index -> longer sleep -> finishes later. Workers only read
	// this map (built before the run), so concurrent access is safe.
	sleepByName := make(map[string]time.Duration, total)
	for i, p := range pkgs {
		sleepByName[p.Name] = time.Duration(total-i) * 3 * time.Millisecond
	}

	p, _ := newTestPrinter()
	code, results := executePackages(p, pkgs, total, 0,
		func(_ context.Context, p goutil.Package) updateResult {
			time.Sleep(sleepByName[p.Name])
			return updateResult{pkg: p, status: statusUpdated}
		},
		nil)

	if code != 0 {
		t.Fatalf("executePackages exit code = %d, want 0", code)
	}
	if len(results) != total {
		t.Fatalf("received %d results, want %d", len(results), total)
	}
	for i, r := range results {
		if want := pkgs[i].Name; r.pkg.Name != want {
			t.Errorf("results[%d].pkg.Name = %q, want %q (input order)", i, r.pkg.Name, want)
		}
	}

	// The JSON records must follow the same input order.
	recs := resultsToJSONPackages(results)
	for i, rec := range recs {
		if want := pkgs[i].Name; rec.Name != want {
			t.Errorf("json record[%d].Name = %q, want %q (input order)", i, rec.Name, want)
		}
	}
}

// TestExecutePackages_StableInputOrderWithFailures verifies #365 holds when the
// results are a mix of successes and failures (the update/check --json case):
// failed and succeeded records stay interleaved in input order, not grouped by
// outcome or completion time.
func TestExecutePackages_StableInputOrderWithFailures(t *testing.T) {
	t.Parallel()

	const total = 10
	pkgs := makePkgs(total)

	sleepByName := make(map[string]time.Duration, total)
	for i, p := range pkgs {
		sleepByName[p.Name] = time.Duration(total-i) * 2 * time.Millisecond
	}

	// Even-indexed packages fail; odd-indexed succeed.
	failByName := make(map[string]bool, total)
	for i, p := range pkgs {
		failByName[p.Name] = i%2 == 0
	}

	p, _ := newTestPrinter()
	code, results := executePackages(p, pkgs, total, 0,
		func(_ context.Context, p goutil.Package) updateResult {
			time.Sleep(sleepByName[p.Name])
			if failByName[p.Name] {
				return updateResult{pkg: p, err: errors.New("boom"), status: statusError}
			}
			return updateResult{pkg: p, status: statusUpdated}
		},
		nil)

	if code != 1 {
		t.Fatalf("executePackages exit code = %d, want 1 (some failed)", code)
	}
	if len(results) != total {
		t.Fatalf("received %d results, want %d", len(results), total)
	}
	for i, r := range results {
		if want := pkgs[i].Name; r.pkg.Name != want {
			t.Fatalf("results[%d].pkg.Name = %q, want %q (input order preserved across mixed outcomes)", i, r.pkg.Name, want)
		}
		wantErr := i%2 == 0
		if (r.err != nil) != wantErr {
			t.Errorf("results[%d] err presence = %v, want %v", i, r.err != nil, wantErr)
		}
	}
}

// TestExecutePackages_streamsPrefixesAndReportsErrors covers the command-side
// output the wrapper owns: with a single worker (so completion order is
// deterministic), a failed package prints the shared "[i/n] <error>" line to
// STDERR and is NOT passed to onResult, while successful packages reach onResult
// with their "[i/n]" prefix and the exit code becomes 1.
func TestExecutePackages_streamsPrefixesAndReportsErrors(t *testing.T) {
	t.Parallel()

	p, buf := newTestPrinter()

	pkgs := []goutil.Package{{Name: "a"}, {Name: pkgFail}, {Name: "c"}}

	var prefixes []string
	code, results := executePackages(p, pkgs, 1, 0,
		func(_ context.Context, p goutil.Package) updateResult {
			if p.Name == pkgFail {
				return updateResult{pkg: p, err: errors.New("boom")}
			}
			return updateResult{pkg: p}
		},
		func(prefix string, _ updateResult) {
			prefixes = append(prefixes, prefix)
		})

	if code != 1 {
		t.Errorf("executePackages exit code = %d, want 1 when a package fails", code)
	}
	if len(results) != len(pkgs) {
		t.Fatalf("received %d results, want %d", len(results), len(pkgs))
	}
	// onResult sees only the two successful packages, with completion-ordered
	// prefixes; the failed one (position 2) is reported on STDERR instead.
	if got := strings.Join(prefixes, ","); got != "[1/3],[3/3]" {
		t.Errorf("onResult prefixes = %q, want %q", got, "[1/3],[3/3]")
	}
	if !strings.Contains(buf.String(), "[2/3] boom") {
		t.Errorf("STDERR = %q, want it to contain %q", buf.String(), "[2/3] boom")
	}
}
