<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-15-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[English](../../README.md) | [日本語](../ja/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [Français](../fr/README.md)

<!-- gup:translation-sync -->
> 📖 これは翻訳版です。最新の情報は、正典である [English README](../../README.md) を参照してください（翻訳は英語版より遅れることがあります）。

# gup - "go install"でインストールしたバイナリを更新

![sample](../img/sample.gif)

**gup** コマンドは、"go install" でインストールしたバイナリを1つずつではなく並列で更新するため、非常に高速です。

gupは `$GOPATH/bin`（`$GOBIN`）配下のツールを管理するサブコマンドも提供します。インストール済みのバイナリを `list`／`check` し、`remove` で削除し、`export`／`import` でツール一式を別マシンに再現し、`migrate` で別の `$GOBIN` へ移行できます。Windows、Mac、Linuxで動作します。

oh-my-zshを使用している場合、gupにはエイリアスが設定されています。そのエイリアスは `gup - git pull --rebase` です。そのため、oh-my-zshのエイリアスを無効にして使用してください（例：$ \gup update）。

## ベンチマーク
gupは更新を並列実行するため、バイナリを1つずつ更新するツールよりも短時間で完了します。新しいバージョンが利用可能な9個のバイナリを更新した場合:

| ツール | 方式 | 時間 |
|------|----------|-----:|
| gup update | 並列 | 0.7s |
| [go-global-update](https://github.com/Gelio/go-global-update) | 逐次 | 2.9s |
| `go install` ループ | 逐次 | 2.9s |

計測環境: AMD Ryzen AI Max+ 395（32コア）/ 64 GB RAM / Ubuntu 26.04 / go 1.26.4。ウォームな Go モジュールキャッシュで5回計測した中央値です。時間は各バイナリのビルド時間とCPUに依存します。

## 機能比較

| 機能 | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| 並列更新 | はい | いいえ | 手動 |
| パッケージごとの更新チャネル（`latest`/`main`/`master`） | はい | いいえ | 手動 |
| ツール一式のエクスポート／インポート | はい | いいえ | 手動 |
| 新しい `$GOBIN` へのバイナリ移行 | はい | いいえ | 手動 |
| 機械可読なJSON出力（`--json`） | はい | いいえ | いいえ |
| シェル補完の生成／インストール | はい | いいえ | いいえ |
| `update` が最新状態のバイナリを再インストール | いいえ | はい | はい |
| 対象が既に存在する場合に `migrate --force` で再インストール | はい | いいえ | 手動 |
| 失敗時の診断／次の手順のヒント | いいえ | はい | いいえ |
| `NO_COLOR` 対応 | はい | はい | — |
| 追加ツールが不要（公式ツールチェインのみ） | いいえ | いいえ | はい |

## サポート対象OS（GitHub Actionsでユニットテスト実施）
- Linux
- Mac
- Windows

## インストール方法
gup は `go install` と Homebrew に加えて、`winget`、`mise`、`nix` からもインストールできます。

### "go install"を使用
システムにGolang開発環境がインストールされていない場合は、[Golang公式サイト](https://go.dev/doc/install)からGolangをインストールしてください。
```
go install github.com/nao1215/gup@latest
```

### homebrewを使用
```shell
brew install nao1215/tap/gup
```

### wingetを使用（Windows）
```shell
winget install --id nao1215.gup
```

### mise-en-placeを使用
```shell
mise use -g gup@latest
```

### nixを使用（Nix profile）
```shell
nix profile install nixpkgs#gogup
```

### パッケージまたはバイナリからインストール
[リリースページ](https://github.com/nao1215/gup/releases) には、.deb、.rpm、.apk形式のパッケージが含まれています。gupコマンドは内部的にgoコマンドを使用するため、Golangのインストールが必要です。

## リリースの完全性を検証する
各リリースには、ダウンロードしたものを検証できるようにサプライチェーンのメタデータが添付されています:

- **署名付きチェックサム** — `checksums.txt` は [cosign](https://github.com/sigstore/cosign)（keyless）で署名され、`checksums.txt.sig` と `checksums.txt.pem` が生成されます。
- **SBOM** — 各リリースアーカイブに SPDX 形式のソフトウェア部品表（SBOM）が添付されます。
- **ビルドプロベナンス** — GitHub OIDC を用いて SLSA ビルドプロベナンスが付与されます。

署名付きチェックサムを検証する（その後アーカイブを `checksums.txt` と照合する）:

```shell
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

GitHub CLI でダウンロードしたアーティファクトのビルドプロベナンスを検証する:

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

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

### @main、@master、@latestでバイナリを更新
バイナリごとに更新元を指定したい場合、以下のオプションを使用します。
- `--main` (`-m`): `@main` で更新（リポジトリに `main` ブランチが無い場合のみ `@master` にフォールバック）
- `--master`: `@master` で更新
- `--latest`: `@latest` で更新

`@main` → `@master` のフォールバックは `main` ブランチが存在しない場合に限られます。ビルド・ネットワーク・認証・タイムアウト・キャンセルによる失敗はそのまま報告され、暗黙に `@master` をインストールすることはありません。

選択したチャネルは `gup.json` に保存され、次回以降の `gup update` でも再利用されます。
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
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

### 大量のツールに対する出力の抑制
`check` と `update` はデフォルトですべてのバイナリを表示するため、多数のツールをインストールしている場合は出力が煩雑になります。`--quiet`（`-q`）を渡すと、最新状態の行を抑制し、更新されたバイナリ（または更新可能なバイナリ）と失敗のみを表示し、最後に1行のサマリを出力します。エラーは常にSTDERRに書き出されるため、必ず表示されます。`--json` も同時に指定された場合、`--quiet` は無視され、完全なJSON配列が出力されます。
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

### 機械可読なJSON出力（スクリプト／CI向け）
`list`、`check`、`update` は `--json` を受け付け、人間向けの出力の代わりにJSON配列を出力します（デフォルトは従来どおり人間向け出力）。

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

各要素のフィールドは次のとおりです: `name`、`import_path`、`module_path`、`channel`（`latest`／`main`／`master`）、`current_version`、`latest_version`（`list` では空）、`current_go_version`、`installed_go_version`、`status`、`error`（無い場合は省略）。`status` は `installed`（list）、`up-to-date`、`update-available`（check）、`updated`（update）、`error` のいずれかです。

部分的な失敗を含めて出力は常に valid な JSON です（失敗したパッケージは `"status": "error"` になり、エラー詳細はSTDERRに出るためSTDOUTはJSONのみに保たれます）。終了コードは従来と同じで、`check` が `update-available` を報告しても終了コードは `0` のままです。

### 空の環境での挙動
グローバル環境が空（`go install` でインストールしたバイナリがまだ1つも無い状態）の場合は、エラーではなく通常の初回実行として扱われます。

- `list`、`check`、`update` は終了コード `0` で短い案内を表示します（`--json` 指定時は valid な空配列 `[]`）。
- `export` は終了コード `0` で空の `gup.json` を書き出します。

ただし、インストールされていないバイナリ名を指定した場合や、すべてのバイナリを除外した場合は、従来どおり使い方の誤りとして終了コード `1` になります。

### Export／Importサブコマンド
複数のシステム間で同じGolangバイナリをインストールしたい場合は、export／importサブコマンドを使用します。
`gup.json` は import path、バイナリバージョン、更新チャネル（`latest` / `main` / `master`）を保存し、`import` はそのバージョンをそのままインストールします。

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

デフォルトでは次の挙動です。
- `gup export` は `$XDG_CONFIG_HOME/gup/gup.json` に書き出します。
- `gup import`・`gup check`・`gup update` は設定ファイルを次の順で自動検出します。
  1) `$XDG_CONFIG_HOME/gup/gup.json`（存在する場合）
  2) `./gup.json`（存在する場合）

ユーザーレベルの `gup.json` と `./gup.json` の**両方**が存在する場合、`import`・`check`・`update` はどちらか一方を暗黙に選ばず、`--file` での明示を促してすぐにエラー終了します。`--file` (`-f`) でパスを上書き指定できます。

`gup export` は保存済みの更新チャネルを常に正規のユーザーレベル `gup.json` から解決します。`--file`/`--output` は書き出し先を変えるだけなので、新しいファイルへエクスポートしてもパッケージのチャネルが `latest` に戻ることはありません。

```shell
※ 環境A (例: ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.json

※ 環境B (例: debian)
$ gup import
```

また、exportサブコマンドは `--output` オプションを使用すると、`gup.json` と同じ内容をSTDOUTに出力できます。importサブコマンドは `--file` オプションを使用して読み込みファイルを指定できます。
```shell
※ 環境A (例: ubuntu)
$ gup export --output > gup.json

※ 環境B (例: debian)
$ gup import --file=gup.json
```

### 新しい$GOBINへのバイナリ移行

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` は `BEFORE_PATH` 配下のGoバイナリを、各バイナリのビルド情報に記録された `import path@version` のまま `AFTER_PATH` に再インストールします（暗黙に `@latest` へ更新することはありません）。内部的には `GOBIN` を `AFTER_PATH` に設定して通常の `go install` を実行するだけなので、現在使用中のGoツールチェインで再ビルドされます。

#### これが役立つ場面（例: `mise`）

[`mise`](https://mise.jdx.dev/) でGoを管理している場合、Goを更新するとGoのバージョンごとに `$GOBIN` の実体パスが変わることがあります。その結果、以前の `$GOBIN` 配下にインストールしたツールが新しいGoから見えなくなります。`gup migrate` を使うと、古い `$GOBIN` から新しい `$GOBIN` へ同じツール一式を再インストールできます:

```shell
# 古いGOBINのすべてのgo-installツールを新しいGOBINへ再インストール
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# 特定のバイナリだけを移行
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` は追加のみを行います:

- `AFTER_PATH` 内のファイルを削除・整理することはありません。
- `AFTER_PATH` に既に存在するバイナリはデフォルトでスキップされます。上書き再インストールするには `--force` を使用します。
- `AFTER_PATH` が存在しない場合は自動的に作成されます。
- `BEFORE_PATH` と `AFTER_PATH` は異なるディレクトリである必要があります。

import path や version を解決できないバイナリ、および開発ビルド（`devel` / `(devel)`）は、更新されずにスキップされます。これにより、ローカルビルドや再現不能なビルドが壊れることはありません。

対応フラグ: `--dry-run`（`-n`）、`--notify`（`-N`）、`--jobs`（`-j`）、`--force`。

### manページの生成（LinuxとMac用）
manサブコマンドはデフォルトで `/usr/share/man/man1` 配下にmanページを生成します。`MANPATH` が設定されている場合は、その各エントリ配下の `man1` ディレクトリに書き出し、存在しなければ作成します。書き込めない出力先の場合は明確なエラーを表示して終了します。
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

### シェル補完ファイルの生成（bash、zsh、fish、PowerShell用）
`completion` はシェル名を引数で指定すると、標準出力に補完スクリプトを出力します。
bash/fish/zsh の補完ファイルをユーザー環境にインストールするには、`--install` を使用します。
PowerShell は出力を `.ps1` ファイルにリダイレクトし、profile から読み込んでください。

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# 既定のユーザーパスに補完ファイルを自動インストール
$ gup completion --install
```

`--install` は `HOME` が設定されている必要があります。`HOME` が空の場合は（カレントディレクトリにファイルを書き出すことなく）即座に失敗し、いずれかの補完ファイルを書き込めなかった場合は非ゼロで終了します。

### デスクトップ通知
--notifyオプションでgupを使用すると、更新完了後にgupコマンドがデスクトップで更新の成功・失敗を通知します。
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)

### カラー出力を無効化する
gupはデフォルトで出力に色を付けます。色を無効にするには、`--no-color` を渡すか、環境変数 `NO_COLOR` を空でない値に設定してください（[NO_COLOR](https://no-color.org/) の慣習に従っています）。出力をパイプする場合、CIのログ、または `NO_COLOR` をグローバルに設定している場合に便利です。
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## 貢献
まず、貢献に時間を割いていただき、ありがとうございます！詳細については、[CONTRIBUTING.md](../../CONTRIBUTING.md)をご覧ください。
開発フロー、品質チェックリスト、ツール管理の手順は [CONTRIBUTING.md](../../CONTRIBUTING.md) に記載しています。
貢献は開発に関連するものだけではありません。たとえば、GitHub Starは開発のモチベーションになります！

### Star履歴
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

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
      <td align="center" valign="top" width="14.28%"><a href="https://mochaa.ws/?utm_source=github_user"><img src="https://avatars.githubusercontent.com/u/21154023?v=4?s=100" width="100px;" alt="Zephyr Lykos"/><br /><sub><b>Zephyr Lykos</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=mochaaP" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://itrooz.fr"><img src="https://avatars.githubusercontent.com/u/42669835?v=4?s=100" width="100px;" alt="iTrooz"/><br /><sub><b>iTrooz</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=iTrooz" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://pacman.blog.br"><img src="https://avatars.githubusercontent.com/u/59438?v=4?s=100" width="100px;" alt="Tiago Peczenyj"/><br /><sub><b>Tiago Peczenyj</b></sub></a><br /><a href="https://github.com/nao1215/gup/commits?author=peczenyj" title="Code">💻</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

このプロジェクトは[all-contributors](https://github.com/all-contributors/all-contributors)仕様に従っています。どのような種類の貢献も歓迎します！
