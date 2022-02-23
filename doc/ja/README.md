[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
# gupとは
**gup**コマンドは、"go install"でインストールしたバイナリを最新版にアップデートします。
![sample](../img/sample.png)
# インストール方法
### Step.1 前準備
現在は、" $ go install"によるインストールのみをサポートしています。そのため、golangの開発環境をシステムにインストールしていない場合、[golang公式サイト](https://go.dev/doc/install)からgolangをインストールしてください。

### Step2. インストール
```
$ go install github.com/nao1215/gup@latest
```

# 使用方法
### 全てのバイナリをアップデート
全てのバイナリをアップデートしたい場合は、`$ gup`を実行してください。

```
$ gup
29 / 29 [--------------------------------------------------------------] 100.00%
gup:INFO: update success: github.com/nao1215/goavl
gup:INFO: update success: github.com/uudashr/gopkgs/v2/cmd/gopkgs
gup:INFO: update success: github.com/nao1215/gup
gup:INFO: update success: golang.org/x/tools/cmd/gorename
gup:INFO: update success: github.com/nao1215/speaker/cmd/speaker
gup:INFO: update success: github.com/git-chglog/git-chglog/cmd/git-chglog
gup:INFO: update success: github.com/haya14busa/goplay/cmd/goplay
gup:INFO: update success: github.com/pborzenkov/goupdate
gup:INFO: update success: github.com/skanehira/pst
gup:INFO: update success: github.com/google/go-licenses
gup:INFO: update success: github.com/furusax0621/go-nabeatsu/cmd/nabeatsu
gup:INFO: update success: github.com/cheat/cheat/cmd/cheat
gup:INFO: update success: github.com/onsi/ginkgo/ginkgo
gup:INFO: update success: github.com/nao1215/mimixbox/cmd/mimixbox
gup:INFO: update success: github.com/nao1215/subaru
gup:INFO: update success: github.com/nao1215/ubume/cmd/ubume
gup:INFO: update success: github.com/nao1215/gal/cmd/gal
gup:INFO: update success: github.com/ramya-rao-a/go-outline
gup:INFO: update success: github.com/Songmu/gocredits/cmd/gocredits
gup:INFO: update success: github.com/kemokemo/gomrepo
gup:INFO: update success: golang.org/x/tools/gopls
gup:INFO: update success: github.com/josharian/impl
gup:INFO: update success: github.com/shogo82148/goa-v1/goagen
gup:INFO: update success: github.com/fatih/gomodifytags
gup:INFO: update success: github.com/cweill/gotests/gotests
gup:INFO: update success: fyne.io/fyne/v2/cmd/fyne_demo
gup:INFO: update success: github.com/jesseduffield/lazygit
gup:INFO: update success: github.com/mgechev/revive
gup:INFO: update success: honnef.co/go/tools/cmd/staticcheck
```

### $GOPATH/bin以下にあるバイナリ情報の一覧出力
listサブコマンドは、$GOPATH/bin（もしくは$GOBIN）以下にあるバイナリの情報を表示します。表示内容は、コマンド名、パッケージパス、コマンドバージョンです。

![sample](../img/list.png)

### 指定バイナリのみアップデート
指定のバイナリのみを更新したい場合、--fileオプションを使用してください。--fileオプションでは、複数のコマンド名をカンマ区切りで指定できます。
```
$ gup --file=subaru,gup,ubume
3 / 3 [----------------------------------------------------------------] 100.00%
gup:INFO: update success: github.com/nao1215/gup
gup:INFO: update success: github.com/nao1215/subaru
gup:INFO: update success: github.com/nao1215/ubume/cmd/ubume
```

### エクスポート／インポートサブコマンド
複数のシステム間で、$GOPATH/bin（もしくは$GOBIN）以下にあるバイナリを揃えたい場合、export／importサブコマンドを使ってください。exportサブコマンドは、$HOME/.config/gup/gup.confファイルを生成し、このファイル内にはシステムにインストール済みのコマンド情報が記載されています。  
別のシステム環境に$HOME/.config/gup/gup.confファイルを同じ階層にコピーした後、importサブコマンドを実行してください。gupコマンドは、gup.confの内容に従ってインストールを開始します。
```
※ 環境A (e.g. ubuntu)
$ gup export
gup:INFO: Export /home/nao/.config/gup/gup.conf

※ 環境B (e.g. debian)
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```
# 連絡先
開発者に対して「バグ報告」や「機能の追加要望」がある場合は、コメントをください。その際、以下の連絡先を使用してください。
- [GitHub Issue](https://github.com/nao1215/gup/issues)

# ライセンス
gupプロジェクトは、[Apache License 2.0条文](./../../LICENSE)の下でライセンスされています。
