<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-14-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![gosec](https://github.com/nao1215/gup/actions/workflows/security.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[日本語](./doc/ja/README.md) | [Русский](./doc/ru/README.md) | [中文](./doc/zh-cn/README.md) | [한국어](./doc/ko/README.md) | [Español](./doc/es/README.md) | [Français](./doc/fr/README.md)

# gup - Update binaries installed by "go install"

![sample](./doc/img/sample.png)

**gup** command update binaries installed by "go install" to the latest version. gup updates all binaries in parallel, so very fast. It also provides subcommands for manipulating binaries under \$GOPATH/bin (\$GOBIN). It is a cross-platform software that runs on Windows, Mac and Linux.

If you are using oh-my-zsh, then gup has an alias set up. The alias is `gup - git pull --rebase`. Therefore, please make sure that the oh-my-zsh alias is disabled (e.g. $ \gup update).


## Supported OS (unit testing with GitHub Actions)
- Linux
- Mac
- Windows

## How to install
### Use "go install"
If you does not have the golang development environment installed on your system, please install golang from the [golang official website](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```

### Use homebrew
```shell
brew install nao1215/tap/gup
```

### Use mise-en-place
```shell
mise use -g gup@latest
``

### Install from Package or Binary
[The release page](https://github.com/nao1215/gup/releases) contains packages in .deb, .rpm, and .apk formats. gup command uses the go command internally, so the golang installation is required.


## How to use
### Update all binaries
If you update all binaries, you just run `$ gup update`.

```shell
$ gup update
update binary under $GOPATH/bin or $GOBIN
[ 1/30] github.com/cheat/cheat/cmd/cheat (Already up-to-date: v0.0.0-20211009161301-12ffa4cb5c87 / go1.22.4)
[ 2/30] fyne.io/fyne/v2/cmd/fyne_demo (Already up-to-date: v2.1.3 / go1.22.4)
[ 3/30] github.com/nao1215/gal/cmd/gal (v1.0.0 to v1.2.0 / go1.22.4)
[ 4/30] github.com/matsuyoshi30/germanium/cmd/germanium (Already up-to-date: v1.2.2 / go1.22.4)
[ 5/30] github.com/onsi/ginkgo/ginkgo (Already up-to-date: v1.16.5 / go1.22.4)
[ 6/30] github.com/git-chglog/git-chglog/cmd/git-chglog (Already up-to-date: v0.15.1 / go1.22.4)
   :
   :
```

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

### Update binaries with @main or @master
If you want to update binaries with @master or @main, you can specify the -m or --master option.
```shell
$ gup update --main=gup,lazygit,sqly
```

### List up command name with package path and version under $GOPATH/bin
list subcommand print command information under $GOPATH/bin or $GOBIN. The output information is the command name, package path, and command version.
![sample](doc/img/list.png)

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
### Export／Import subcommand
You use the export／import subcommand if you want to install the same golang binaries across multiple systems. By default, export-subcommand exports the file to $XDG_CONFIG_HOME/gup/gup.conf. If you want to know [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html), see this link. After you have placed gup.conf in the same path hierarchy on another system, you execute import-subcommand. gup start the installation
according to the contents of gup.conf.

```shell
※ Environments A (e.g. ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.conf

※ Environments B (e.g. debian)
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```

Alternatively, export subcommand print package information (it's same as gup.conf) that you want to export at STDOUT if you use --output option. import subcommand can also specify the gup.conf file path if you use --input option.
```shell
※ Environments A (e.g. ubuntu)
$ gup export --output > gup.conf

※ Environments B (e.g. debian)
$ gup import --input=gup.conf
```

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
Generate /usr/share/man/man1/gup-remove.1.gz
Generate /usr/share/man/man1/gup-update.1.gz
Generate /usr/share/man/man1/gup-version.1.gz
Generate /usr/share/man/man1/gup.1.gz
```

### Generate shell completion file (for bash, zsh, fish)
completion subcommand generates shell completion files for bash, zsh, and fish. If the shell completion file does not exist in the system, the generation process will begin. To activate the completion feature, restart the shell.

```shell
$ gup completion
create bash-completion file: /home/nao/.bash_completion
create fish-completion file: /home/nao/.config/fish/completions/gup.fish
create zsh-completion file: /home/nao/.zsh/completion/_gup
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
Contributions are not only related to development. For example, GitHub Star motivates me to develop!

### Star History
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

### For Developers
When adding new features or fixing bugs, please write unit tests. The gup is unit tested for all packages as the unit test tree map below shows.

![treemap](./doc/img/cover-tree.svg)

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
      <td align="center" valign="top" width="14.28%"><a href="https://shogo82148.github.io/"><img src="https://avatars.githubusercontent.com/u/1157344?v=4?s=100" width="100px;" alt="ICHINOSE Shogo"/><br /><sub><b>ICHINOSE Shogo</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=shogo82148" title="Documentation">📖</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
