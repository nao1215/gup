[![Build](https://github.com/nao1215/gup/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/build.yml)
[![UnitTest](https://github.com/nao1215/gup/actions/workflows/unit_test.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/unit_test.yml)
# gup - Update binaries installed by "go install"
**gup** command update binaries installed by "go install" to the latest version. The gup command saves the command's package path (that is, \<PATH\> in `$ go install <PATH>`) in the configuration file.  

# How to install
### Step.1 Install golang
gup command only supports installation with `$ go install`. If you does not have the golang development environment installed on your system, please install golang from the [golang official website] (https://go.dev/doc/install).

### Step2. Install gup
```
$ go install github.com/nao1215/gup@latest
```
# How to use
### Update all binaries
If you update all binaries, you just run `$ gup`.  After executing the gup command, a configuration file is automatically created in `$HOME/.config/gup/gup.conf`.

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

### Import binary from configuration file
If "$ gup" is successful, `$HOME/.config/gup/gup.conf` has been generated. gup.conf has the settings in the `$BINARY_NAME = $PATH` format. After copying gup.conf from one environment to another, run "gup import" in another environment to install the binaries according to gup.conf.
```
$ gup import
```

# Contact
If you would like to send comments such as "find a bug" or "request for additional features" to the developer, please use one of the following contacts.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

# LICENSE
The gup project is licensed under the terms of the Apache License 2.0.
