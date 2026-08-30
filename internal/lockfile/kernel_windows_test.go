//go:build windows

package lockfile

import (
	"strings"
	"testing"
)

// TestExtendedPath covers the rewriting openLockFile has to do for itself.
// os.OpenFile hands long paths to Win32 in the extended-length form; CreateFile,
// which openLockFile uses instead so it can refuse a reparse point, does not - so
// a lock file deep enough in a tree would fail to open for a reason that has
// nothing to do with locking.
func TestExtendedPath(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", maxWin32Path)
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "a short path is left exactly as it is",
			path: `C:\Users\you\go\bin\.gup.lock`,
			want: `C:\Users\you\go\bin\.gup.lock`,
		},
		{
			name: "a long path gets the extended-length prefix",
			path: `C:\` + long,
			want: `\\?\C:\` + long,
		},
		{
			name: "a long UNC path gets the UNC spelling of it",
			path: `\\server\share\` + long,
			want: `\\?\UNC\server\share\` + long,
		},
		{
			name: "a path that already has the prefix is untouched",
			path: `\\?\C:\` + long,
			want: `\\?\C:\` + long,
		},
		{
			name: "a device path is not mangled into a UNC path",
			path: `\\.\C:\` + long,
			want: `\\?\` + `\\.\C:\` + long,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extendedPath(tt.path); got != tt.want {
				t.Errorf("extendedPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
