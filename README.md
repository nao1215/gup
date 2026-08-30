<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-36-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![tested with atago](https://img.shields.io/badge/tested%20with-atago-7c3aed?logo=data:image/svg%2Bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI%2BPHBhdGggZmlsbD0iI2ZmZiIgZD0iTTMuNiA0LjIgMTEuOSAxMmwtOC4zIDcuOC0xLjktMi4yTDcuOSAxMiAxLjcgNi40eiIvPjxyZWN0IGZpbGw9IiNmZmYiIHg9IjEyLjYiIHk9IjE3LjIiIHdpZHRoPSI5LjciIGhlaWdodD0iMi44IiByeD0iMS40Ii8%2BPC9zdmc%2B&logoColor=white)](https://github.com/nao1215/atago)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)
[![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/nao1215/gup/total)](https://github.com/nao1215/gup/releases)

![sample](./doc/img/sample.gif)

gup updates and manages the global Go command-line tools in your `$GOBIN`. `go install` places each program in `$GOBIN` (`$GOPATH/bin`) but never updates it again, keeps no manifest of what it installed, and offers no way to hold a tool at a version you depend on. gup manages that tool set: it brings the whole set up to date in parallel, can `pin` selected tools to exact versions, and adds the management commands `go install` lacks: `list`/`check` what is installed, `remove` binaries, `export`/`import` the set to reproduce it on another machine, and `migrate` it to a new `$GOBIN`. Runs on Windows, macOS, and Linux.

Documentation: **https://nao1215.github.io/gup/**

## Supported OS (unit testing with GitHub Actions)

- Linux
- Mac (macOS 13 Ventura or newer, the range Go 1.27 supports)
- Windows

### Supported Go versions
Unit tests run on Go 1.25, 1.26, and 1.27 across Linux, macOS, and Windows, and a
separate job tracks the latest Go release so a toolchain that is not yet a
supported minor cannot break gup unnoticed. `go.mod` declares Go 1.25 as the
minimum, so building from source needs Go 1.25 or newer.

The prebuilt release binaries are built with the latest Go 1.27 patch release:
installing a package or archive gives you the current Go runtime even if you
never install Go yourself. Because Go 1.27 dropped support for macOS 12 and
earlier, the macOS release binaries require macOS 13 Ventura or newer; on an
older macOS, build gup from source with a Go version that still supports it.

## How to install
gup is packaged in homebrew-core, the winget community repository, its own Scoop bucket, the mise and aqua registries, nixpkgs, and the AUR, in addition to `go install` and the prebuilt packages on the release page.

### Use "go install"
If you do not have the Go development environment installed on your system, please install it from the [official website](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```
Building from source needs Go 1.25 or newer. On an older Go, install a prebuilt release binary or a package (see below) instead.

### Use homebrew
gup is in homebrew-core, so no tap is required:
```shell
brew install gup
```
The GoReleaser-built formula in `nao1215/tap` is still published as an alternative. It installs the prebuilt release binary instead of building from source:
```shell
brew install nao1215/tap/gup
```

### Use winget (Windows)
```shell
winget install --id nao1215.gup
```

### Use Scoop (Windows)
[Scoop](https://scoop.sh/) installs gup from this repository's own bucket:
```shell
scoop bucket add nao1215 https://github.com/nao1215/gup
scoop install nao1215/gup
```
The bucket manifest lives in [`bucket/`](./bucket) and is regenerated on every
release, so the Windows `amd64`/`arm64` archive URLs and their SHA-256 hashes
always match the artifacts on the release page.

### Use mise-en-place
```shell
mise use -g gup@latest
```

### Use nix (Nix profile)
```shell
nix profile install nixpkgs#gogup
```

### Use aqua
gup is registered in the [aqua](https://aquaproj.github.io/) standard registry. Add it to your `aqua.yaml`:
```shell
aqua g -i nao1215/gup
```

### Use the AUR (Arch Linux)
Two community-maintained packages are available: [`gup`](https://aur.archlinux.org/packages/gup) builds from source, [`gup-bin`](https://aur.archlinux.org/packages/gup-bin) installs the release binary.
```shell
paru -S gup      # or: yay -S gup
paru -S gup-bin  # prebuilt binary
```

### Install from Package or Binary
[The release page](https://github.com/nao1215/gup/releases) contains packages in .deb, .rpm, and .apk formats for `amd64` and `arm64`, plus `.tar.gz` archives for Linux/macOS and `.zip` archives for Windows. gup command uses the go command internally, so the golang installation is required.

Download the package that matches your distribution and architecture, then:
```shell
# Debian, Ubuntu
$ sudo dpkg -i gup_1.8.1_linux_amd64.deb

# Fedora, RHEL, openSUSE
$ sudo rpm -Uvh gup_1.8.1_linux_amd64.rpm

# Alpine Linux
$ sudo apk add --allow-untrusted gup_1.8.1_linux_amd64.apk
```
Replace `1.8.1` with the release you downloaded and `amd64` with `arm64` where applicable. The packages also install the bash, fish, and zsh completion files.

## Verifying release integrity
Every release ships supply-chain metadata so you can verify what you download:

- Signed checksums: `checksums.txt` is signed with [cosign](https://github.com/sigstore/cosign) (keyless), producing `checksums.txt.sigstore.json`.
- SBOM: an SPDX Software Bill of Materials is attached to each release archive.
- Build provenance: SLSA build provenance is attested via GitHub OIDC.

Verify the signed checksums (then check your archive against `checksums.txt`):

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Verify the build provenance of a downloaded artifact with the GitHub CLI:

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

## How to use
### Check the gup version
Print the version with either the `version` subcommand or the top-level `--version`/`-V` flag.
```shell
$ gup --version
$ gup version
```

### Update all binaries
`gup update` updates every binary under `$GOBIN`, in parallel.

![update](./doc/img/update.gif)

### Update the specified binary
If you want to update only the specified binaries, you specify multiple command names separated by space.
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### Exclude binaries during gup update
If you don't want to update some binaries simply specify binaries which should not be updated separated using ',' without spaces as a delimiter.
Also works in combination with --dry-run
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude or -e, this example will exclude 'gopls' and 'golangci-lint'
```

### Update binaries with @main, @master, or @latest
If you want to control update source per binary, use the following options:
- `--main` (`-m`): update by `@main` (falls back to `@master` only when the repository has no `main` branch)
- `--master`: update by `@master`
- `--latest`: update by `@latest`

The `@main` → `@master` fallback applies only to a missing `main` branch. Build, network, authentication, timeout, and cancellation failures are reported as-is and never silently install `@master`.

The selected channel is saved to `gup.json` and reused by future `gup update` runs.
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
```

### Pin a tool to a specific version

Use `pin` when a global tool must stay on a specific version, for example when it needs to match CI or a team-wide development environment.

```shell
$ gup pin golangci-lint v1.62.0
$ gup update
```

A pinned tool is installed with the recorded version (`go install <import_path>@<version>`), never `@latest`. `gup update` keeps it at that version and reinstalls it there if the installed version differs; the rest of the tool set still updates as usual. The pin locks the module version, not the Go build, so a pinned tool is still rebuilt at the pinned version when the Go toolchain changes (use `--ignore-go-update` to suppress that, exactly as for unpinned tools). The pin is stored in `gup.json` with `channel: "pinned"`:

```json
{
  "schema_version": 2,
  "packages": [
    {
      "name": "golangci-lint",
      "import_path": "github.com/golangci/golangci-lint/cmd/golangci-lint",
      "version": "v1.62.0",
      "channel": "pinned"
    }
  ]
}
```

`gup pin` also accepts the `tool@version` form (`gup pin golangci-lint@v1.62.0`). The tool must already be installed under `$GOBIN`. To allow the tool to update again:

```shell
$ gup unpin golangci-lint
```

`gup check` reports a pinned tool as `pinned` when it is at the pinned version and built with the current Go toolchain, or `pin-mismatch` (with a `gup update <name>` suggestion) when the installed version differs or a Go-toolchain rebuild is pending; it never compares a pinned tool against `@latest`.

### List up command name with package path and version under $GOPATH/bin
list subcommand print command information under $GOPATH/bin or $GOBIN. The output information is the command name, package path, and command version.
![list](./doc/img/list.gif)

### Remove the specified binary
If you want to remove a command under $GOPATH/bin or $GOBIN, use the remove subcommand. The remove subcommand asks if you want to remove it before removing it.
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

If you want to force the removal, use the --force option.
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

In non-interactive execution (when stdin is not a TTY, e.g. CI or a pipe), `gup remove` no longer blocks waiting for confirmation. It fails fast with a clear message; pass `--force` to remove without confirmation.

### Check if the binary is the latest version
If you want to know if the binary is the latest version, use the check subcommand. check subcommand checks if the binary is the latest version and displays the name of the binary that needs to be updated.
```shell
$ gup check
check binary under $GOPATH/bin or $GOBIN
[ 1/33] github.com/cheat/cheat (Already up-to-date: v0.0.0-20211009161301-12ffa4cb5c87 / go1.22.4)
[ 2/33] fyne.io/fyne/v2 (current: v2.1.3, latest: v2.1.4 / current: go1.20.2, installed: go1.22.4)
   :
[33/33] github.com/nao1215/ubume (Already up-to-date: v1.5.0 / go1.22.4)

If you want to update binaries, the following command.
           $ gup update fyne_demo gup mimixbox
```

Like other subcommands, you can only check the specified binaries.
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
           $ gup update mimixbox
```

### Quiet output for large tool sets
`check` and `update` print every binary by default, which is noisy when you have many tools installed. Pass `--quiet` (`-q`) to suppress the up-to-date lines and show only the binaries that were updated (or have an update available) plus failures, followed by a one-line summary. Errors are always written to STDERR, so they stay visible. When `--json` is also given, `--quiet` is ignored and the full JSON array is printed.
```shell
$ gup update --quiet
github.com/nao1215/gup (v0.7.0 to v0.7.1)
gup: 1 updated, 8 up-to-date, 0 failed

$ gup check -q
github.com/nao1215/gup (current: v0.7.0, latest: v0.7.1 / go1.22.4)

If you want to update binaries, run the following command.
           $ gup update gup
gup: 1 update available, 8 up-to-date, 0 failed
```

### Machine-readable JSON output (for scripting / CI)
`list`, `check`, and `update` accept `--json`, printing a JSON array instead of the human-readable output (which stays the default).

```shell
$ gup check --json
[
  {
    "name": "gup",
    "import_path": "github.com/nao1215/gup",
    "module_path": "github.com/nao1215/gup",
    "channel": "latest",
    "current_version": "v1.0.0",
    "latest_version": "v1.1.0",
    "current_go_version": "go1.22.4",
    "installed_go_version": "go1.22.4",
    "status": "update-available"
  }
]
```

Each element has these fields: `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`/`pinned`), `current_version`, `latest_version` (empty for `list` and for pinned packages), `pinned_version` (present only for `channel: "pinned"`), `current_go_version`, `installed_go_version`, `status`, `error` (omitted when absent), and `hint` (a next-step suggestion, present only when one applies to the error). `status` is `installed` (list), `up-to-date`, `update-available` (check), `updated` (update), `pinned`/`pin-mismatch` (a pinned package at / away from its pinned version), or `error`.

The array is always valid JSON, including partial failures (those packages get `"status": "error"`; error detail also goes to STDERR so STDOUT stays pure JSON). Exit codes are unchanged—`check` reporting `update-available` still exits `0`.

### Failure diagnostics / next-step hints
When `update` or `check` fails, gup turns the Go toolchain's cryptic output into a short, actionable next step printed on STDERR right after the error (and exposed as the `hint` field with `--json`):

```shell
$ gup update
gup:ERROR: [1/1] tool: can't install gup.test/moved/cmd/tool:
go: gup.test/moved/cmd/tool@latest: module gup.test/moved@latest found (v1.1.0), but does not contain package gup.test/moved/cmd/tool
gup:HINT : The module no longer provides this command at its import path. The project likely moved to a new major version (e.g. a `/v2` module path) or relocated the command; check its current install instructions and reinstall with the new path.
```

Hints cover module renames/major-version moves, relocated commands, `go.mod` `replace` directives, binaries not installed via `go install`, missing branch/tag, unresolvable/private/deleted repositories, permission and network errors, and an out-of-date Go toolchain. gup stays silent when it has nothing reliable to add (e.g. a timeout, whose message already names the remedy).

### Behavior on an empty environment
An empty global environment (no binaries installed by `go install` yet) is treated as a normal first-run condition, not an error:

- `list`, `check`, and `update` exit `0`, printing a short informational note (or a valid empty `[]` with `--json`).
- `export` exits `0` and writes an empty `gup.json`.

Naming a binary that is not installed, or excluding every binary, is still a usage error and exits `1`.

A config problem is also still reported even on an empty environment: if the `gup.json` that would be read (an explicit `--file`, or an auto-detected one) is malformed, has an unsupported schema/channel/pin, or is ambiguous (both the user-level config and `./gup.json` exist with no `--file`), `check`, `update`, and `list --json` fail fast and exit `1` instead of silently ignoring it.

### Export／Import subcommand
Use export/import when you want to install the same Go binaries across multiple systems.
`gup.json` stores each tool's import path, the recorded binary `version`, and its update `channel` (`latest` / `main` / `master` / `pinned`). For `channel: "pinned"`, `version` is the exact target version the tool is held at; for the other channels it is the version that was recorded at export time. `import` installs the exact version written in the file, and a pinned package stays pinned after import.

```json
{
  "schema_version": 1,
  "packages": [
    {
      "name": "gal",
      "import_path": "github.com/nao1215/gal/cmd/gal",
      "version": "v1.1.1",
      "channel": "latest"
    },
    {
      "name": "posixer",
      "import_path": "github.com/nao1215/posixer",
      "version": "v0.1.0",
      "channel": "main"
    }
  ]
}
```

By default:
- `gup export` writes to `$XDG_CONFIG_HOME/gup/gup.json`
- `gup import`, `gup check`, and `gup update` auto-detect the config path in this order:
  1) `$XDG_CONFIG_HOME/gup/gup.json` (if exists)
  2) `./gup.json` (if exists)

