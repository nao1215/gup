#!/bin/sh
# scripts/perf.sh
#
# Reproducible command-level performance harness for gup (see issue #271).
#
# It builds gup, populates a synthetic GOBIN with real Go binaries copied from
# cmd/testdata/check_success, and times `gup list` / `gup check` /
# `gup update --dry-run` over small, medium, and large binary sets, comparing
# cold (first) and warm (repeated) runs and a few -j values.
#
# `check` and `update` resolve the latest versions over the network, so their
# absolute timings depend on network conditions; treat them as relative,
# same-machine measurements. `list` is local-only and stable.
#
# Usage:
#   sh scripts/perf.sh                # default sizes 3 30 150, 10 runs each
#   RUNS=20 SIZES="3 50 200" sh scripts/perf.sh
#   CMDS="list" sh scripts/perf.sh    # restrict to specific commands
set -eu

RUNS="${RUNS:-10}"
SIZES="${SIZES:-3 30 150}"
CMDS="${CMDS:-list check update}"
JOBS="${JOBS:-1 4}"

repo_root=$(cd "$(dirname "$0")/.." && pwd)
fixtures="$repo_root/cmd/testdata/check_success"
bin="$(mktemp -d)/gup"
gobin="$(mktemp -d)"

cleanup() { rm -rf "$(dirname "$bin")" "$gobin"; }
trap cleanup EXIT

echo "building gup..."
( cd "$repo_root" && go build -o "$bin" main.go )

populate_gobin() {
	count="$1" # number of binaries to create
	rm -rf "$gobin"
	mkdir -p "$gobin"
	i=0
	while [ "$i" -lt "$count" ]; do
		case $(( i % 3 )) in
			0) src=gal ;;
			1) src=posixer ;;
			*) src=subaru ;;
		esac
		cp "$fixtures/$src" "$gobin/bin$(printf '%03d' "$i")"
		i=$(( i + 1 ))
	done
}

# time_run <label> <args...>
time_run() {
	label="$1"
	shift
	# warm once (page cache, module cache) before timing. Fail fast on a real
	# command error so we never report timings for failed invocations.
	GOBIN="$gobin" "$bin" "$@" >/dev/null 2>&1
	start=$(date +%s%N)
	r=0
	while [ "$r" -lt "$RUNS" ]; do
		GOBIN="$gobin" "$bin" "$@" >/dev/null 2>&1
		r=$(( r + 1 ))
	done
	end=$(date +%s%N)
	total_ms=$(( (end - start) / 1000000 ))
	avg_ms=$(( (end - start) / (RUNS * 1000000) ))
	printf '%-40s total=%5dms  avg=%4dms/run (%d runs)\n' "$label" "$total_ms" "$avg_ms" "$RUNS"
}

for n in $SIZES; do
	echo
	echo "=== GOBIN with $n binaries ==="
	populate_gobin "$n"
	for c in $CMDS; do
		case "$c" in
			list)
				time_run "list (n=$n)" list
				;;
			check)
				for j in $JOBS; do time_run "check -j$j (n=$n)" check -j "$j"; done
				;;
			update)
				for j in $JOBS; do time_run "update --dry-run -j$j (n=$n)" update --dry-run -j "$j"; done
				;;
		esac
	done
done
