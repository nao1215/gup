#!/bin/sh
# Prepares the inputs of the himorime suites in bench/, with no network.
#
#   sh gen.sh proxy N DIR           a file GOPROXY of N synthetic modules
#   sh gen.sh install N V GOBIN     install version V of the N synthetic modules
#   sh gen.sh copies N GOBIN        N copies of the binaries in cmd/testdata
#   sh gen.sh link FROM GOBIN       GOBIN of hard links to the binaries in FROM
#   sh gen.sh verify N V GOBIN      fail unless the N binaries in GOBIN are version V
#
# install needs GOPROXY, GOMODCACHE and GOCACHE from the benchmark's env.
set -eu

here=$(cd "$(dirname "$0")" && pwd)

case "$1" in
proxy)
	go build -C "$here/.." -o "$3/../proxygen" ./bench/proxygen
	"$3/../proxygen" "$3" "$2"
	;;
install)
	mkdir -p "$4"
	i=0
	while [ "$i" -lt "$2" ]; do
		GOBIN="$4" go install "example.test/m$(printf '%04d' "$i")@$3"
		i=$((i + 1))
	done
	;;
copies)
	mkdir -p "$3"
	i=0
	while [ "$i" -lt "$2" ]; do
		case $((i % 3)) in
		0) src=gal ;;
		1) src=posixer ;;
		*) src=subaru ;;
		esac
		cp "$here/../cmd/testdata/check_success/$src" "$3/bin$(printf '%03d' "$i")"
		i=$((i + 1))
	done
	;;
link)
	# A GOBIN of hard links to the installed binaries: go install replaces a
	# binary by renaming a new file over it, so the originals stay intact.
	rm -rf "$3"
	mkdir -p "$3"
	for f in "$2"/*; do
		ln "$f" "$3/"
	done
	;;
verify)
	i=0
	while [ "$i" -lt "$2" ]; do
		m="m$(printf '%04d' "$i")"
		go version -m "$4/$m" | grep -q "mod[[:space:]]*example.test/$m[[:space:]]*$3" || {
			echo "gen.sh: $4/$m is not $3" >&2
			exit 1
		}
		i=$((i + 1))
	done
	;;
*)
	echo "gen.sh: unknown kind $1" >&2
	exit 2
	;;
esac