If both the user-level `gup.json` and `./gup.json` exist, `import`, `check`, `update`, and `list --json` fail fast and ask you to disambiguate with `--file`, instead of silently picking one. You can always override the path with `--file` (`-f`); `list` accepts `--file` together with `--json` to choose the config that supplies the reported `channel`.

`schema_version` is `1` for configs with no pinned packages and `2` once any package is pinned, so an environment that uses no pins keeps producing the `1` format that older gup releases can read. gup reads both `1` and `2`. The `pinned` channel is only valid under `schema_version: 2`; a `pinned` entry under `schema_version: 1`, a pinned package without a concrete version, an unknown channel value, or an unsupported `schema_version` is rejected.

A malformed or invalid `gup.json` (invalid JSON, an unknown channel, an unsupported `schema_version`, or an unsafe pin) is treated as an error rather than silently ignored: `check`, `update`, and `export` fail fast and name the offending file, so saved per-package channels are never quietly downgraded to `latest` because the config could not be parsed. An unknown channel is never normalized to `latest`.

When exporting to a file, `gup export` reads saved update channels from the same `gup.json` it writes to: a default export (no `--file`) reads from and writes to the canonical user-level `gup.json`, while `gup export --file <path>` reads from and writes to `<path>`. Exporting back to the same alternate config file therefore preserves its saved channels (round-trip safe) instead of resetting them to `latest` from another source. A first export to a brand-new file has no saved channels to read, so its packages are recorded as `latest`. With `--output`, `--file` still selects the channel source, but the exported config is printed to STDOUT instead of being written back to that path.

