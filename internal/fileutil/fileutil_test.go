package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFile(t *testing.T) {
	t.Parallel()

	t.Run("existing file", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "testfile")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if !IsFile(f.Name()) {
			t.Errorf("IsFile(%q) = false, want true", f.Name())
		}
	})

	t.Run("directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if IsFile(dir) {
			t.Errorf("IsFile(%q) = true, want false for directory", dir)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		t.Parallel()
		if IsFile("/non/existent/path") {
			t.Error("IsFile should return false for non-existent path")
		}
	})
}

func TestIsDir(t *testing.T) {
	t.Parallel()

	t.Run("existing directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if !IsDir(dir) {
			t.Errorf("IsDir(%q) = false, want true", dir)
		}
	})

	t.Run("file", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "testfile")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if IsDir(f.Name()) {
			t.Errorf("IsDir(%q) = true, want false for file", f.Name())
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		t.Parallel()
		if IsDir("/non/existent/path") {
			t.Error("IsDir should return false for non-existent path")
		}
	})
}

func TestIsHiddenFile(t *testing.T) {
	t.Parallel()

	t.Run("hidden file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		hidden := filepath.Join(dir, ".hidden")
		if err := os.WriteFile(hidden, []byte("test"), FileModeCreatingFile); err != nil {
			t.Fatal(err)
		}
		if !IsHiddenFile(hidden) {
			t.Errorf("IsHiddenFile(%q) = false, want true", hidden)
		}
	})

	t.Run("non-hidden file", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "visible")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if IsHiddenFile(f.Name()) {
			t.Errorf("IsHiddenFile(%q) = true, want false", f.Name())
		}
	})

	t.Run("non-existent hidden path", func(t *testing.T) {
		t.Parallel()
		if IsHiddenFile("/non/existent/.hidden") {
			t.Error("IsHiddenFile should return false for non-existent path")
		}
	})

	t.Run("directory with dot prefix", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dotDir := filepath.Join(dir, ".hiddendir")
		if err := os.Mkdir(dotDir, FileModeCreatingDir); err != nil {
			t.Fatal(err)
		}
		if IsHiddenFile(dotDir) {
			t.Errorf("IsHiddenFile(%q) = true, want false for directory", dotDir)
		}
	})
}

func TestResolveSymlinkTarget(t *testing.T) {
	if isWindows() {
		t.Skip("symlink behavior is POSIX-specific")
	}
	t.Parallel()

	t.Run("regular file returned unchanged", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "file")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveSymlinkTarget(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != path {
			t.Fatalf("ResolveSymlinkTarget(%q) = %q, want unchanged", path, got)
		}
	})

	t.Run("missing path returned unchanged", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "missing")
		got, err := ResolveSymlinkTarget(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != path {
			t.Fatalf("ResolveSymlinkTarget(%q) = %q, want unchanged", path, got)
		}
	})

	t.Run("symlink resolves to target", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveSymlinkTarget(link)
		if err != nil {
			t.Fatal(err)
		}
		if got != target {
			t.Fatalf("ResolveSymlinkTarget(%q) = %q, want %q", link, got, target)
		}
	})

	t.Run("dangling symlink resolves to missing target", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveSymlinkTarget(link)
		if err != nil {
			t.Fatal(err)
		}
		if got != target {
			t.Fatalf("ResolveSymlinkTarget(%q) = %q, want %q", link, got, target)
		}
	})

	t.Run("symlink cycle reports an error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a := filepath.Join(dir, "a")
		b := filepath.Join(dir, "b")
		if err := os.Symlink(a, b); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(b, a); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveSymlinkTarget(a); err == nil {
			t.Fatal("ResolveSymlinkTarget() should error on a symlink cycle")
		}
	})
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
