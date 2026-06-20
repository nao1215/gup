package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

func TestForEachPackage(t *testing.T) {
	t.Parallel()

	t.Run("runs worker for each package", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		}

		ch := forEachPackage(context.Background(), pkgs, 2, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p}
		})

		got := map[string]bool{}
		for i := 0; i < len(pkgs); i++ {
			r := <-ch
			if r.err != nil {
				t.Errorf("unexpected error for %s: %v", r.pkg.Name, r.err)
			}
			got[r.pkg.Name] = true
		}

		for _, p := range pkgs {
			if !got[p.Name] {
				t.Errorf("missing result for package %s", p.Name)
			}
		}
	})

	t.Run("propagates worker errors", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "fail"}}
		wantErr := fmt.Errorf("test error")

		ch := forEachPackage(context.Background(), pkgs, 1, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p, err: wantErr}
		})

		r := <-ch
		if r.err == nil {
			t.Fatal("expected error, got nil")
		}
		if r.err.Error() != wantErr.Error() {
			t.Errorf("got error %q, want %q", r.err, wantErr)
		}
	})

	t.Run("returns error when context is canceled", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "x"}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		ch := forEachPackage(ctx, pkgs, 1, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p}
		})

		r := <-ch
		if r.err == nil {
			t.Fatal("expected error from canceled context, got nil")
		}
	})

	t.Run("applies a per-package timeout", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "slow"}}
		ch := forEachPackage(context.Background(), pkgs, 1, 10*time.Millisecond,
			func(ctx context.Context, p goutil.Package) updateResult {
				<-ctx.Done()
				return updateResult{pkg: p, err: ctx.Err()}
			})

		r := <-ch
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded from per-package timeout, got: %v", r.err)
		}
	})

	t.Run("no deadline when timeout is zero", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "a"}}
		ch := forEachPackage(context.Background(), pkgs, 1, 0,
			func(ctx context.Context, p goutil.Package) updateResult {
				if _, ok := ctx.Deadline(); ok {
					return updateResult{pkg: p, err: fmt.Errorf("unexpected deadline with timeout=0")}
				}
				return updateResult{pkg: p}
			})

		r := <-ch
		if r.err != nil {
			t.Fatalf("timeout=0 should not set a deadline: %v", r.err)
		}
	})
}

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

func TestCollectResults_AllSuccess(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	ch := make(chan updateResult, len(pkgs))
	for _, p := range pkgs {
		ch <- updateResult{pkg: p}
	}
	close(ch)

	var prefixes []string
	code := collectResults(ch, len(pkgs), func(prefix string, _ updateResult) {
		prefixes = append(prefixes, prefix)
	})

	if code != 0 {
		t.Errorf("collectResults() = %d, want 0", code)
	}
	if got := strings.Join(prefixes, ","); got != "[1/3],[2/3],[3/3]" {
		t.Errorf("prefixes = %q, want [1/3],[2/3],[3/3]", got)
	}
}

//nolint:paralleltest // captures the global print.Stderr
func TestCollectResults_WithError(t *testing.T) {
	orgStderr := print.Stderr
	var buf bytes.Buffer
	print.Stderr = &buf
	t.Cleanup(func() { print.Stderr = orgStderr })

	ch := make(chan updateResult, 2)
	ch <- updateResult{pkg: goutil.Package{Name: "ok"}}
	ch <- updateResult{pkg: goutil.Package{Name: "bad"}, err: errors.New("boom")}
	close(ch)

	onResultCalls := 0
	code := collectResults(ch, 2, func(_ string, _ updateResult) {
		onResultCalls++
	})

	if code != 1 {
		t.Errorf("collectResults() = %d, want 1 when a result has an error", code)
	}
	if onResultCalls != 1 {
		t.Errorf("onResult called %d times, want 1 (only the successful result)", onResultCalls)
	}
	if !strings.Contains(buf.String(), "[2/2] boom") {
		t.Errorf("error output = %q, want it to contain %q", buf.String(), "[2/2] boom")
	}
}
