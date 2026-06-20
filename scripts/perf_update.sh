#!/bin/sh
# scripts/perf_update.sh
#
# Reproducible, offline, REAL-install performance harness for `gup update`
# (issue #271 follow-up). Real network is noisy, so this serves synthetic
# modules from a local file GOPROXY and measures `gup update` actually
# compiling and installing binaries — not just --dry-run.
#
# It builds a throwaway module generator, publishes N modules at v1.0.0 and
# v1.0.1 to a file proxy, installs v1.0.0 into a temp GOBIN, then times
# `gup update` (which upgrades every binary to v1.0.1) across -j values, plus a
# "noop" run where everything is already up to date (resolution only, no
# install). Each case reports the median of several runs.
#
# Usage:
#   sh scripts/perf_update.sh                 # sizes "3 30 150", jobs "1 4 8 16 0"
#   SIZES="30" JOBS="8 0" RUNS="7" sh scripts/perf_update.sh
#   (-j 0 means the gup default, i.e. NumCPU)
set -eu

SIZES="${SIZES:-3 30 150}"
JOBS="${JOBS:-1 4 8 16 0}"
RUNS="${RUNS:-5}"

repo_root=$(cd "$(dirname "$0")/.." && pwd)
work="$(mktemp -d)"
proxy="$work/proxy"
gen="$work/gen"
gup="$work/gup"
gobin="$work/gobin"

cleanup() { chmod -R u+w "$work" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT

export GOPROXY="file://$proxy"
export GOSUMDB=off
export GOFLAGS=-mod=mod
export GOTOOLCHAIN=local
export GOBIN="$gobin"

# --- build the synthetic module proxy generator -----------------------------
cat > "$work/gen.go" <<'GOEOF'
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	proxy, n := os.Args[1], mustAtoi(os.Args[2])
	for i := 0; i < n; i++ {
		mod := fmt.Sprintf("example.test/m%04d", i)
		dir := filepath.Join(proxy, mod, "@v")
		must(os.MkdirAll(dir, 0o755))
		must(os.WriteFile(filepath.Join(dir, "list"), []byte("v1.0.0\nv1.0.1\n"), 0o644))
		gomod := "module " + mod + "\n\ngo 1.21\n"
		for _, v := range []string{"v1.0.0", "v1.0.1"} {
			must(os.WriteFile(filepath.Join(dir, v+".info"), []byte(`{"Version":"`+v+`","Time":"2020-01-01T00:00:00Z"}`), 0o644))
			must(os.WriteFile(filepath.Join(dir, v+".mod"), []byte(gomod), 0o644))
			writeZip(filepath.Join(dir, v+".zip"), mod, v, gomod)
		}
	}
}

func writeZip(path, mod, v, gomod string) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	prefix := mod + "@" + v + "/"
	write(zw, prefix+"go.mod", gomod)
	write(zw, prefix+"main.go", "package main\n\nfunc main() { _ = \""+v+"\" }\n")
}

func write(zw *zip.Writer, name, body string) {
	w, err := zw.Create(name)
	must(err)
	_, err = w.Write([]byte(body))
	must(err)
}

func mustAtoi(s string) int { n, err := strconv.Atoi(s); must(err); return n }
func must(err error)        { if err != nil { panic(err) } }
GOEOF
( cd "$work" && go mod init perfgen >/dev/null 2>&1 && go build -o "$gen" gen.go )

echo "building gup..."
( cd "$repo_root" && go build -o "$gup" main.go )

install_baseline() { # $1 = n  (installs v1.0.0 of all modules into a clean GOBIN)
	chmod -R u+w "$gobin" 2>/dev/null || true
	rm -rf "$gobin"
	mkdir -p "$gobin"
	i=0
	while [ "$i" -lt "$1" ]; do
		go install "example.test/m$(printf '%04d' "$i")@v1.0.0" 2>/dev/null
		i=$(( i + 1 ))
	done
}

median() { # reads space-separated numbers on $1
	echo "$1" | tr ' ' '\n' | grep -v '^$' | sort -n | awk '{a[NR]=$0} END{print a[int((NR+1)/2)]}'
}

jobs_label() { [ "$1" = "0" ] && echo "default" || echo "$1"; }

for n in $SIZES; do
	echo
	echo "=== GOBIN with $n modules (each upgrades v1.0.0 -> v1.0.1) ==="
	"$gen" "$proxy" "$n" >/dev/null

	for j in $JOBS; do
		jflag=""
		[ "$j" = "0" ] || jflag="-j $j"

		# real install: re-create the v1.0.0 baseline before every timed run
		times=""
		r=0
		while [ "$r" -lt "$RUNS" ]; do
			install_baseline "$n"
			start=$(date +%s%N)
			# shellcheck disable=SC2086
			GOBIN="$gobin" "$gup" update $jflag >/dev/null 2>&1
			end=$(date +%s%N)
			times="$times $(( (end - start) / 1000000 ))"
			r=$(( r + 1 ))
		done
		printf 'update (real install) -j=%-7s median=%5sms  runs:%s\n' "$(jobs_label "$j")" "$(median "$times")" "$times"
	done

	# noop: everything already at v1.0.1 -> resolution only, no install
	# (install v1.0.1 once, then update is a pure resolve pass)
	chmod -R u+w "$gobin" 2>/dev/null || true
	rm -rf "$gobin"; mkdir -p "$gobin"
	i=0
	while [ "$i" -lt "$n" ]; do go install "example.test/m$(printf '%04d' "$i")@v1.0.1" 2>/dev/null; i=$(( i + 1 )); done
	GOBIN="$gobin" "$gup" update >/dev/null 2>&1 # warm
	times=""
	r=0
	while [ "$r" -lt "$RUNS" ]; do
		start=$(date +%s%N)
		GOBIN="$gobin" "$gup" update >/dev/null 2>&1
		end=$(date +%s%N)
		times="$times $(( (end - start) / 1000000 ))"
		r=$(( r + 1 ))
	done
	printf 'update (all up-to-date, no install)  median=%5sms  runs:%s\n' "$(median "$times")" "$times"
done