```shell
※ Environments A (e.g. ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.json

※ Environments B (e.g. debian)
$ gup import
```

`export` can print config content to STDOUT by `--output`. `import` can read a specific file by `--file`.
```shell
※ Environments A (e.g. ubuntu)
$ gup export --output > gup.json

※ Environments B (e.g. debian)
$ gup import --file=gup.json
```

### Migrate binaries to a new $GOBIN

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` reinstalls the Go binaries under `BEFORE_PATH` into `AFTER_PATH`, using the exact `import path@version` recorded in each binary's build info (it never silently upgrades to `@latest`). Internally it just sets `GOBIN` to `AFTER_PATH` and runs the normal `go install` path, so the binaries are rebuilt with the Go toolchain currently in use.

#### Why this is useful (e.g. with `mise`)

When you manage Go with [`mise`](https://mise.jdx.dev/), updating Go can change the real path of `$GOBIN` per Go version. As a result, tools you installed under the previous `$GOBIN` are no longer visible to the new Go. `gup migrate` lets you reinstall the same Go tool set from the old `$GOBIN` into the new one:

```shell
# Reinstall every go-install tool from the old GOBIN into the new GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrate only specific binaries
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` is add-only:

- It never deletes or cleans up files in `AFTER_PATH`.
- Binaries that already exist in `AFTER_PATH` are skipped by default. Use `--force` to reinstall over them.
- `AFTER_PATH` is created automatically when it does not exist.
- `BEFORE_PATH` and `AFTER_PATH` must be different directories.

Binaries whose import path or version cannot be resolved, and development builds (`devel` / `(devel)`), are skipped instead of being upgraded, so local or non-reproducible builds are never broken.

Supported flags: `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`), `--force`.

### Generate man-pages (for linux, mac)
man subcommand generates man-pages under /usr/share/man/man1 by default. If `MANPATH` is set, gup writes to the `man1` directory under each entry instead, creating it when it does not exist yet. An unwritable target exits with a clear error.
```shell
$ sudo gup man
Generate /usr/share/man/man1/gup-bug-report.1.gz
Generate /usr/share/man/man1/gup-check.1.gz
Generate /usr/share/man/man1/gup-completion.1.gz
Generate /usr/share/man/man1/gup-export.1.gz
Generate /usr/share/man/man1/gup-import.1.gz
Generate /usr/share/man/man1/gup-list.1.gz
Generate /usr/share/man/man1/gup-man.1.gz
Generate /usr/share/man/man1/gup-migrate.1.gz
Generate /usr/share/man/man1/gup-remove.1.gz
Generate /usr/share/man/man1/gup-update.1.gz
Generate /usr/share/man/man1/gup-version.1.gz
Generate /usr/share/man/man1/gup.1.gz
```

### Generate shell completion file (for bash, zsh, fish, PowerShell)
`completion` prints completion scripts to STDOUT when you pass a shell name.
`--install` sets completion up for you instead, for the shells of the platform you are on.

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# Install files automatically to default user paths
$ gup completion --install
```

