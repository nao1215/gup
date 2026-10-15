package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOwnerReportsClosedFile(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeOwner(file, "update"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("writeOwner error = %v, want closed file error", err)
	}
}

func TestReleaseReportsCleanupFailureAndFreesRegistry(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.lock")
	lock, err := Acquire(t.Context(), path, "update")
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Release error = %v, want closed file error", err)
	}
	next, err := Acquire(t.Context(), path, "update")
	if err != nil {
		t.Fatalf("failed cleanup retained registry entry: %v", err)
	}
	if err := next.Release(); err != nil {
		t.Fatal(err)
	}
}
