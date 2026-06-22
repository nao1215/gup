#!/usr/bin/env bash
#
# smoke_artifacts.sh validates GoReleaser output before it is published.
#
# It checks that:
#   - the host (linux/amd64) archive extracts and `gup --version` runs,
#   - every archive contains the gup binary and the shell completion files,
#   - the Linux .deb installs and the installed gup runs, with its bash
#     completion present.
#
# Usage: scripts/smoke_artifacts.sh [dist-dir]   (default: dist)
set -euo pipefail

DIST="${1:-dist}"

if [ ! -d "$DIST" ]; then
	echo "smoke: dist directory '$DIST' does not exist (run goreleaser first)" >&2
	exit 1
fi

fail() {
	echo "smoke: FAIL: $*" >&2
	exit 1
}

note() { echo "smoke: $*"; }

# Collect archives.
shopt -s nullglob
tarballs=("$DIST"/*.tar.gz)
zips=("$DIST"/*.zip)
shopt -u nullglob

if [ ${#tarballs[@]} -eq 0 ] && [ ${#zips[@]} -eq 0 ]; then
	fail "no archives (*.tar.gz / *.zip) found in $DIST"
fi

# 1) Every archive must contain the gup binary and the completion files.
check_archive_contents() {
	local archive="$1" listing
	case "$archive" in
	*.zip) listing="$(unzip -Z1 "$archive")" ;;
	*) listing="$(tar -tzf "$archive")" ;;
	esac
	echo "$listing" | grep -Eq '(^|/)gup(\.exe)?$' || fail "$archive is missing the gup binary"
	echo "$listing" | grep -q 'completions/gup\.bash' || fail "$archive is missing completions/gup.bash"
	echo "$listing" | grep -q 'completions/gup\.zsh' || fail "$archive is missing completions/gup.zsh"
	echo "$listing" | grep -q 'completions/gup\.fish' || fail "$archive is missing completions/gup.fish"
	note "archive contents OK: $(basename "$archive")"
}

for a in "${tarballs[@]}" "${zips[@]}"; do
	check_archive_contents "$a"
done

# 2) The host (linux/amd64) archive must extract and run.
host_archive=""
for a in "${tarballs[@]}"; do
	case "$a" in
	*_linux_amd64.tar.gz) host_archive="$a" ;;
	esac
done
if [ -z "$host_archive" ]; then
	fail "no linux/amd64 archive found to execute"
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
tar -xzf "$host_archive" -C "$workdir"
[ -x "$workdir/gup" ] || fail "extracted gup binary is not executable"
version_out="$("$workdir/gup" --version)"
echo "$version_out" | grep -q 'gup version' || fail "gup --version output unexpected: $version_out"
note "extracted binary runs: $version_out"

# 3) The Linux .deb (amd64) must install and run, with bash completion present.
shopt -s nullglob
debs=("$DIST"/*_linux_amd64.deb)
shopt -u nullglob
if [ ${#debs[@]} -gt 0 ] && command -v dpkg >/dev/null 2>&1; then
	deb="${debs[0]}"
	note "installing $(basename "$deb")"
	sudo dpkg -i "$deb"
	pkg_version="$(gup --version)"
	echo "$pkg_version" | grep -q 'gup version' || fail "installed gup --version unexpected: $pkg_version"
	[ -f /usr/share/bash-completion/completions/gup ] || fail ".deb did not install the bash completion"
	note "deb install OK: $pkg_version"
else
	note "skipping .deb install check (no amd64 .deb or dpkg unavailable)"
fi

note "all artifact smoke checks passed"
