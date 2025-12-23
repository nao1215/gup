<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-15-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![gosec](https://github.com/nao1215/gup/actions/workflows/security.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

# gup - 更新通过"go install"安装的二进制文件

![sample](../img/sample.png)

**gup** 命令将通过"go install"安装的二进制文件更新到最新版本。gup 并行更新所有二进制文件，因此非常快速。它还提供用于操作 \$GOPATH/bin (\$GOBIN) 下二进制文件的子命令。它是一个跨平台软件，可在 Windows、Mac 和 Linux 上运行。

如果您正在使用 oh-my-zsh，那么 gup 设置了一个别名。该别名是 `gup - git pull --rebase`。因此，请确保禁用 oh-my-zsh 别名（例如 $ \gup update）。


## 支持的操作系统（通过 GitHub Actions 进行单元测试）
- Linux
- Mac
- Windows

## 如何安装
### 使用"go install"
如果您的系统上没有安装 golang 开发环境，请从 [golang 官方网站](https://go.dev/doc/install)安装 golang。
```
go install github.com/nao1215/gup@latest
```

### 使用 homebrew
```shell
brew install nao1215/gup
```

### 使用 mise-en-place
```shell
mise use -g gup@latest
```

### 从包或二进制文件安装
[发布页面](https://github.com/nao1215/gup/releases)包含 .deb、.rpm 和 .apk 格式的包。gup 命令内部使用 go 命令，因此需要安装 golang。


## 如何使用
### 更新所有二进制文件
如果要更新所有二进制文件，只需运行 `$ gup update`。

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

### 更新指定的二进制文件
如果您只想更新指定的二进制文件，请指定多个用空格分隔的命令名称。
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### 在 gup update 期间排除二进制文件
如果您不想更新某些二进制文件，只需指定不应更新的二进制文件，使用","作为分隔符，不要有空格。
也可以与 --dry-run 结合使用
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude 或 -e，此示例将排除 'gopls' 和 'golangci-lint'
```

### 使用 @main 或 @master 更新二进制文件
如果您想使用 @master 或 @main 更新二进制文件，可以指定 -m 或 --master 选项。
```shell
$ gup update --main=gup,lazygit,sqly
```

### 列出 $GOPATH/bin 下的命令名称及其包路径和版本
list 子命令打印 $GOPATH/bin 或 $GOBIN 下的命令信息。输出信息是命令名称、包路径和命令版本。
![sample](../img/list.png)

### 移除指定的二进制文件
如果您想移除 $GOPATH/bin 或 $GOBIN 下的命令，请使用 remove 子命令。remove 子命令在移除之前会询问您是否要移除它。
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

如果您想强制移除，请使用 --force 选项。
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

### 检查二进制文件是否为最新版本
如果您想知道二进制文件是否为最新版本，请使用 check 子命令。check 子命令检查二进制文件是否为最新版本，并显示需要更新的二进制文件的名称。
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

与其他子命令一样，您只能检查指定的二进制文件。
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
          $ gup update mimixbox
```
### 导出/导入子命令
如果您想在多个系统中安装相同的 golang 二进制文件，您可以使用 export/import 子命令。默认情况下，export 子命令将文件导出到 $XDG_CONFIG_HOME/gup/gup.conf。如果您想了解 [XDG 基目录规范](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)，请查看此链接。在您将 gup.conf 放置在另一个系统上的相同路径层次结构中之后，您执行 import 子命令。gup 根据 gup.conf 的内容开始安装。

```shell
※ 环境 A（例如 ubuntu）
$ gup export
Export /home/nao/.config/gup/gup.conf

※ 环境 B（例如 debian）
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```

或者，如果您使用 --output 选项，export 子命令会在 STDOUT 打印您想要导出的包信息（与 gup.conf 相同）。如果您使用 --input 选项，import 子命令也可以指定 gup.conf 文件路径。
```shell
※ 环境 A（例如 ubuntu）
$ gup export --output > gup.conf

※ 环境 B（例如 debian）
$ gup import --input=gup.conf
```

### 生成手册页（适用于 linux、mac）
man 子命令在 /usr/share/man/man1 下生成手册页。
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

### 生成 shell 补全文件（适用于 bash、zsh、fish）
completion 子命令为 bash、zsh 和 fish 生成 shell 补全文件。如果系统中不存在 shell 补全文件，生成过程将开始。要激活补全功能，请重新启动 shell。

```shell
$ gup completion
create bash-completion file: /home/nao/.bash_completion
create fish-completion file: /home/nao/.config/fish/completions/gup.fish
create zsh-completion file: /home/nao/.zsh/completion/_gup
```

### 桌面通知
如果您使用 --notify 选项运行 gup，gup 命令会在更新完成后通知您桌面更新是成功还是失败。
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)


## 贡献
首先，感谢您抽出时间来贡献！❤️ 更多信息请查看 [CONTRIBUTING.md](../../CONTRIBUTING.md)。
贡献不仅与开发相关。例如，GitHub Star 激励我进行开发！

### Star 历史记录
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

### 对于开发者
在添加新功能或修复错误时，请编写单元测试。如下面的单元测试树状图所示，sqly 对所有包都进行了单元测试。

![treemap](../img/cover-tree.svg)

## 联系
如果您想向开发者发送诸如"发现错误"或"请求附加功能"等评论，请使用以下联系方式之一。

- [GitHub Issue](https://github.com/nao1215/gup/issues)

您可以使用 bug-report 子命令发送错误报告。
```
$ gup bug-report
※ 通过您的默认浏览器打开 GitHub issue 页面
```

## 许可证
gup 项目根据 [Apache License 2.0](../../LICENSE) 的条款进行许可。


## 贡献者 ✨

感谢这些出色的人员（[表情符号键](https://allcontributors.org/docs/en/emoji-key)）：

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
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

这个项目遵循 [all-contributors](https://github.com/all-contributors/all-contributors) 规范。欢迎任何形式的贡献！
