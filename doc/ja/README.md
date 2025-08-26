<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-10-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![gosec](https://github.com/nao1215/gup/actions/workflows/security.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[日本語](./README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

# gup - "go install"でインストールしたバイナリを更新

![sample](../img/sample.png)

**gup** コマンドは、"go install" でインストールしたバイナリを最新版に更新します。gupはすべてのバイナリを並列で更新するため、非常に高速です。また、\$GOPATH/bin (\$GOBIN) 配下のバイナリを操作するためのサブコマンドも提供しています。Windows、Mac、Linuxで動作するクロスプラットフォーム対応のソフトウェアです。

oh-my-zshを使用している場合、gupにはエイリアスが設定されています。そのエイリアスは `gup - git pull --rebase` です。そのため、oh-my-zshのエイリアスを無効にして使用してください（例：$ \gup update）。


## サポート対象OS（GitHub Actionsでユニットテスト実施）
- Linux
- Mac
- Windows

## インストール方法
### "go install"を使用
システムにGolang開発環境がインストールされていない場合は、[Golang公式サイト](https://go.dev/doc/install)からGolangをインストールしてください。
```
go install github.com/nao1215/gup@latest
```

### homebrewを使用
```shell
brew install nao1215/tap/gup
```

### パッケージまたはバイナリからインストール
[リリースページ](https://github.com/nao1215/gup/releases) には、.deb、.rpm、.apk形式のパッケージが含まれています。gupコマンドは内部的にgoコマンドを使用するため、Golangのインストールが必要です。


## 使用方法
### すべてのバイナリを更新
すべてのバイナリを更新する場合は、`$ gup update` を実行するだけです。

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

### 指定したバイナリのみ更新
指定したバイナリのみを更新したい場合は、複数のコマンド名をスペース区切りで指定します。
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### gup update実行時にバイナリを除外
一部のバイナリを更新したくない場合は、スペースなしで「,」区切りを使用して、更新すべきでないバイナリを指定してください。
--dry-run オプションとの組み合わせでも動作します。
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude または -e、この例では 'gopls' と 'golangci-lint' を除外します
```

### @mainまたは@masterでバイナリを更新
@masterや@mainでバイナリを更新したい場合は、-mまたは--masterオプションを指定できます。
```shell
$ gup update --main=gup,lazygit,sqly
```

### $GOPATH/bin配下のコマンド名とパッケージパス、バージョンを一覧表示
listサブコマンドは$GOPATH/binまたは$GOBIN配下のコマンド情報を出力します。出力される情報は、コマンド名、パッケージパス、コマンドバージョンです。
![sample](../img/list.png)

### 指定したバイナリを削除
$GOPATH/binまたは$GOBIN配下のコマンドを削除したい場合は、removeサブコマンドを使用します。removeサブコマンドは削除前に確認を行います。
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

強制的に削除したい場合は、--forceオプションを使用してください。
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

### バイナリが最新バージョンかチェック
バイナリが最新バージョンかどうかを知りたい場合は、checkサブコマンドを使用してください。checkサブコマンドはバイナリが最新バージョンかどうかをチェックし、更新が必要なバイナリの名前を表示します。
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

他のサブコマンドと同様、指定したバイナリのみをチェックすることもできます。
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
          $ gup update mimixbox
```
### Export／Importサブコマンド
複数のシステム間で同じGolangバイナリをインストールしたい場合は、export／importサブコマンドを使用します。デフォルトで、exportサブコマンドは$XDG_CONFIG_HOME/gup/gup.confにファイルをエクスポートします。[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)について知りたい場合は、このリンクを参照してください。別のシステムの同じパス階層にgup.confを配置した後、importサブコマンドを実行します。gupはgup.confの内容に従ってインストールを開始します。

```shell
※ 環境A (例: ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.conf

※ 環境B (例: debian)
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```

また、exportサブコマンドは--outputオプションを使用すると、エクスポートしたいパッケージ情報（gup.confと同じ内容）をSTDOUTに出力できます。importサブコマンドも--inputオプションを使用してgup.confファイルパスを指定できます。
```shell
※ 環境A (例: ubuntu)
$ gup export --output > gup.conf

※ 環境B (例: debian)
$ gup import --input=gup.conf
```

### manページの生成（LinuxとMac用）
manサブコマンドは/usr/share/man/man1配下にmanページを生成します。
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

### シェル補完ファイルの生成（bash、zsh、fish用）
completionサブコマンドはbash、zsh、fish用のシェル補完ファイルを生成します。システムにシェル補完ファイルが存在しない場合、生成処理が開始されます。補完機能を有効にするには、シェルを再起動してください。

```shell
$ gup completion
create bash-completion file: /home/nao/.bash_completion
create fish-completion file: /home/nao/.config/fish/completions/gup.fish
create zsh-completion file: /home/nao/.zsh/completion/_gup
```

### デスクトップ通知
--notifyオプションでgupを使用すると、更新完了後にgupコマンドがデスクトップで更新の成功・失敗を通知します。
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)


## 貢献
まず、貢献に時間を割いていただき、ありがとうございます！詳細については、[CONTRIBUTING.md](../../CONTRIBUTING.md)をご覧ください。
貢献は開発に関連するものだけではありません。たとえば、GitHub Starは開発のモチベーションになります！

### Star履歴
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

### 開発者向け
新機能の追加やバグ修正を行う際は、ユニットテストを書いてください。sqlyは以下のユニットテストツリーマップが示すように、すべてのパッケージに対してユニットテストが実施されています。

![treemap](../img/cover-tree.svg)

## 連絡先
「バグを見つけた」や「追加機能のリクエスト」などのコメントを開発者に送りたい場合は、以下の連絡先をご利用ください。

- [GitHub Issue](https://github.com/nao1215/gup/issues)

bug-reportサブコマンドを使用してバグレポートを送信できます。
```
$ gup bug-report
※ デフォルトブラウザでGitHub issueページを開きます
```

## ライセンス
gupプロジェクトは[Apache License 2.0](../../LICENSE)の条件の下でライセンスされています。


## コントリビューター ✨

これらの素晴らしい人々に感謝します（[emoji key](https://allcontributors.org/docs/en/emoji-key)）：

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
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

このプロジェクトは[all-contributors](https://github.com/all-contributors/all-contributors)仕様に従っています。どのような種類の貢献も歓迎します！