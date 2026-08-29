#!/usr/bin/env bash
#
# smoke_artifacts.sh validates GoReleaser output before it is published.
#
# A release is the only gup artifact most users ever run, and it is built by a
# pipeline nothing else exercises: the archives, the .deb/.rpm/.apk packages, the
# Scoop manifest and the winget manifest are all generated, and any of them can
# break without a single Go test failing. This script is the gate that runs
# before `goreleaser release` publishes anything.
#
# What it checks:
#   - the artifact inventory is exactly the supported matrix (linux/darwin/
#     windows x amd64/arm64) and nothing else -- an unexpected 386 build here
#     means .goreleaser.yml lost its explicit goarch list again,
#   - every archive contains the gup binary and all four completion files,
#   - every binary really is built for the OS and architecture its file name
#     claims (via `go version -m`, which reads that out of the binary itself),
#   - the archive matching THIS host extracts and runs: `gup --version`, `gup
#     list --json` and `gup check --json` produce the expected output and valid
#     JSON,
#   - the .deb installs and the installed gup runs, with its completions on disk,
#   - the .rpm and .apk carry the same file tree,
#   - the Scoop manifest's URLs, SHA-256 hashes and binary name match the zips
#     actually built,
#   - the winget manifest covers the same two architectures.
#
# Anything that cannot be checked on this host is recorded and printed in the
# closing scope report rather than passing silently -- a smoke test whose skips
# are invisible reports a green release it never inspected. The cross-OS legs
# that Ubuntu cannot execute (running the macOS and Windows binaries) are covered
# by the per-OS jobs in .github/workflows/release-smoke.yml and release.yml,
# which run this script on macOS and scripts/verify_artifact.ps1 on Windows.
#
# Usage: scripts/smoke_artifacts.sh [dist-dir]   (default: dist)
set -euo pipefail

DIST="${1:-dist}"

if [ ! -d "$DIST" ]; then
	echo "smoke: dist directory '$DIST' does not exist (run goreleaser first)" >&2
	exit 1
fi

# The supported distribution matrix, kept in step with `builds.goos`/`goarch` in
# .goreleaser.yml and with the "Supported OS" section of README.md.
SUPPORTED_OS=(linux darwin windows)
SUPPORTED_ARCH=(amd64 arm64)

CHECKED=()
SKIPPED=()

fail() {
	echo "smoke: FAIL: $*" >&2
	exit 1
}

note() { echo "smoke: $*"; }

checked() {
	CHECKED+=("$1")
	note "OK   $1"
}

skipped() {
	SKIPPED+=("$1 -- $2")
	note "SKIP $1 ($2)"
}

# ---------------------------------------------------------------------------
# Host identification. The host decides which artifact can actually be executed
# here; everything else is inspected without running it.
# ---------------------------------------------------------------------------
host_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) echo unknown ;;
	esac
}

host_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*) echo unknown ;;
	esac
}

HOST_OS="$(host_os)"
HOST_ARCH="$(host_arch)"
note "host: ${HOST_OS}/${HOST_ARCH}, dist: $DIST"

