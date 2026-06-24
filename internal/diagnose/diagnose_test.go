package diagnose

import (
	"errors"
	"strings"
	"testing"
)

// subResolved is the distinctive fragment of the repository-resolution hint,
// shared by several cases that funnel to it.
const subResolved = "could not be resolved"

// subNetwork is the distinctive fragment of the network-error hint, shared by
// the several real proxy/transport wordings that funnel to it.
const subNetwork = "Network error"

type hintCase struct {
	name     string
	err      error
	wantSub  string // substring the hint must contain ("" means expect no hint)
	wantNone bool
}

// hintTestCases is the single source of truth for the diagnose fixtures. The
// error strings are real Go toolchain / git / net output (see the per-case
// comments), so they double as a regression anchor: the toolchain's English
// wording is an external contract a Go release can change, and pinning the real
// strings here turns such a drift into a visible test failure
// (see TestEveryMatcherFiresOnARealFixture).
func hintTestCases() []hintCase {
	return []hintCase{
		{
			name:     "nil error has no hint",
			err:      nil,
			wantNone: true,
		},
		{
			// Verified against real output of:
			//   go install github.com/cosmtrek/air@latest
			name: "module path mismatch names the new path",
			err: errors.New(`go: github.com/cosmtrek/air@latest: version constraints conflict:
	github.com/cosmtrek/air@v1.65.3: parsing go.mod:
	module declares its path as: github.com/air-verse/air
	        but was required as: github.com/cosmtrek/air`),
			wantSub: "github.com/air-verse/air",
		},
		{
			// Verified against real output of:
			//   go install github.com/golang-migrate/migrate/cmd/migrate@latest
			// (the tool moved to a /v4 module path, so its old v1 import path is
			// gone). This is the realistic "v2+ appeared" failure.
			name:    "command path gone after major version bump",
			err:     errors.New("go: github.com/golang-migrate/migrate/cmd/migrate@latest: module github.com/golang-migrate/migrate@latest found (v3.5.4+incompatible), but does not contain package github.com/golang-migrate/migrate/cmd/migrate"),
			wantSub: "new major version",
		},
		{
			// Verified against real output of building a module-less import path,
			// e.g. `go install example.com/no/such/pkg@latest` in module mode:
			//   go: example.com/no/such/pkg@latest: no required module provides
			//   package example.com/no/such/pkg; to add it: ...
			name:    "no required module provides package",
			err:     errors.New("go: example.com/no/such/pkg@latest: no required module provides package example.com/no/such/pkg; to add it:\n\tgo get example.com/no/such/pkg"),
			wantSub: "new major version",
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
			// Verified against real output of `go install .` from a directory that
			// is not a main package / has no install location: the go command names
			// the synthetic "command-line-arguments" pseudo-package.
			name:    "command-line-arguments has no install location",
			err:     errors.New("go: no install location for directory /tmp/x outside GOPATH\n\tFor more details see: 'go help gopath'\ncommand-line-arguments"),
			wantSub: "local checkout",
		},
		{
			// Verified against real output of:
			//   go install github.com/wagoodman/dive@v0.9.0
			name: "go.mod replace directives",
			err: errors.New(`go: github.com/wagoodman/dive@v0.9.0 (in github.com/wagoodman/dive@v0.9.0):
	The go.mod file for the module providing named packages contains one or
	more replace directives. It must not contain directives that would cause
	it to be interpreted differently than if it were the main module.`),
			wantSub: "`replace` directives",
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
			// An SSH git auth failure also says "permission denied", but it is an
			// access problem, not a local write-permission one, so it must get the
			// repository/credentials hint instead.
			name:    "ssh auth failure is not a write-permission error",
			err:     errors.New("go: github.com/x@latest: git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository."),
			wantSub: subResolved,
		},
		{
			// Verified against real output of a private/HTTPS repo with no
			// credentials in a non-interactive shell:
			//   fatal: could not read Username for 'https://github.com': terminal
			//   prompts disabled
			name:    "terminal prompts disabled is an access problem",
			err:     errors.New("go: github.com/x/private@latest: git ls-remote -q origin in /cache: exit status 128:\n\tfatal: could not read Username for 'https://github.com': terminal prompts disabled"),
			wantSub: subResolved,
		},
		{
			name:    "no matching versions",
			err:     errors.New(`go: github.com/x@latest: no matching versions for query "latest"`),
			wantSub: "another channel",
		},
		{
			// Verified against real output of:
			//   go list -m github.com/nao1215/gup@v999.0.0
			name:    "invalid version",
			err:     errors.New("go: github.com/nao1215/gup@v999.0.0: invalid version: unknown revision v999.0.0"),
			wantSub: "does not exist",
		},
		{
			// Verified against real output of:
			//   go list -m example.com/nope/nope@latest
			name:    "unrecognized import path",
			err:     errors.New(`go: example.com/nope/nope@latest: unrecognized import path "example.com/nope/nope": reading https://example.com/nope/nope?go-get=1: 404 Not Found`),
			wantSub: subResolved,
		},
		{
			// A module proxy returns 410 Gone for a path it once served but no
			// longer does (e.g. a retracted/removed module).
			name:    "proxy 410 gone",
			err:     errors.New("go: github.com/x/gone@latest: reading https://proxy.golang.org/github.com/x/gone/@latest: 410 Gone"),
			wantSub: subResolved,
		},
		{
			// Verified against real output of:
			//   go list -m github.com/nao1215/<deleted>@latest  (direct git fallback)
			name:    "deleted or private repository",
			err:     errors.New("go: module github.com/nao1215/nope: git ls-remote -q https://github.com/nao1215/nope in /cache: exit status 128:\n\tremote: Repository not found.\n\tfatal: repository 'https://github.com/nao1215/nope/' not found"),
			wantSub: subResolved,
		},
		{
			name:    "network dial error",
			err:     errors.New("can't check x:\ndial tcp: lookup proxy.golang.org: no such host"),
			wantSub: subNetwork,
		},
		{
			// Real net wording: a TCP connect that times out.
			name:    "network i/o timeout",
			err:     errors.New("can't check x:\ndial tcp 142.250.72.17:443: i/o timeout"),
			wantSub: subNetwork,
		},
		{
			// Real net wording: the proxy host actively refuses the connection.
			name:    "network connection refused",
			err:     errors.New("can't check x:\ndial tcp 127.0.0.1:443: connect: connection refused"),
			wantSub: subNetwork,
		},
		{
			// Real net/http wording: TLS negotiation stalls.
			name:    "network tls handshake timeout",
			err:     errors.New("can't check x:\nGet \"https://proxy.golang.org/...\": net/http: TLS handshake timeout"),
			wantSub: subNetwork,
		},
		{
			// Real net wording when GOPROXY points at an unreachable HTTP proxy.
			// (Kept free of "timed out" wording, which Hint treats as an
			// already-actionable timeout and intentionally leaves unhinted.)
			name:    "network proxyconnect",
			err:     errors.New("can't check x:\nGet \"https://proxy.golang.org/...\": proxyconnect tcp: dial tcp 10.0.0.1:8080: connect: connection refused"),
			wantSub: subNetwork,
		},
		{
			// Real net wording: no route to the proxy host.
			name:    "network unreachable",
			err:     errors.New("can't check x:\ndial tcp 142.250.72.17:443: connect: network is unreachable"),
			wantSub: subNetwork,
		},
		{
			// Real git wording surfaced through the go command's direct fallback.
			name:    "network could not connect (git)",
			err:     errors.New("go: github.com/x@latest: git ls-remote -q origin in /cache: exit status 128:\n\tfatal: unable to access 'https://github.com/x/': Could not connect to server"),
			wantSub: subNetwork,
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
			// context.DeadlineExceeded renders as "context deadline exceeded"; that
			// path already carries its own remedy, so no hint is added.
			name:     "deadline exceeded, no hint",
			err:      errors.New("version check of x canceled: context deadline exceeded"),
			wantNone: true,
		},
		{
			name:     "unrecognized failure, no hint",
			err:      errors.New("some entirely unexpected failure"),
			wantNone: true,
		},
	}
}

