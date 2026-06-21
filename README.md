<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-16-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[日本語](./doc/ja/README.md) | [Русский](./doc/ru/README.md) | [中文](./doc/zh-cn/README.md) | [한국어](./doc/ko/README.md) | [Español](./doc/es/README.md) | [Français](./doc/fr/README.md)


![sample](./doc/img/sample.gif)

gup updates binaries installed with `go install`, running the updates in parallel instead of one at a time.

gup also manages the tools under `$GOPATH/bin` (`$GOBIN`): `list` and `check` what is installed, `remove` binaries, `export`/`import` the set to reproduce the same tool set on another machine, and `migrate` them into a different `$GOBIN`. Runs on Windows, macOS, and Linux.

If you are using oh-my-zsh, then gup has an alias set up. The alias is `gup - git pull --rebase`. Therefore, please make sure that the oh-my-zsh alias is disabled (e.g. $ \gup update).

## Benchmark
gup runs updates in parallel, so it finishes faster than tools that update binaries one at a time. Updating 9 binaries that each had a newer version available:

| Tool | Strategy | Time |
|------|----------|-----:|
| gup update | parallel | 0.7s |
| [go-global-update](https://github.com/Gelio/go-global-update) | sequential | 2.9s |
| `go install` loop | sequential | 2.9s |

Measured on AMD Ryzen AI Max+ 395 (32 cores) / 64 GB RAM / Ubuntu 26.04 / go 1.26.4, median of 5 runs with a warm Go module cache. Times depend on each binary's build time and your CPU.


## Supported OS (unit testing with GitHub Actions)
- Linux
- Mac
- Windows

## How to install
gup is already available via `winget`, `mise`, and `nix` in addition to `go install` and Homebrew.

### Use "go install"
If you do not have the Go development environment installed on your system, please install it from the [official website](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```

### Use homebrew
```shell
brew install nao1215/gup
```

### Use winget (Windows)
```shell
winget install --id nao1215.gup
```

### Use mise-en-place
```shell
mise use -g gup@latest
```

### Use nix (Nix profile)
```shell
nix profile install nixpkgs#gogup
```

### Install from Package or Binary
[The release page](https://github.com/nao1215/gup/releases) contains packages in .deb, .rpm, and .apk formats. gup command uses the go command internally, so the golang installation is required.


## How to use
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
- `--main` (`-m`): update by `@main` (fallback to `@master`)
- `--master`: update by `@master`
- `--latest`: update by `@latest`

The selected channel is saved to `gup.json` and reused by future `gup update` runs.
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
```

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

### Machine-readable JSON output (for scripting / CI)
`list`, `check`, and `update` accept `--json`, printing a JSON array instead of the human-readable output (which stays the default).

```shell
$ gup check --json | jq -r '.[] | select(.status == "update-available") | .name'
```

Each element has these fields: `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`), `current_version`, `latest_version` (empty for `list`), `current_go_version`, `installed_go_version`, `status`, and `error` (omitted when absent). `status` is `installed` (list), `up-to-date`, `update-available` (check), `updated` (update), or `error`.

The array is always valid JSON, including partial failures (those packages get `"status": "error"`; error detail also goes to STDERR so STDOUT stays pure JSON). Exit codes are unchanged—`check` reporting `update-available` still exits `0`.

### Export／Import subcommand
Use export/import when you want to install the same Go binaries across multiple systems.
`gup.json` stores import path, binary version, and update channel (`latest` / `main` / `master`).
`import` installs the exact version written in the file.

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
- `gup import` auto-detects config path in this order:
  1) `$XDG_CONFIG_HOME/gup/gup.json` (if exists)
  2) `./gup.json` (if exists)

You can always override the path with `--file`.

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

`gup migrate` reinstalls the Go binaries under `BEFORE_PATH` into `AFTER_PATH`,
using the exact `import path@version` recorded in each binary's build info
(it never silently upgrades to `@latest`). Internally it just sets `GOBIN` to
`AFTER_PATH` and runs the normal `go install` path, so the binaries are rebuilt
with the Go toolchain currently in use.

#### Why this is useful (e.g. with `mise`)

When you manage Go with [`mise`](https://mise.jdx.dev/), updating Go can change
the real path of `$GOBIN` per Go version. As a result, tools you installed
under the previous `$GOBIN` are no longer visible to the new Go. `gup migrate`
lets you reinstall the same Go tool set from the old `$GOBIN` into the new one:

```shell
# Reinstall every go-install tool from the old GOBIN into the new GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrate only specific binaries
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` is add-only:

- It never deletes or cleans up files in `AFTER_PATH`.
- Binaries that already exist in `AFTER_PATH` are skipped by default. Use
  `--force` to reinstall over them.
- `AFTER_PATH` is created automatically when it does not exist.
- `BEFORE_PATH` and `AFTER_PATH` must be different directories.

Binaries whose import path or version cannot be resolved, and development
builds (`devel` / `(devel)`), are skipped instead of being upgraded, so local
or non-reproducible builds are never broken.

Supported flags: `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`),
`--force`.

