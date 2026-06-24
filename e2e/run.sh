#!/usr/bin/env bash
#
# run.sh builds gup, starts a self-contained offline Go module proxy, and runs
# the ShellSpec end-to-end suite against the real CLI. Everything happens in a
# throwaway temp tree, so the developer's real $HOME, ~/.config/gup, and $GOBIN
# are never touched, and no network access is required.
#
# Usage: e2e/run.sh [shellspec args...]
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

if ! command -v shellspec >/dev/null 2>&1; then
	echo "e2e: shellspec is not installed. Install it from https://github.com/shellspec/shellspec" >&2
	echo "e2e: e.g. 'curl -fsSL https://git.io/shellspec | sh' or 'brew install shellspec'" >&2
	exit 127
fi

TMP="$(mktemp -d "${TMPDIR:-/tmp}/gup-e2e.XXXXXX")"
cleanup() {
	if [ -n "${PROXY_PID:-}" ]; then
		kill "$PROXY_PID" >/dev/null 2>&1 || true
		wait "$PROXY_PID" 2>/dev/null || true
	fi
	# The Go module cache is written read-only; make it removable.
	chmod -R u+w "$TMP" >/dev/null 2>&1 || true
	rm -rf "$TMP"
}
trap cleanup EXIT

mkdir -p "$TMP/bin"

echo "e2e: building gup and the test proxy..."
( cd "$REPO_ROOT" && go build -ldflags '-X github.com/nao1215/gup/internal/cmdinfo.Version=v0.0.0-e2e' -o "$TMP/bin/gup" . )
( cd "$REPO_ROOT" && go build -o "$TMP/bin/testproxy" ./e2e/testproxy )

echo "e2e: starting offline module proxy..."
"$TMP/bin/testproxy" -dir "$TMP/proxy" -url-file "$TMP/proxy.url" -addr 127.0.0.1:0 &
PROXY_PID=$!

# Wait for the proxy to report its URL.
for _ in $(seq 1 50); do
	[ -s "$TMP/proxy.url" ] && break
	sleep 0.1
done
if [ ! -s "$TMP/proxy.url" ]; then
	echo "e2e: test proxy did not start" >&2
	exit 1
fi

# Shared, read-only-ish caches across specs (downloads are deduplicated); each
# spec still gets its own HOME/GOBIN/working dir (see spec_helper.sh).
export GUP_E2E_BIN="$TMP/bin/gup"
export GUP_E2E_PROXY="$(cat "$TMP/proxy.url")"
export GUP_E2E_GOMODCACHE="$TMP/gomodcache"
export GUP_E2E_GOCACHE="$TMP/gocache"
export GUP_E2E_TMP="$TMP"

# Pre-warm the shared module cache so gup's child "go" calls during the measured
# examples never emit "downloading ..." progress to stderr. Branch/build-failure
# fixtures are warmed too (failures ignored) so only their zips are cached.
echo "e2e: warming module cache..."
(
	export HOME="$TMP/warm/home" GOBIN="$TMP/warm/gobin" GOPATH="$TMP/warm/gopath"
	export GOMODCACHE="$GUP_E2E_GOMODCACHE" GOCACHE="$GUP_E2E_GOCACHE"
	export GOPROXY="$GUP_E2E_PROXY" GOSUMDB=off GOFLAGS=-mod=mod GOTOOLCHAIN=local
	mkdir -p "$GOBIN"
	for m in \
		gup.test/uptodate@v1.0.0 \
		gup.test/outdated@v1.0.0 \
		gup.test/outdated@v1.1.0 \
		gup.test/pinnable@v1.0.0 \
		gup.test/pinnable@v1.1.0 \
		gup.test/maintool@main \
		gup.test/mastertool@master \
		gup.test/badmaintool@v1.0.0 \
		gup.test/badmaintool@main \
		gup.test/badmaintool@master \
		gup.test/moved/cmd/tool@v1.0.0 \
		gup.test/moved@v1.1.0 \
		gup.test/replaced@v1.0.0 \
		gup.test/replaced@v1.1.0; do
		go install "$m" >/dev/null 2>&1 || true
	done
)

echo "e2e: GOPROXY=$GUP_E2E_PROXY"
cd "$SCRIPT_DIR"
shellspec "$@"