# artifact_for prints the single artifact matching os/arch/extension, or nothing.
# The version is part of every file name and is not known here, so the match is
# by suffix.
artifact_for() {
	local os="$1" arch="$2" ext="$3" match=""
	shopt -s nullglob
	for candidate in "$DIST"/*"_${os}_${arch}.${ext}"; do
		match="$candidate"
	done
	shopt -u nullglob
	echo "$match"
}

# ---------------------------------------------------------------------------
# 1) Inventory: the supported matrix is present, and nothing outside it is.
# ---------------------------------------------------------------------------
for os in "${SUPPORTED_OS[@]}"; do
	for arch in "${SUPPORTED_ARCH[@]}"; do
		ext="tar.gz"
		[ "$os" = "windows" ] && ext="zip"
		archive="$(artifact_for "$os" "$arch" "$ext")"
		[ -n "$archive" ] || fail "no ${os}/${arch} archive (*_${os}_${arch}.${ext}) in $DIST"
	done
done
checked "archive inventory: linux/darwin/windows x amd64/arm64"

# An artifact for an architecture gup does not claim to support means the build
# matrix went back to GoReleaser's defaults. 386 is the one that used to appear.
shopt -s nullglob
unexpected=()
for artifact in "$DIST"/*.tar.gz "$DIST"/*.zip "$DIST"/*.deb "$DIST"/*.rpm "$DIST"/*.apk; do
	name="$(basename "$artifact")"
	keep=""
	for arch in "${SUPPORTED_ARCH[@]}"; do
		case "$name" in
		*_"${arch}".*) keep=1 ;;
		esac
	done
	[ -n "$keep" ] || unexpected+=("$name")
done
shopt -u nullglob
if [ ${#unexpected[@]} -gt 0 ]; then
	fail "artifacts built for unsupported architectures: ${unexpected[*]}"
fi
checked "no artifacts outside the supported architectures"

for arch in "${SUPPORTED_ARCH[@]}"; do
	for ext in deb rpm apk; do
		package="$(artifact_for linux "$arch" "$ext")"
		[ -n "$package" ] || fail "no linux/${arch} .${ext} package in $DIST"
	done
done
checked "package inventory: .deb/.rpm/.apk for amd64 and arm64"

# ---------------------------------------------------------------------------
# 2) Archive contents: the binary plus every completion file gup ships.
# ---------------------------------------------------------------------------
check_archive_contents() {
	local archive="$1" listing
	case "$archive" in
	*.zip) listing="$(unzip -Z1 "$archive")" ;;
	*) listing="$(tar -tzf "$archive")" ;;
	esac
	echo "$listing" | grep -Eq '(^|/)gup(\.exe)?$' || fail "$archive is missing the gup binary"
	for shell in bash zsh fish ps1; do
		echo "$listing" | grep -q "completions/gup\.${shell}\$" ||
			fail "$archive is missing completions/gup.${shell}"
	done
}

shopt -s nullglob
archives=("$DIST"/*.tar.gz "$DIST"/*.zip)
shopt -u nullglob
for archive in "${archives[@]}"; do
	check_archive_contents "$archive"
done
checked "every archive carries the gup binary and the bash/zsh/fish/powershell completions"

# ---------------------------------------------------------------------------
# 3) Binary identity: each archive's binary is built for the OS and architecture
#    its name claims. A cross-compiled artifact cannot be executed here, but the
#    build settings recorded inside it can be read anywhere Go is installed --
#    which is what makes the macOS and arm64 artifacts verifiable from Ubuntu
#    rather than merely assumed.
# ---------------------------------------------------------------------------
extract_binary() {
	local archive="$1" dest="$2"
	mkdir -p "$dest"
	case "$archive" in
	*.zip) unzip -q -o "$archive" -d "$dest" ;;
	*) tar -xzf "$archive" -C "$dest" ;;
	esac
}

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

if command -v go >/dev/null 2>&1; then
	for os in "${SUPPORTED_OS[@]}"; do
		for arch in "${SUPPORTED_ARCH[@]}"; do
			ext="tar.gz"
			binary="gup"
			if [ "$os" = "windows" ]; then
				ext="zip"
				binary="gup.exe"
			fi
			archive="$(artifact_for "$os" "$arch" "$ext")"
			dest="$WORKDIR/identity/${os}_${arch}"
			extract_binary "$archive" "$dest"
			settings="$(go version -m "$dest/$binary")"
			echo "$settings" | grep -q "GOOS=${os}$" ||
				fail "$(basename "$archive") does not contain a ${os} binary"
			echo "$settings" | grep -q "GOARCH=${arch}$" ||
				fail "$(basename "$archive") does not contain a ${arch} binary"
		done
	done
	checked "every archive's binary reports the GOOS/GOARCH its file name claims"
else
	skipped "binary GOOS/GOARCH identity" "the go command is not installed"
fi

# ---------------------------------------------------------------------------
# 4) Execution: the archive built for THIS host must extract and work. Only the
#    native artifact can be run; the other five are covered by the per-OS jobs.
# ---------------------------------------------------------------------------
assert_json_array() {
	local label="$1" payload="$2"
	case "$(printf '%s' "$payload" | tr -d '[:space:]')" in
	'['*']') ;;
	*) fail "$label did not produce a JSON array: $payload" ;;
	esac
	if command -v jq >/dev/null 2>&1; then
		printf '%s' "$payload" | jq -e 'type == "array"' >/dev/null ||
			fail "$label produced invalid JSON: $payload"
	fi
}

run_cli_checks() {
	local gup="$1" label="$2" empty_gobin="$WORKDIR/empty-gobin"
	mkdir -p "$empty_gobin"

	local version_out
	version_out="$("$gup" --version)"
	echo "$version_out" | grep -q 'gup version' || fail "$label: --version output unexpected: $version_out"

	"$gup" --help | grep -q 'Available Commands' || fail "$label: --help did not list the subcommands"

	# The JSON modes are what scripts and CI consume, so a distributed binary that
	# cannot produce parseable JSON on an empty $GOBIN is broken for its main
	# non-interactive use. An empty $GOBIN keeps the check hermetic: no network,
	# and no dependence on what the runner happens to have installed.
	assert_json_array "$label: list --json" "$(GOBIN="$empty_gobin" "$gup" list --json)"
	assert_json_array "$label: check --json" "$(GOBIN="$empty_gobin" "$gup" check --json)"

	note "$label: $version_out"
}

if [ "$HOST_OS" != "unknown" ] && [ "$HOST_ARCH" != "unknown" ]; then
	host_archive="$(artifact_for "$HOST_OS" "$HOST_ARCH" tar.gz)"
	[ -n "$host_archive" ] || fail "no ${HOST_OS}/${HOST_ARCH} archive to execute"
	extract_binary "$host_archive" "$WORKDIR/native"
	[ -x "$WORKDIR/native/gup" ] || fail "the extracted gup binary is not executable"
	run_cli_checks "$WORKDIR/native/gup" "extracted ${HOST_OS}/${HOST_ARCH} binary"
	checked "the ${HOST_OS}/${HOST_ARCH} archive extracts and its gup runs (--version, --help, list/check --json)"
else
	skipped "native archive execution" "unrecognized host $(uname -s)/$(uname -m)"
fi

for os in "${SUPPORTED_OS[@]}"; do
	for arch in "${SUPPORTED_ARCH[@]}"; do
		if [ "$os" = "$HOST_OS" ] && [ "$arch" = "$HOST_ARCH" ]; then
			continue
		fi
		skipped "running the ${os}/${arch} binary" "cannot execute on ${HOST_OS}/${HOST_ARCH}; covered by the per-OS smoke jobs"
	done
done

# ---------------------------------------------------------------------------
# 5) Linux packages. The .deb is installed for real, because "the package
#    installs" is a claim only an install can support. The .rpm and .apk are
#    inspected: their file tree is what a user gets, and it is generated from the
#    same nfpms block, so a missing completion file shows up here.
# ---------------------------------------------------------------------------
package_paths_ok() {
	local listing="$1" label="$2"
	echo "$listing" | grep -Eq '(^|/)usr/bin/gup$' || fail "$label does not install /usr/bin/gup"
	echo "$listing" | grep -q 'bash-completion/completions/gup$' || fail "$label is missing the bash completion"
	echo "$listing" | grep -q 'fish/vendor_completions.d/gup.fish$' || fail "$label is missing the fish completion"
	echo "$listing" | grep -q 'zsh/vendor-completions/_gup$' || fail "$label is missing the zsh completion"
}

if [ "$HOST_OS" != "linux" ]; then
	# The Linux package formats are inspected on the Linux smoke job. macOS ships
	# bsdtar, which reads only the first member of an APK's concatenated gzip
	# streams, and has neither dpkg-deb nor rpm, so running these checks here
	# would need per-tool workarounds to produce an answer that is already known
	# on Linux.
	skipped "Linux package inspection (.deb/.rpm/.apk)" "not a Linux host; covered by the Linux smoke job"
else
	if command -v dpkg-deb >/dev/null 2>&1; then
		for arch in "${SUPPORTED_ARCH[@]}"; do
			deb="$(artifact_for linux "$arch" deb)"
			package_paths_ok "$(dpkg-deb -c "$deb" | awk '{print $NF}')" "$(basename "$deb")"
		done
		checked ".deb file tree for amd64 and arm64 (binary + bash/fish/zsh completions)"
	else
		skipped ".deb file tree" "dpkg-deb is not installed"
	fi

	# `sudo -n true` distinguishes a CI runner with passwordless sudo from a
	# developer's machine, where prompting for a password would hang the script.
	deb="$(artifact_for linux "$HOST_ARCH" deb)"
	if [ -n "$deb" ] && command -v dpkg >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
		note "installing $(basename "$deb")"
		sudo dpkg -i "$deb"
		run_cli_checks "$(command -v gup)" "installed .deb"
		[ -f /usr/share/bash-completion/completions/gup ] || fail ".deb did not install the bash completion"
		[ -f /usr/share/fish/vendor_completions.d/gup.fish ] || fail ".deb did not install the fish completion"
		[ -f /usr/share/zsh/vendor-completions/_gup ] || fail ".deb did not install the zsh completion"
		checked ".deb installs and the installed gup runs, with its completions on disk"
	else
		skipped ".deb install" "no ${HOST_ARCH} .deb, or no dpkg with passwordless sudo"
	fi

	if command -v rpm >/dev/null 2>&1; then
		for arch in "${SUPPORTED_ARCH[@]}"; do
			rpm_file="$(artifact_for linux "$arch" rpm)"
			package_paths_ok "$(rpm -qpl "$rpm_file")" "$(basename "$rpm_file")"
		done
		checked ".rpm file tree for amd64 and arm64 (binary + bash/fish/zsh completions)"
	else
		# Without rpm the payload cannot be listed, so the check degrades to the
		# format's magic number rather than being passed off as a full inspection.
		for arch in "${SUPPORTED_ARCH[@]}"; do
			rpm_file="$(artifact_for linux "$arch" rpm)"
			magic="$(head -c 4 "$rpm_file" | od -An -tx1 | tr -d ' \n')"
			[ "$magic" = "edabeedb" ] || fail "$(basename "$rpm_file") is not an RPM (magic $magic)"
		done
		skipped ".rpm file tree" "rpm is not installed; only the RPM magic number was verified"
	fi

	# An .apk is a tar archive, so GNU tar lists its file tree with no Alpine
	# tooling on the runner.
	for arch in "${SUPPORTED_ARCH[@]}"; do
		apk="$(artifact_for linux "$arch" apk)"
		listing="$(tar -tzf "$apk" 2>/dev/null || true)"
		[ -n "$listing" ] || fail "$(basename "$apk") could not be read as an APK archive"
		echo "$listing" | grep -q '^\.PKGINFO$' || fail "$(basename "$apk") has no .PKGINFO; it is not a valid APK"
		package_paths_ok "$listing" "$(basename "$apk")"
	done
	checked ".apk structure and file tree for amd64 and arm64 (.PKGINFO + binary + completions)"
fi

# ---------------------------------------------------------------------------
# 6) Windows package manifests. Both are generated from the same build matrix,
#    and both point at URLs that do not exist yet at snapshot time -- so what is
#    verified is the shape: the right architectures, file names that match the
#    zips actually built, and, for Scoop, hashes that match those zips byte for
#    byte. A wrong hash there is a manifest that fails on every user's machine
#    and cannot be fixed without a new release.
# ---------------------------------------------------------------------------
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

scoop_manifest="$DIST/scoop/bucket/gup.json"
if [ -f "$scoop_manifest" ]; then
	if command -v jq >/dev/null 2>&1; then
		jq -e . "$scoop_manifest" >/dev/null || fail "the Scoop manifest is not valid JSON"

		# Scoop names the 64-bit x86 architecture "64bit" and Arm "arm64".
		for pair in "64bit:amd64" "arm64:arm64"; do
			key="${pair%%:*}"
			arch="${pair##*:}"
			zip="$(artifact_for windows "$arch" zip)"

			url="$(jq -r --arg k "$key" '.architecture[$k].url' "$scoop_manifest")"
			[ "$url" != "null" ] || fail "the Scoop manifest has no $key architecture"
			case "$url" in
			*"$(basename "$zip")") ;;
			*) fail "the Scoop manifest's $key url ($url) does not point at $(basename "$zip")" ;;
			esac
			case "$url" in
			https://github.com/nao1215/gup/releases/download/*) ;;
			*) fail "the Scoop manifest's $key url is not a gup release download URL: $url" ;;
			esac

			hash="$(jq -r --arg k "$key" '.architecture[$k].hash' "$scoop_manifest")"
			want="$(sha256_of "$zip")"
			[ "$hash" = "$want" ] ||
				fail "the Scoop manifest's $key hash ($hash) does not match $(basename "$zip") ($want)"

			bin="$(jq -r --arg k "$key" '.architecture[$k].bin | join(",")' "$scoop_manifest")"
			[ "$bin" = "gup.exe" ] || fail "the Scoop manifest's $key bin is \"$bin\", want gup.exe"
		done

		[ "$(jq -r '.homepage' "$scoop_manifest")" = "https://github.com/nao1215/gup" ] ||
			fail "the Scoop manifest's homepage is not the gup repository"
		checked "Scoop manifest: valid JSON, amd64/arm64 release URLs, SHA-256 hashes matching the zips, gup.exe as the binary"
	else
		skipped "Scoop manifest contents" "jq is not installed"
	fi
else
	fail "no Scoop manifest at $scoop_manifest (the scoops block in .goreleaser.yml did not run)"
fi

shopt -s nullglob
winget_installers=("$DIST"/winget/manifests/*/*/*/*/*.installer.yaml)
shopt -u nullglob
if [ ${#winget_installers[@]} -gt 0 ]; then
	winget="${winget_installers[0]}"
	for want in x64 arm64; do
		grep -q "Architecture: ${want}\$" "$winget" || fail "the winget manifest has no ${want} installer"
	done
	for arch in "${SUPPORTED_ARCH[@]}"; do
		zip="$(artifact_for windows "$arch" zip)"
		grep -q "$(basename "$zip")" "$winget" || fail "the winget manifest does not reference $(basename "$zip")"
	done
	checked "winget manifest: x64 and arm64 installers referencing the built zips"
else
	skipped "winget manifest" "no installer manifest in $DIST/winget (publishing was skipped)"
fi

# ---------------------------------------------------------------------------
# Scope report. A smoke test that hides what it did not look at is worse than no
# smoke test, because it is believed.
# ---------------------------------------------------------------------------
echo
echo "smoke: ===== verification scope on ${HOST_OS}/${HOST_ARCH} ====="
for entry in "${CHECKED[@]}"; do
	echo "smoke:   verified: $entry"
done
for entry in "${SKIPPED[@]}"; do
	echo "smoke:   not verified here: $entry"
done
echo "smoke: ${#CHECKED[@]} checks verified, ${#SKIPPED[@]} not verifiable on this host"
echo "smoke: all artifact smoke checks passed"