func TestHint(t *testing.T) {
	t.Parallel()

	for _, tt := range hintTestCases() {
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

// TestEveryMatcherFiresOnARealFixture guarantees every matcher's hint is
// produced by at least one real-message fixture in hintTestCases(). This is the
// regression anchor for toolchain-wording drift: if a Go release reworks the
// message a matcher keys on, that matcher's fixture stops firing, its hint
// disappears from the produced set, and this test fails — instead of the matcher
// silently going dead in production. It also fails if a new matcher is added
// without a backing real-message fixture.
func TestEveryMatcherFiresOnARealFixture(t *testing.T) {
	t.Parallel()

	produced := make(map[string]bool)
	for _, tt := range hintTestCases() {
		if h := Hint(tt.err); h != "" {
			produced[h] = true
		}
	}

	for i, m := range matchers {
		if !produced[m.hint] {
			t.Errorf("matchers[%d] hint is never produced by any fixture in hintTestCases(); "+
				"add a fixture with the current real Go/git wording so a future change is caught.\nhint: %q",
				i, m.hint)
		}
	}
}

// TestMatcherNeedlesAreLowercase guards the invariant that needles are matched
// against the lower-cased error text, so an upper-case needle would be dead code
// that never fires.
func TestMatcherNeedlesAreLowercase(t *testing.T) {
	t.Parallel()

	for i, m := range matchers {
		for _, n := range m.needles {
			if n != strings.ToLower(n) {
				t.Errorf("matchers[%d] needle %q is not lower-case; it can never match the lower-cased error text", i, n)
			}
		}
	}
}
