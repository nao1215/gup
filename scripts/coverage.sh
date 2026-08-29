#!/usr/bin/env bash
#
# coverage.sh combines unit-test coverage with self-hosted E2E coverage into a
# single cover.out. Unit tests report line coverage, but they never exercise the
# real gup binary the way an end user does; the atago-driven E2E specs do. Go
# 1.20+ lets us instrument a built binary (`go build -cover`) and collect its
# runtime coverage via GOCOVERDIR, so we can merge "what the unit tests cover"
# with "what a real CLI run covers" and get one honest number.
#
# This is intentionally a separate, heavier target: `make test` / `make e2e`
# stay fast. Everything lands under .coverage/ (gitignored) except the final
# cover.out / cover.html, which are the same artifacts `make test` already
# produces so octocov and local tooling need no changes.
set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

COV="$REPO_ROOT/.coverage"
rm -rf "$COV"
mkdir -p "$COV/unit" "$COV/e2e" "$COV/merged"

# 1. Unit-test coverage as raw covdata (GOCOVERDIR form) so it can be merged
#    with the E2E covdata below. -covermode=atomic must match the binary build.
echo ">> unit coverage -> $COV/unit"
go test -count=1 -cover -covermode=atomic -coverpkg=./... ./... \
	-args -test.gocoverdir="$COV/unit"

# 2. Self-hosted E2E via the existing runner, but with COVER=1 (which builds a
#    `go build -cover` gup) and GOCOVERDIR exported. atago passes GOCOVERDIR
#    through to every spec command, so each gup child writes its own covdata.
echo ">> e2e coverage -> $COV/e2e"
COVER=1 GOCOVERDIR="$COV/e2e" go run ./e2e/runner

# 3. Merge the raw covdata and render the combined text profile + reports.
echo ">> merging unit + e2e covdata -> cover.out"
go tool covdata merge -i="$COV/unit,$COV/e2e" -o="$COV/merged"
go tool covdata textfmt -i="$COV/merged" -o="$REPO_ROOT/cover.out"

go tool cover -func=cover.out | tail -n 1
go tool cover -html=cover.out -o cover.html
echo ">> wrote cover.out and cover.html (unit + e2e combined)"
