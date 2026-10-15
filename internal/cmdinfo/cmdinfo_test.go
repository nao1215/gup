//nolint:paralleltest // tests change shared environment or package seams; fixture values are intentionally repeated
package cmdinfo

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	t.Run("default version", func(t *testing.T) {
		old := Version
		Version = ""
		defer func() { Version = old }()

		got := GetVersion()
		if !strings.Contains(got, Name) {
			t.Errorf("GetVersion() = %q, should contain command name %q", got, Name)
		}
	})

	t.Run("version set by ldflags", func(t *testing.T) {
		old := Version
		Version = "1.2.3"
		defer func() { Version = old }()

		got := GetVersion()
		if !strings.Contains(got, "1.2.3") {
			t.Errorf("GetVersion() = %q, should contain version 1.2.3", got)
		}
	})
}
