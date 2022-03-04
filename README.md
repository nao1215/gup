[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)  
[[日本語](./doc/ja/README.md)]  
# gup - Update binaries installed by "go install"
**gup** command update binaries installed by "go install" to the latest version. It also provides subcommands for manipulating binaries under \$GOPATH/bin (\$GOBIN).
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
If you update all binaries, you just run `$ gup update`. 

```
$ gup update
gup:INFO : update all binary under $GOPATH/bin or $GOBIN
gup:INFO : [ 1/29] update success: github.com/cheat/cheat/cmd/cheat
gup:INFO : [ 2/29] update success: fyne.io/fyne/v2/cmd/fyne_demo
gup:INFO : [ 3/29] update success: github.com/nao1215/gal/cmd/gal
gup:INFO : [ 4/29] update success: github.com/matsuyoshi30/germanium/cmd/germanium
gup:INFO : [ 5/29] update success: github.com/onsi/ginkgo/ginkgo
gup:INFO : [ 6/29] update success: github.com/git-chglog/git-chglog/cmd/git-chglog
gup:INFO : [ 7/29] update success: github.com/ramya-rao-a/go-outline
gup:INFO : [ 8/29] update success: github.com/shogo82148/goa-v1/goagen
   :
   :
```

### Update the specified binary
If you want to update only the specified binaries, you specify multiple command names separated by space.
```
$ gup update subaru gup ubume
gup:INFO : update all binary under $GOPATH/bin or $GOBIN
gup:INFO : [1/3] update success: github.com/nao1215/gup
gup:INFO : [2/3] update success: github.com/nao1215/subaru
gup:INFO : [3/3] update success: github.com/nao1215/ubume/cmd/ubume
```

### List up command name with package path and version under $GOPATH/bin
list subcommand print command information under $GOPATH/bin or $GOBIN. The output information is the command name, package path, and command version.
![sample](doc/img/list.png)

### Remove the specified binary
If you want to remove a command under $GOPATH/bin or $GOBIN, use the remove subcommand. The remove subcommand asks if you want to remove it before removing it.
```
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
gup:INFO : removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
gup:INFO : cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
gup:INFO : removed /home/nao/.go/bin/ubume
```

If you want to force the removal, use the --force option.
```
$ gup remove --force gal
gup:INFO : removed /home/nao/.go/bin/gal
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
