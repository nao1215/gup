[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
# gupとは
**gup**コマンドは、"go install"でインストールしたバイナリを最新版にアップデートします。gupは、\$GOPATH/bin (\$GOBIN) 以下にあるバイナリをするためのサブコマンドも提供しています。
![sample](../img/sample.png)

gupコマンドはアップデートが終わった後、成功したか失敗したかをデスクトップ通知します。
![success](..//img/notify_success.png)
![warning](../img/notify_warning.png)
# インストール方法
### Step.1 前準備
現在は、" $ go install"によるインストールのみをサポートしています。そのため、golangの開発環境をシステムにインストールしていない場合、[golang公式サイト](https://go.dev/doc/install)からgolangをインストールしてください。

### Step2. インストール
```
$ go install github.com/nao1215/gup@latest
```

# 使用方法
### 全てのバイナリをアップデート
全てのバイナリをアップデートしたい場合は、`$ gup update`を実行してください。

```
$ gup update
gup:INFO : update all binary under $GOPATH/bin or $GOBIN
gup:INFO : [ 1/30] update success: github.com/cheat/cheat/cmd/cheat (Already up-to-date: v0.0.0-20211009161301-12ffa4cb5c87)
gup:INFO : [ 2/30] update success: fyne.io/fyne/v2/cmd/fyne_demo (Already up-to-date: v2.1.3)
gup:INFO : [ 3/30] update success: github.com/nao1215/gal/cmd/gal (v1.0.0 to v1.2.0)
gup:INFO : [ 4/30] update success: github.com/matsuyoshi30/germanium/cmd/germanium (Already up-to-date: v1.2.2)
gup:INFO : [ 5/30] update success: github.com/onsi/ginkgo/ginkgo (Already up-to-date: v1.16.5)
gup:INFO : [ 6/30] update success: github.com/git-chglog/git-chglog/cmd/git-chglog (Already up-to-date: v0.15.1)
   :
   :
```
### 指定バイナリのみアップデート
指定のバイナリのみを更新したい場合、updateサブコマンドに複数のコマンド名をスペース区切りで渡してください。
```
$ gup update subaru gup ubume
gup:INFO : update all binary under $GOPATH/bin or $GOBIN
gup:INFO : [1/3] update success: github.com/nao1215/gup (v0.7.0 to v0.7.1)
gup:INFO : [2/3] update success: github.com/nao1215/subaru (Already up-to-date: v1.0.2)
gup:INFO : [3/3] update success: github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1)
```
### $GOPATH/bin以下にあるバイナリ情報の一覧出力
listサブコマンドは、$GOPATH/bin（もしくは$GOBIN）以下にあるバイナリの情報を表示します。表示内容は、コマンド名、パッケージパス、コマンドバージョンです。

![sample](../img/list.png)

### 指定バイナリを削除
\$GOPATH/bin (\$GOBIN) 以下にあるバイナリを削除する場合は、remove サブコマンドを使用してください。remove サブコマンドは、削除前に削除してよいかどうかを確認します。
```
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
gup:INFO : removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
gup:INFO : cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
gup:INFO : removed /home/nao/.go/bin/ubume
```

確認無しで削除したい場合は, --forceオプションを使用してください。
```
$ gup remove --force gal
gup:INFO : removed /home/nao/.go/bin/gal
```

### バイナリが最新版かどうかのチェック
バイナリが最新版かどうかを知りたい場合は、checkサブコマンドを使用してください。checkサブコマンドは、バイナリが最新バージョンかどうかをチェックし、アップデートが必要なバイナリ名を表示します。しかし、更新はしません。
```
$ gup check
gup:INFO : check all binary under $GOPATH/bin or $GOBIN
gup:INFO : [ 1/33] check success: github.com/cheat/cheat (Already up-to-date: v0.0.0-20211009161301-12ffa4cb5c87)
gup:INFO : [ 2/33] check success: fyne.io/fyne/v2 (v2.1.3 to v2.1.4)
   :
gup:INFO : [33/33] check success: github.com/nao1215/ubume (Already up-to-date: v1.5.0)

gup:INFO : If you want to update binaries, the following command.
           $ gup update fyne_demo gup mimixbox 
```
  
他のサブコマンドと同様、指定のバイナリのみをチェックする事もできます。
```
$ gup check lazygit mimixbox
gup:INFO : check all binary under $GOPATH/bin or $GOBIN
gup:INFO : [1/2] check success: github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2)
gup:INFO : [2/2] check success: github.com/nao1215/mimixbox (v0.32.1 to v0.33.2)

gup:INFO : If you want to update binaries, the following command.
           $ gup update mimixbox 
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