On Linux and macOS, `--install` writes bash, fish, and zsh completion to the paths that match your shell/config layout: bash honors `XDG_DATA_HOME` (falling back to `$HOME/.local/share`), fish honors `XDG_CONFIG_HOME` (falling back to `$HOME/.config`), and zsh resolves both the completion file and `.zshrc` via `ZDOTDIR` (falling back to `$HOME`). It still requires `HOME` to be set; it fails fast (without writing files into the current directory) when `HOME` is empty, and exits non-zero if any completion file cannot be written.

On Windows, the same command sets up PowerShell — no redirecting and no hand-editing:

```powershell
PS> gup completion --install
PS> . $PROFILE   # or open a new PowerShell window
```

It writes `gup.completion.ps1` next to your PowerShell profile and adds one guarded dot-source line to the profile itself, inside a block marked `# setting for gup command (auto generate)`. Everything else in your profile is left exactly as it was, the profile (and its parent directory) is created if it does not exist yet, and the write is atomic. gup installs into the profile `$PROFILE` names when that variable is exported, and otherwise into **every** profile that already exists under `Documents\PowerShell` (PowerShell 7) and `Documents\WindowsPowerShell` (Windows PowerShell 5.1) — the two shells read different profiles and are commonly installed side by side, so wiring up only one would leave the other with no completion after a command that reported success. If neither exists, the PowerShell 7 profile is created. Those paths resolve under `USERPROFILE`, falling back to `HOME`; with neither set it fails fast with a message naming both rather than guessing.

