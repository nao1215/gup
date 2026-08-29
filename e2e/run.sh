#!/usr/bin/env bash
#
# run.sh is a thin wrapper kept for muscle memory and for anything that still
# calls it by path. The bootstrap itself moved to e2e/runner, a Go program: the
# end-to-end suite runs on Windows now, and a bash bootstrap would have made the
# Windows leg depend on Git for Windows being installed, which tests the runner
# image rather than gup.
#
# Usage: e2e/run.sh [atago args...]        (e.g. e2e/run.sh --filter update)
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

cd "$REPO_ROOT"
exec go run ./e2e/runner "$@"
