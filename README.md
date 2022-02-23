[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)  
[[日本語](./doc/ja/README.md)]  
# gup - Update binaries installed by "go install"
**gup** command update binaries installed by "go install" to the latest version.
![sample](./doc/img/sample.png)
# How to install
### Step.1 Install golang
gup command only supports installation with `$ go install`. If you does not have the golang development environment installed on your system, please install golang from the [golang official website](https://go.dev/doc/install).

### Step2. Install gup
```
$ go install github.com/nao1215/gup@latest
```
# How to use
### Update all binaries
If you update all binaries, you just run `$ gup`. 

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

### List up command name with package path and version under $GOPATH/bin
list subcommand print command information under $GOPATH/bin or $GOBIN. The output information is the command name, package path, and command version.
![sample](doc/img/list.png)
### Update the specified binary
If you want to update only the specified binaries, use the --file option. You specify multiple command names separated by commas.
```
$ gup --file=subaru,gup,ubume
3 / 3 [----------------------------------------------------------------] 100.00%
gup:INFO: update success: github.com/nao1215/gup
gup:INFO: update success: github.com/nao1215/subaru
gup:INFO: update success: github.com/nao1215/ubume/cmd/ubume
```
### Export／Import subcommand
You use the export／import subcommand if you want to install the same golang binaries across multiple systems. By default, export-subcommand exports the file to $HOME/.config/gup/gup.conf. After you have placed gup.conf in the same path hierarchy on another system, you execute import-subcommand. gup start the installation 
according to the contents of gup.conf.

```
※ Environmet A (e.g. ubuntu)
$ gup export
gup:INFO: Export /home/nao/.config/gup/gup.conf

※ Environmet B (e.g. debian)
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```

# Contact
If you would like to send comments such as "find a bug" or "request for additional features" to the developer, please use one of the following contacts.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

# LICENSE
The gup project is licensed under the terms of [the Apache License 2.0](./LICENSE).
