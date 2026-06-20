#!/bin/sh
# scripts/bench_compare.sh
#
# Benchmarks `gup update` against other tools that update binaries installed by
# `go install`, using real modules so the numbers reflect what users actually
# experience. gup updates binaries in parallel; the alternatives are sequential.
#
# Tools compared:
#   - gup update            (this repo; parallel)
#   - go-global-update      (github.com/Gelio/go-global-update; sequential)
#   - go install <pkg> loop (the no-tool baseline; sequential)
#
# It installs older versions of a fixed set of small CLIs into a temp GOBIN,
# then times each tool upgrading them to the latest version (median of RUNS,
# warm Go module cache). Requires network access.
#
# Usage:
#   sh scripts/bench_compare.sh
#   RUNS=7 sh scripts/bench_compare.sh
set -eu

RUNS="${RUNS:-5}"

repo_root=$(cd "$(dirname "$0")/.." && pwd)
work="$(mktemp -d)"
gup="$work/gup"
gobin="$work/gobin"
tools="$work/tools"

cleanup() { chmod -R u+w "$work" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT

# Old versions to install; each has a newer release so every tool does real work.
OLD="\
github.com/nao1215/posixer@v0.1.0 \
github.com/nao1215/subaru@v1.0.0 \
github.com/nao1215/gal/cmd/gal@v1.1.1 \
github.com/nao1215/ubume/cmd/ubume@v1.5.0 \
github.com/sivchari/tenv/cmd/tenv@v1.0.0 \
github.com/Songmu/ghch/cmd/ghch@v0.10.0 \
github.com/nao1215/leadtime@v0.0.3 \
github.com/mattn/goveralls@v0.0.11 \
github.com/fatih/gomodifytags@v1.16.0"

echo "building gup and installing go-global-update..."
( cd "$repo_root" && go build -o "$gup" main.go )
GOBIN="$tools" go install github.com/Gelio/go-global-update@latest

export GOBIN="$gobin"
export PATH="$tools:$PATH"

# Warm the module and build caches so we measure the tools, not one-time downloads.
mkdir -p "$gobin"
for spec in $OLD; do go install "${spec%@*}@latest" >/dev/null 2>&1 || true; done

pin_old() {
	chmod -R u+w "$gobin" 2>/dev/null || true
	rm -rf "$gobin"; mkdir -p "$gobin"
	for spec in $OLD; do go install "$spec" >/dev/null 2>&1 || true; done
}

install_loop() {
	for spec in $OLD; do go install "${spec%@*}@latest" >/dev/null 2>&1 || true; done
}

median() { echo "$1" | tr ' ' '\n' | grep -v '^$' | sort -n | awk '{a[NR]=$0} END{print a[int((NR+1)/2)]}'; }

bench() { # $@ = command to run after pinning old versions
	times=""; r=0
	while [ "$r" -lt "$RUNS" ]; do
		pin_old
		start=$(date +%s%N)
		"$@" >/dev/null 2>&1 || true
		end=$(date +%s%N)
		times="$times $(( (end - start) / 1000000 ))"
		r=$(( r + 1 ))
	done
	median "$times"
}

n=$(echo $OLD | wc -w)
echo
echo "updating $n binaries to latest, median of $RUNS runs (ms):"
printf '  %-18s %s\n' "gup update"        "$(bench "$gup" update)"
printf '  %-18s %s\n' "go-global-update"  "$(bench go-global-update)"
printf '  %-18s %s\n' "go install loop"   "$(bench install_loop)"
