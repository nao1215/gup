#!/bin/sh
# scripts/bench_compare.sh
#
# Reproducible, offline benchmark comparing `gup update` against other tools
# that update binaries installed by `go install`. Real network is noisy, so
# synthetic modules are served from a local file GOPROXY; with trivial modules
# this isolates each tool's update orchestration (gup runs in parallel; the
# others are sequential). Real-world times are dominated by each binary's build,
# but gup's parallelism keeps it ahead.
#
# Tools compared:
#   - gup update            (this repo; parallel)
#   - go-global-update      (github.com/Gelio/go-global-update; sequential)
#   - go install <pkg> loop (the no-tool baseline; sequential)
#
# Usage:
#   sh scripts/bench_compare.sh                 # sizes "10 30 50", 5 runs
#   SIZES="30" RUNS=7 sh scripts/bench_compare.sh
set -eu

SIZES="${SIZES:-10 30 50}"
RUNS="${RUNS:-5}"

repo_root=$(cd "$(dirname "$0")/.." && pwd)
work="$(mktemp -d)"
proxy="$work/proxy"
gen="$work/gen"
gup="$work/gup"
gobin="$work/gobin"
tools="$work/tools"

cleanup() { chmod -R u+w "$work" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT

# go-global-update is installed with the normal network proxy before we switch
# GOPROXY to the local file proxy.
echo "installing go-global-update..."
GOBIN="$tools" go install github.com/Gelio/go-global-update@latest

export GOPROXY="file://$proxy"
export GOSUMDB=off GOFLAGS=-mod=mod GOTOOLCHAIN=local GOBIN="$gobin"
export PATH="$tools:$PATH"

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
	proxy, n := os.Args[1], must(strconv.Atoi(os.Args[2]))
	for i := 0; i < n; i++ {
		mod := fmt.Sprintf("example.test/m%04d", i)
		dir := filepath.Join(proxy, mod, "@v")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "list"), []byte("v1.0.0\nv1.0.1\n"), 0o644)
		gomod := "module " + mod + "\n\ngo 1.21\n"
		for _, v := range []string{"v1.0.0", "v1.0.1"} {
			_ = os.WriteFile(filepath.Join(dir, v+".info"), []byte(`{"Version":"`+v+`","Time":"2020-01-01T00:00:00Z"}`), 0o644)
			_ = os.WriteFile(filepath.Join(dir, v+".mod"), []byte(gomod), 0o644)
			f, _ := os.Create(filepath.Join(dir, v+".zip"))
			zw := zip.NewWriter(f)
			p := mod + "@" + v + "/"
			w, _ := zw.Create(p + "go.mod")
			_, _ = w.Write([]byte(gomod))
			w2, _ := zw.Create(p + "main.go")
			_, _ = w2.Write([]byte("package main\n\nfunc main() { _ = \"" + v + "\" }\n"))
			_ = zw.Close()
			_ = f.Close()
		}
	}
}

func must(n int, err error) int {
	if err != nil {
		panic(err)
	}
	return n
}
GOEOF
( cd "$work" && go mod init bench >/dev/null 2>&1 && go build -o "$gen" gen.go )
( cd "$repo_root" && go build -o "$gup" main.go )

median() { echo "$1" | tr ' ' '\n' | grep -v '^$' | sort -n | awk '{a[NR]=$0} END{print a[int((NR+1)/2)]}'; }

install_old() { # $1 = n
	chmod -R u+w "$gobin" 2>/dev/null || true
	rm -rf "$gobin"; mkdir -p "$gobin"
	i=0
	while [ "$i" -lt "$1" ]; do
		go install "example.test/m$(printf '%04d' "$i")@v1.0.0" 2>/dev/null
		i=$(( i + 1 ))
	done
}

bench() { # $1=n  $2=command...
	n="$1"; shift
	times=""; r=0
	while [ "$r" -lt "$RUNS" ]; do
		install_old "$n"
		start=$(date +%s%N)
		"$@" >/dev/null 2>&1 || true
		end=$(date +%s%N)
		times="$times $(( (end - start) / 1000000 ))"
		r=$(( r + 1 ))
	done
	median "$times"
}

install_loop() { # $1 = n  (sequential `go install @latest`)
	i=0
	while [ "$i" -lt "$1" ]; do
		go install "example.test/m$(printf '%04d' "$i")@latest" 2>/dev/null || true
		i=$(( i + 1 ))
	done
}

printf '%-6s %-12s %-20s %-18s\n' "N" "gup(ms)" "go-global-update(ms)" "go-install(ms)"
for n in $SIZES; do
	"$gen" "$proxy" "$n" >/dev/null
	g=$(bench "$n" "$gup" update)
	o=$(bench "$n" go-global-update)
	l=$(bench "$n" sh -c 'i=0; while [ "$i" -lt '"$n"' ]; do go install "example.test/m$(printf %04d "$i")@latest" 2>/dev/null || true; i=$((i+1)); done')
	printf '%-6s %-12s %-20s %-18s\n' "$n" "$g" "$o" "$l"
done
