package diagnose

import (
	"errors"
	"strings"
	"testing"
)

func TestHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantSub  string // substring the hint must contain ("" means expect no hint)
		wantNone bool
	}{
		{
			name:     "nil error has no hint",
			err:      nil,
			wantNone: true,
		},
		{
			name: "module path mismatch names the new path",
			err: errors.New(`go: github.com/cosmtrek/air@latest: version constraints conflict:
	github.com/cosmtrek/air@v1.52.2: parsing go.mod:
	module declares its path as: github.com/air-verse/air
	        but was required as: github.com/cosmtrek/air`),
			wantSub: "github.com/air-verse/air",
		},
		{
			name:    "not installed by go install",
			err:     errors.New("foo is not installed by 'go install' (or permission incorrect)"),
			wantSub: "go install <importpath>@latest",
		},
		{
			name:    "devel binary",
			err:     errors.New("is devel-binary copied from local environment"),
			wantSub: "local checkout",
		},
		{
			name:    "go toolchain too old",
			err:     errors.New("can't install x:\ngo: module requires go >= 1.23 (running go 1.21.0)"),
			wantSub: "newer Go toolchain",
		},
		{
			name:    "build constraints exclude all go files",
			err:     errors.New("can't install x:\nbuild constraints exclude all Go files in /tmp/foo"),
			wantSub: "buildable for your platform",
		},
		{
			name:    "permission denied",
			err:     errors.New("can't install x:\nmkdir /usr/local/bin: permission denied"),
			wantSub: "Permission denied",
		},
		{
			name:    "no matching versions",
			err:     errors.New(`go: github.com/x@latest: no matching versions for query "latest"`),
			wantSub: "another channel",
		},
		{
			name:    "unknown revision",
			err:     errors.New("go: github.com/x@main: unknown revision main"),
			wantSub: "does not exist",
		},
		{
			name:    "unrecognized import path",
			err:     errors.New(`go: github.com/x@latest: unrecognized import path "github.com/x"`),
			wantSub: "could not be resolved",
		},
		{
			name:    "repository not found",
			err:     errors.New("remote: Repository not found.\nfatal: repository not found"),
			wantSub: "could not be resolved",
		},
		{
			name:    "network dial error",
			err:     errors.New("can't check x:\ndial tcp: lookup proxy.golang.org: no such host"),
			wantSub: "Network error",
		},
		{
			name:     "timeout already actionable, no hint",
			err:      errors.New("install of x timed out; run `go install x@latest` manually or raise --timeout (0 disables it)"),
			wantNone: true,
		},
		{
			name:     "canceled, no hint",
			err:      errors.New("install of x canceled: context canceled"),
			wantNone: true,
		},
		{
			name:     "unrecognized failure, no hint",
			err:      errors.New("some entirely unexpected failure"),
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Hint(tt.err)
			if tt.wantNone {
				if got != "" {
					t.Fatalf("Hint() = %q, want empty", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("Hint() = %q, want a hint containing %q", got, tt.wantSub)
			}
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("Hint() = %q, want substring %q", got, tt.wantSub)
			}
		})
	}
}
