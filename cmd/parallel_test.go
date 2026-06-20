package cmd

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
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
