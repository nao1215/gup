#shellcheck shell=bash
# spec_helper.sh sets up the isolated, offline environment every e2e spec runs
# in. run.sh exports GUP_E2E_BIN (the built gup binary), GUP_E2E_PROXY (the local
# module proxy URL), and the shared Go caches before invoking shellspec.

# shellcheck disable=SC2034  # used by shellspec
spec_helper_precheck() {
  if [ -z "${GUP_E2E_BIN:-}" ] || [ -z "${GUP_E2E_PROXY:-}" ]; then
    abort "e2e specs must be run via 'make e2e' (or e2e/run.sh), not 'shellspec' directly."
  fi
}

spec_helper_loaded() { :; }
spec_helper_configure() { :; }

# e2e_setup creates a fresh isolated environment for a single example: a private
# HOME, XDG_CONFIG_HOME, GOBIN, GOPATH and working directory, plus the offline
# go toolchain settings. It guarantees the test never touches the developer's
# real ~/.config/gup or real $GOBIN, and never reaches the network.
e2e_setup() {
  E2E_WORK="$(mktemp -d "${GUP_E2E_TMP:-${TMPDIR:-/tmp}}/case.XXXXXX")"
  export HOME="$E2E_WORK/home"
  export XDG_CONFIG_HOME="$E2E_WORK/home/.config"
  export GOBIN="$E2E_WORK/gobin"
  export GOPATH="$E2E_WORK/gopath"
  export GOMODCACHE="${GUP_E2E_GOMODCACHE}"
  export GOCACHE="${GUP_E2E_GOCACHE}"

  # Fully offline go toolchain: only our local proxy, no checksum DB, no
  # automatic toolchain download.
  export GOPROXY="$GUP_E2E_PROXY"
  export GOSUMDB="off"
  export GOFLAGS="-mod=mod"
  export GOTOOLCHAIN="local"
  unset GOPRIVATE GONOSUMDB GONOPROXY GONOSUMCHECK 2>/dev/null || true

  mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$GOBIN" "$E2E_WORK/work"
  cd "$E2E_WORK/work" || return 1
}

# e2e_teardown removes the per-example working tree.
e2e_teardown() {
  if [ -n "${E2E_WORK:-}" ]; then
    chmod -R u+w "$E2E_WORK" >/dev/null 2>&1 || true
    rm -rf "$E2E_WORK"
  fi
}

# gup runs the built binary under test.
gup() { "$GUP_E2E_BIN" "$@"; }

# gup_no_tty runs gup with stdin connected to a pipe (not a character device),
# so gup's "stdin is not a TTY" detection fires deterministically regardless of
# how the suite is launched. (/dev/null is itself a character device, so it would
# read as a TTY; a pipe does not.)
gup_no_tty() { printf '' | "$GUP_E2E_BIN" "$@"; }

# install_fixture installs a module from the offline proxy into the isolated
# GOBIN using the real go toolchain (e.g. install_fixture gup.test/outdated@v1.0.0).
# Output is swallowed on success (the module cache is pre-warmed by run.sh, so no
# download progress is expected) and surfaced only on failure.
install_fixture() {
  local out
  if ! out="$(go install "$1" 2>&1)"; then
    echo "install_fixture failed for $1: $out" >&2
    return 1
  fi
}

# install_fixture_into installs a module into a specific directory (used to seed
# a source GOBIN for migrate). Usage: install_fixture_into <dir> <module@version>.
install_fixture_into() {
  local out
  if ! out="$(GOBIN="$1" go install "$2" 2>&1)"; then
    echo "install_fixture_into failed for $2 -> $1: $out" >&2
    return 1
  fi
}