Re-running `--install` is idempotent on every platform: it does not duplicate the zsh init snippet in `.zshrc` or the gup block in your PowerShell profile.

### Running two gup commands at once
The commands that change state take a lock on each resource they write, so a second one refuses to start rather than interleaving with the first.

| Command | What it locks |
|:--|:--|
| `update` | `$GOBIN` and the `gup.json` it may write |
| `import` | `$GOBIN` |
| `remove` | `$GOBIN` |
| `migrate` | `BEFORE_PATH` and `AFTER_PATH` |
| `export`, `pin` | `$GOBIN` and the `gup.json` they write |
| `unpin` | the `gup.json` it writes |

The lock is the operating system's own — `flock` on Linux and macOS, `LockFileEx` on Windows — taken on a file gup keeps open for as long as it holds the resource. That choice is what makes the rest of this section short: a lock the kernel owns is released the moment the process holding it ends, however it ends, so there is no such thing as a stale gup lock and never a file for you to delete.

The files it takes the lock on are `$GOBIN/.gup.lock` and `<gup.json>.lock`, next to what they guard. That is deliberate: `$GOBIN` and your config directory move independently, so a per-project `XDG_CONFIG_HOME` still shares one `$GOBIN` with every other project, and two commands given the same `--file` may be started from different config directories entirely. A lock kept in the config directory would serialize neither. (`.gup.lock` starts with a dot, so `gup list` never shows it, and `gup remove` refuses to delete it — it is gup's, not a tool you installed.)

A lock is scoped to the *file*, not to the path that names it. Two arguments that reach one directory — `gup migrate ~/go/bin ~/bin` where `~/bin` is a symlink to `~/go/bin`, or a `$GOBIN` spelled two ways on the case-insensitive filesystems macOS and Windows use — take one lock, not two, so gup never waits for itself. And a lock path that is a symlink is refused rather than followed: gup truncates its lock file to record who holds it, so writing through a link somebody put there would mean truncating a file that is not gup's.

```shell
$ gup update
```
The second one exits non-zero after reporting who is in the way:

> another gup process is already running (pid 40321 on carbon, running "gup update",
> since 2026-08-29T17:04:11+09:00). gup serializes commands that change your $GOBIN or
> gup.json, so wait for it to finish and run this command again. The lock is held by the
> operating system, not by /home/you/go/bin/.gup.lock, so it is released the moment that
> process ends and there is never a file to delete by hand

`export`, `pin` and `migrate` lock directories they never write to, because what they write is derived from what they read there: `export`'s whole output is a description of `$GOBIN`, `pin` resolves its target against it, and `migrate` reinstalls into `AFTER_PATH` the versions it read in `BEFORE_PATH`. A `gup remove` deleting a binary halfway through any of those leaves a result describing a tool set that never existed. `unpin` names an entry in `gup.json` and never looks at `$GOBIN`, so it does not wait behind one.

A `$GOBIN` that does not exist yet is created so it can be locked, by the commands that read it as well as those that install into it. Whether it exists is exactly what a concurrent `gup import` changes, and a command that skipped the lock because the directory was missing would be the one command in the set with no protection at all.

Two commands write files and still take no lock: `gup completion --install` and `gup man`. Both write with the same atomic replace `gup.json` gets, and two runs of either produce byte-identical content, so the only thing a lock would add is a `.zshrc.lock` in your home directory. What neither a lock nor anything else can protect is your editor writing `.zshrc` at the same moment.

Nothing that changes no state is blocked. `update --dry-run`, `import --dry-run`, `migrate --dry-run`, and `export --output` take no lock at all, and neither do the read-only commands (`list`, `check`, `version`, `completion`, `man`, `bug-report`) — gup replaces `gup.json` with an atomic rename, so a reader always sees a complete file and has nothing to wait for. That holds for a read-only `gup.json` too: the read-only bit is cleared for the length of the rename and put back, rather than moving the old file aside and leaving the path briefly empty.

#### The lock files stay behind, and that is fine
An empty `.gup.lock` in `$GOBIN`, or a `gup.json.lock` beside your config, is not a leftover to clean up. gup never deletes them, because deleting a file another gup may already have opened is precisely what would let two processes take a lock on two different files at one path. Between commands the file holds nothing at all: it is a name for the kernel to hang the next lock on, and it is emptied when the lock is dropped, so it never names a process that has already finished. There is nothing to do about one, and `gup remove .gup.lock` is refused for that reason — as is any other name that reaches the same file, whether that is a hard link, a Windows spelling with a trailing dot, or an 8.3 alias like `GUPLOC~1.LOC`. Deleting it would not release the lock, which lives on an open handle; it would free the *name*, and the next gup would create a fresh file there and lock that instead.

Nothing wedges. A gup killed with `kill -9`, a machine that lost power mid-update, a lock file copied onto a shared home directory from another machine — none of them block anything, because none of them is holding a lock. If you interrupt a `gup update`, the next one runs immediately.

Ctrl-C does not release the lock — the process holding it does, by ending. Releasing it from a signal handler would free the resource while the command is still installing binaries and rewriting `gup.json` on its way out, so a second gup started in that moment would run alongside the first. An interrupted `gup update` therefore stops its work, unwinds, and releases on the way out; a command killed outright never gets that far, and the kernel drops the lock as it reaps the process. Either way no second gup gets in early, and no cleanup is left for you.

#### Where the lock does not reach
Two situations are outside what this can promise, and both are worth naming rather than discovering.

The first is a `$GOBIN` or a `gup.json` on a network filesystem — NFS, SMB, sshfs. `flock` and `LockFileEx` are the kernel's, and what a kernel does with them on a remote mount is up to the mount: some map them to a server-side lock, some make them local to one machine, some ignore them. Two gups on one machine are still serialized; two on different machines sharing the mount may not be.

The second is deleting a lock file *while* gup is running. Doing so does not stop the running command — its lock is on an open handle — but it frees the name, and the next gup creates a new file there and locks that instead, leaving two commands changing one `$GOBIN`. gup never deletes these files itself and refuses to let `gup remove` do it; nothing else should either.

> [!NOTE]
> gup v1.8.1 and earlier take no lock at all. If you keep an older gup on your `PATH` and run it against the same `$GOBIN` at the same time as a current one, the two are not serialized — the older one does not know there is anything to wait for. Two current gups always are.

### Desktop notification
If you use gup with --notify option, gup command notify you on your desktop whether the update was successful or unsuccessful after the update was finished.
```shell
$ gup update --notify
```
![success](./doc/img/notify_success.png)
![warning](./doc/img/notify_warning.png)

### Disable colorized output
gup colorizes its output by default. To turn colors off, pass `--no-color` or set the `NO_COLOR` environment variable to a non-empty value (following the [NO_COLOR](https://no-color.org/) convention). This is useful when piping output, in CI logs, or with `NO_COLOR` set globally.
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## gup vs. `go tool`
Go 1.24's built-in [`go tool`](https://go.dev/doc/modules/managing-dependencies#tools) manages tools scoped to a single project and recorded in that project's `go.mod`, so those tools exist only inside that module. gup manages the binaries installed system-wide under `$GOBIN`, the commands you run from any directory and keep alongside your dotfiles, optionally pinned to versions you depend on. Use `go tool` for per-project tooling and gup for your global toolbox.

## Feature comparison

| Feature | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| Parallel update | Yes | No | Manual |
| Update time, 9 binaries | 0.7s | 2.9s | 2.9s |
| Per-package update channels (`latest`/`main`/`master`) | Yes | No | Manual |
| Version pinning / lock | Yes | No | Manual |
| Export/import tool set | Yes | No | Manual |
| Migrate binaries to a new `$GOBIN` | Yes | No | Manual |
| Machine-readable JSON output (`--json`) | Yes | No | No |
| Shell completion generation/install | Yes | No | No |
| `update` reinstalls up-to-date binaries | No | Yes | Yes |
| `migrate --force` reinstalls when the target already exists | Yes | No | Manual |
| Failure diagnostics / next-step hints | Yes | Yes | No |
| `NO_COLOR` support | Yes | Yes | — |

*Update time: 9 binaries each with a newer version available; gup updates in parallel, the others sequentially. AMD Ryzen AI Max+ 395 / go 1.26.4, median of 5 runs with a warm module cache; times depend on build time and CPU.*

## Integrations
[Topgrade](https://github.com/topgrade-rs/topgrade) updates the Go binaries under `$GOBIN` by running `gup update` when gup is installed. Nothing extra is needed on the gup side; how the step behaves is documented by Topgrade.

## FAQ

### `gup` fails with `fatal: not a git repository`
You are probably on oh-my-zsh, which ships a `gup` alias for `git pull --rebase` that shadows this command ([#16](https://github.com/nao1215/gup/issues/16), [#204](https://github.com/nao1215/gup/issues/204)). Remove or rename that alias, or run gup with a leading backslash to bypass it:
```shell
$ \gup update
```

## Contributing
First off, thanks for taking the time to contribute! ❤️  See [CONTRIBUTING.md](./CONTRIBUTING.md) for more information.
Developer workflow, quality checklist, and tool management are documented in [CONTRIBUTING.md](./CONTRIBUTING.md).
Contributions are not only related to development. For example, GitHub Star motivates me to develop!

## Contact
If you would like to send comments such as "find a bug" or "request for additional features" to the developer, please use one of the following contacts.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

You can use the bug-report subcommand to send a bug report.
```
$ gup bug-report
※ Open GitHub issue page by your default browser
```

## LICENSE
The gup project is licensed under the terms of [the Apache License 2.0](./LICENSE).


## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=64" width="64px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=nao1215" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://qiita.com/KEINOS"><img src="https://avatars.githubusercontent.com/u/11840938?v=4?s=64" width="64px;" alt="KEINOS"/><br /><sub><b>KEINOS</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=KEINOS" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://mattn.kaoriya.net/"><img src="https://avatars.githubusercontent.com/u/10111?v=4?s=64" width="64px;" alt="mattn"/><br /><sub><b>mattn</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=mattn" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://jlec.de/"><img src="https://avatars.githubusercontent.com/u/79732?v=4?s=64" width="64px;" alt="Justin Lecher"/><br /><sub><b>Justin Lecher</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=jlec" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/lincolnthalles"><img src="https://avatars.githubusercontent.com/u/7476810?v=4?s=64" width="64px;" alt="Lincoln Nogueira"/><br /><sub><b>Lincoln Nogueira</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=lincolnthalles" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/matsuyoshi30"><img src="https://avatars.githubusercontent.com/u/16238709?v=4?s=64" width="64px;" alt="Masaya Watanabe"/><br /><sub><b>Masaya Watanabe</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=matsuyoshi30" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/memreflect"><img src="https://avatars.githubusercontent.com/u/59116123?v=4?s=64" width="64px;" alt="memreflect"/><br /><sub><b>memreflect</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=memreflect" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Akimon658"><img src="https://avatars.githubusercontent.com/u/81888693?v=4?s=64" width="64px;" alt="Akimo"/><br /><sub><b>Akimo</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=Akimon658" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/rkscv"><img src="https://avatars.githubusercontent.com/u/155284493?v=4?s=64" width="64px;" alt="rkscv"/><br /><sub><b>rkscv</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=rkscv" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/scop"><img src="https://avatars.githubusercontent.com/u/109152?v=4?s=64" width="64px;" alt="Ville Skyttä"/><br /><sub><b>Ville Skyttä</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=scop" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://mochaa.ws/?utm_source=github_user"><img src="https://avatars.githubusercontent.com/u/21154023?v=4?s=64" width="64px;" alt="Zephyr Lykos"/><br /><sub><b>Zephyr Lykos</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=mochaaP" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://itrooz.fr"><img src="https://avatars.githubusercontent.com/u/42669835?v=4?s=64" width="64px;" alt="iTrooz"/><br /><sub><b>iTrooz</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=iTrooz" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://pacman.blog.br"><img src="https://avatars.githubusercontent.com/u/59438?v=4?s=64" width="64px;" alt="Tiago Peczenyj"/><br /><sub><b>Tiago Peczenyj</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=peczenyj" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://shogo82148.github.io/"><img src="https://avatars.githubusercontent.com/u/1157344?v=4?s=64" width="64px;" alt="ICHINOSE Shogo"/><br /><sub><b>ICHINOSE Shogo</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=shogo82148" title="Documentation">📖</a> <a href="https://github.com/nao1215/gup/commits?author=shogo82148" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="http://blog.lenhof.eu.org/"><img src="https://avatars.githubusercontent.com/u/36410287?v=4?s=64" width="64px;" alt="Jean-Yves LENHOF"/><br /><sub><b>Jean-Yves LENHOF</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=jylenhof" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://clarabennett2626.github.io/"><img src="https://avatars.githubusercontent.com/u/261616207?v=4?s=64" width="64px;" alt="Clara Bennett"/><br /><sub><b>Clara Bennett</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=clarabennett2626" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/LucasM4r"><img src="https://avatars.githubusercontent.com/u/83995229?v=4?s=64" width="64px;" alt="Lucas Marchesan"/><br /><sub><b>Lucas Marchesan</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=LucasM4r" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hsn10"><img src="https://avatars.githubusercontent.com/u/1170075?v=4?s=64" width="64px;" alt="Radim Kolar"/><br /><sub><b>Radim Kolar</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Ahsn10" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Mohabdo21"><img src="https://avatars.githubusercontent.com/u/139122098?v=4?s=64" width="64px;" alt="Mohannad Abdulaziz"/><br /><sub><b>Mohannad Abdulaziz</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3AMohabdo21" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hu3bi"><img src="https://avatars.githubusercontent.com/u/132217293?v=4?s=64" width="64px;" alt="Yannick"/><br /><sub><b>Yannick</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Ahu3bi" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://diegoalcantara.com.br"><img src="https://avatars.githubusercontent.com/u/21999506?v=4?s=64" width="64px;" alt="Diego Alcântara"/><br /><sub><b>Diego Alcântara</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Adgoalcantara" title="Bug reports">🐛</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://gabnotes.org"><img src="https://avatars.githubusercontent.com/u/3630554?v=4?s=64" width="64px;" alt="Crocmagnon"/><br /><sub><b>Crocmagnon</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3ACrocmagnon" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://reliable.network"><img src="https://avatars.githubusercontent.com/u/1992842?v=4?s=64" width="64px;" alt="Luke Hamburg"/><br /><sub><b>Luke Hamburg</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Aluckman212" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://onee3.org"><img src="https://avatars.githubusercontent.com/u/4507647?v=4?s=64" width="64px;" alt="Frederick Zhang"/><br /><sub><b>Frederick Zhang</b></sub></a><br /><a href="#ideas-Frederick888" title="Ideas, Planning, & Feedback">🤔</a> <a href="#platform-Frederick888" title="Packaging/porting to new platform">📦</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/trallnag"><img src="https://avatars.githubusercontent.com/u/24834206?v=4?s=64" width="64px;" alt="Tim Schwenke"/><br /><sub><b>Tim Schwenke</b></sub></a><br /><a href="#ideas-trallnag" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ybrhue"><img src="https://avatars.githubusercontent.com/u/35401453?v=4?s=64" width="64px;" alt="ybrhue"/><br /><sub><b>ybrhue</b></sub></a><br /><a href="#ideas-ybrhue" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://nexiom.net"><img src="https://avatars.githubusercontent.com/u/3214803?v=4?s=64" width="64px;" alt="Samuel D. Leslie"/><br /><sub><b>Samuel D. Leslie</b></sub></a><br /><a href="#ideas-ralish" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://giggio.net"><img src="https://avatars.githubusercontent.com/u/334958?v=4?s=64" width="64px;" alt="Giovanni Bassi"/><br /><sub><b>Giovanni Bassi</b></sub></a><br /><a href="#ideas-giggio" title="Ideas, Planning, & Feedback">🤔</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="http://www.craig-wood.com/nick/"><img src="https://avatars.githubusercontent.com/u/536803?v=4?s=64" width="64px;" alt="Nick Craig-Wood"/><br /><sub><b>Nick Craig-Wood</b></sub></a><br /><a href="#ideas-ncw" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://chenrui.dev"><img src="https://avatars.githubusercontent.com/u/1580956?v=4?s=64" width="64px;" alt="Rui Chen"/><br /><sub><b>Rui Chen</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Achenrui333" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/phanirithvij"><img src="https://avatars.githubusercontent.com/u/29627898?v=4?s=64" width="64px;" alt="phanirithvij"/><br /><sub><b>phanirithvij</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3Aphanirithvij" title="Bug reports">🐛</a> <a href="#platform-phanirithvij" title="Packaging/porting to new platform">📦</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Darkcast"><img src="https://avatars.githubusercontent.com/u/1676655?v=4?s=64" width="64px;" alt="Darkcast"/><br /><sub><b>Darkcast</b></sub></a><br /><a href="https://github.com/nao1215/gup/issues?q=author%3ADarkcast" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hpsbranco"><img src="https://avatars.githubusercontent.com/u/3085888?v=4?s=64" width="64px;" alt="HenriqueB"/><br /><sub><b>HenriqueB</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=hpsbranco" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://rafaeldominiquini.ddns.net/"><img src="https://avatars.githubusercontent.com/u/1180808?v=4?s=64" width="64px;" alt="Rafael Baboni Dominiquini"/><br /><sub><b>Rafael Baboni Dominiquini</b></sub></a><br /><a href="#platform-Dominiquini" title="Packaging/porting to new platform">📦</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/branchv"><img src="https://avatars.githubusercontent.com/u/19800529?v=4?s=64" width="64px;" alt="Branch Vincent"/><br /><sub><b>Branch Vincent</b></sub></a><br /><a href="#platform-branchv" title="Packaging/porting to new platform">📦</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://nikolasgrottendieck.com/"><img src="https://avatars.githubusercontent.com/u/887496?v=4?s=64" width="64px;" alt="Nikolas Grottendieck"/><br /><sub><b>Nikolas Grottendieck</b></sub></a><br /><a href="#platform-Okeanos" title="Packaging/porting to new platform">📦</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