### Generate man-pages (for linux, mac)
man subcommand generates man-pages under /usr/share/man/man1.
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
To install completion files into your user environment for bash/fish/zsh, use `--install`.
For PowerShell, redirect the output to a `.ps1` file and source it from your profile.

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# Install files automatically to default user paths
$ gup completion --install
```

### Desktop notification
If you use gup with --notify option, gup command notify you on your desktop whether the update was successful or unsuccessful after the update was finished.
```shell
$ gup update --notify
```
![success](./doc/img/notify_success.png)
![warning](./doc/img/notify_warning.png)


## Contributing
First off, thanks for taking the time to contribute! ❤️  See [CONTRIBUTING.md](./CONTRIBUTING.md) for more information.
Developer workflow, quality checklist, and tool management are documented in [CONTRIBUTING.md](./CONTRIBUTING.md).
Contributions are not only related to development. For example, GitHub Star motivates me to develop!

### Star History
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

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
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=100" width="100px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=nao1215" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://qiita.com/KEINOS"><img src="https://avatars.githubusercontent.com/u/11840938?v=4?s=100" width="100px;" alt="KEINOS"/><br /><sub><b>KEINOS</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=KEINOS" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://mattn.kaoriya.net/"><img src="https://avatars.githubusercontent.com/u/10111?v=4?s=100" width="100px;" alt="mattn"/><br /><sub><b>mattn</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=mattn" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://jlec.de/"><img src="https://avatars.githubusercontent.com/u/79732?v=4?s=100" width="100px;" alt="Justin Lecher"/><br /><sub><b>Justin Lecher</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=jlec" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/lincolnthalles"><img src="https://avatars.githubusercontent.com/u/7476810?v=4?s=100" width="100px;" alt="Lincoln Nogueira"/><br /><sub><b>Lincoln Nogueira</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=lincolnthalles" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/matsuyoshi30"><img src="https://avatars.githubusercontent.com/u/16238709?v=4?s=100" width="100px;" alt="Masaya Watanabe"/><br /><sub><b>Masaya Watanabe</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=matsuyoshi30" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/memreflect"><img src="https://avatars.githubusercontent.com/u/59116123?v=4?s=100" width="100px;" alt="memreflect"/><br /><sub><b>memreflect</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=memreflect" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Akimon658"><img src="https://avatars.githubusercontent.com/u/81888693?v=4?s=100" width="100px;" alt="Akimo"/><br /><sub><b>Akimo</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=Akimon658" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/rkscv"><img src="https://avatars.githubusercontent.com/u/155284493?v=4?s=100" width="100px;" alt="rkscv"/><br /><sub><b>rkscv</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=rkscv" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/scop"><img src="https://avatars.githubusercontent.com/u/109152?v=4?s=100" width="100px;" alt="Ville Skyttä"/><br /><sub><b>Ville Skyttä</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=scop" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://mochaa.ws/?utm_source=github_user"><img src="https://avatars.githubusercontent.com/u/21154023?v=4?s=100" width="100px;" alt="Zephyr Lykos"/><br /><sub><b>Zephyr Lykos</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=mochaaP" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://itrooz.fr"><img src="https://avatars.githubusercontent.com/u/42669835?v=4?s=100" width="100px;" alt="iTrooz"/><br /><sub><b>iTrooz</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=iTrooz" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://pacman.blog.br"><img src="https://avatars.githubusercontent.com/u/59438?v=4?s=100" width="100px;" alt="Tiago Peczenyj"/><br /><sub><b>Tiago Peczenyj</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=peczenyj" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://shogo82148.github.io/"><img src="https://avatars.githubusercontent.com/u/1157344?v=4?s=100" width="100px;" alt="ICHINOSE Shogo"/><br /><sub><b>ICHINOSE Shogo</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=shogo82148" title="Documentation">📖</a> <a href="https://github.com/nao1215/gup/commits?author=shogo82148" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="http://blog.lenhof.eu.org/"><img src="https://avatars.githubusercontent.com/u/36410287?v=4?s=100" width="100px;" alt="Jean-Yves LENHOF"/><br /><sub><b>Jean-Yves LENHOF</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=jylenhof" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://clarabennett2626.github.io/"><img src="https://avatars.githubusercontent.com/u/261616207?v=4?s=100" width="100px;" alt="Clara Bennett"/><br /><sub><b>Clara Bennett</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=clarabennett2626" title="Documentation">📖</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
