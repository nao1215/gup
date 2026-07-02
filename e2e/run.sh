#!/usr/bin/env bash
#
# run.sh builds gup, starts a self-contained offline Go module proxy, and runs
# the atago end-to-end suite (e2e/atago/*.atago.yaml) against the real CLI.
# Everything happens in a throwaway temp tree, so the developer's real $HOME,
# ~/.config/gup, and $GOBIN are never touched, and no network access is
# required.
#
# The test DEFINITIONS are atago YAML — this script is only the environment
# bootstrap (a plain shell program, not a test framework). Each atago scenario
# builds its own isolated HOME/GOBIN/GOPATH inside its temp workdir via
# scenario-level `env:` + ${workdir}, and inherits GOPROXY and the shared
# module/build caches exported here.
#
# Usage: e2e/run.sh [atago args...]        (e.g. e2e/run.sh --filter update)
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

if ! command -v atago >/dev/null 2>&1; then
	echo "e2e: atago is not installed. Install it from https://github.com/nao1215/atago" >&2
	echo "e2e: e.g. 'go install github.com/nao1215/atago@latest' (CI uses nao1215/setup-atago)" >&2
	exit 127
fi

TMP="$(mktemp -d "${TMPDIR:-/tmp}/gup-e2e.XXXXXX")"
PROXY_PID=""
cleanup() {
	if [ -n "$PROXY_PID" ]; then
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
(cd "$REPO_ROOT" && go build -ldflags '-X github.com/nao1215/gup/internal/cmdinfo.Version=v0.0.0-e2e' -o "$TMP/bin/gup" .)
(cd "$REPO_ROOT" && go build -o "$TMP/bin/testproxy" ./e2e/testproxy)

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

# Shared, offline toolchain settings inherited by every scenario. Per-scenario
# HOME/GOBIN/GOPATH isolation comes from each spec's `env:` + ${workdir}.
export GOPROXY GOSUMDB GOFLAGS GOTOOLCHAIN GOMODCACHE GOCACHE
GOPROXY="$(cat "$TMP/proxy.url")"
GOSUMDB="off"
GOFLAGS="-mod=mod"
GOTOOLCHAIN="local"
GOMODCACHE="$TMP/gomodcache"
GOCACHE="$TMP/gocache"

# Pre-warm the shared module cache so gup's child "go" calls during the
# measured scenarios never emit "downloading ..." progress to stderr — that
# keeps strict stderr assertions deterministic. Branch/build-failure fixtures
# are warmed too (failures ignored) so only their zips are cached.
echo "e2e: warming module cache..."
(
	export HOME="$TMP/warm/home" GOBIN="$TMP/warm/gobin" GOPATH="$TMP/warm/gopath"
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

# Put the e2e-built gup first on PATH so the specs exercise that exact binary.
export PATH="$TMP/bin:$PATH"

echo "e2e: GOPROXY=$GOPROXY"
# Extra args (e.g. --filter X) go before the path so the flag parser sees them.
atago run "$@" "$SCRIPT_DIR/atago"
