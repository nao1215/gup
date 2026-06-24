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
> 📖 本文档为翻译版本，可能落后于作为权威来源的 [英文 README](../../README.md)。

# gup - 更新通过"go install"安装的二进制文件

![sample](../img/sample.gif)

gup 更新并管理您 `$GOBIN` 中全局的 Go 命令行工具。`go install` 会把每个程序放入 `$GOBIN`（`$GOPATH/bin`），但之后再也不会更新它；gup 则并行地将整套工具一次性更新到最新。它还补齐了 `go install` 所缺少的管理命令：`list`/`check` 已安装的内容、`remove` 二进制文件、`export`/`import` 整套工具以便在另一台机器上重现它，以及把它 `migrate` 到新的 `$GOBIN`。可在 Windows、macOS 和 Linux 上运行。

## 支持的操作系统（通过 GitHub Actions 进行单元测试）
- Linux
- Mac
- Windows

## 如何安装
除 `go install` 和 Homebrew 外，gup 也已可通过 `winget`、`mise` 和 `nix` 安装。

### 使用"go install"
如果您的系统上没有安装 golang 开发环境，请从 [golang 官方网站](https://go.dev/doc/install)安装 golang。
```
go install github.com/nao1215/gup@latest
```
从源码构建需要 Go 1.25 或更新版本。如果使用较旧的 Go，请改为安装预编译的发布二进制文件或软件包（见下文）。

### 使用 homebrew
```shell
brew install nao1215/tap/gup
```

### 使用 winget（Windows）
```shell
winget install --id nao1215.gup
```

### 使用 mise-en-place
```shell
mise use -g gup@latest
```

### 使用 nix（Nix profile）
```shell
nix profile install nixpkgs#gogup
```

### 从包或二进制文件安装
[发布页面](https://github.com/nao1215/gup/releases)包含 .deb、.rpm 和 .apk 格式的包。gup 命令内部使用 go 命令，因此需要安装 golang。

## 验证发布完整性
每个发布版本都附带供应链元数据，以便您验证所下载的内容：

- 已签名的校验和：`checksums.txt` 使用 [cosign](https://github.com/sigstore/cosign)（无密钥）签名，生成 `checksums.txt.sigstore.json`。
- SBOM：每个发布归档都附有 SPDX 软件物料清单（Software Bill of Materials）。
- 构建溯源：通过 GitHub OIDC 对 SLSA 构建溯源进行证明。

验证已签名的校验和（然后将您的归档与 `checksums.txt` 进行核对）：

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

使用 GitHub CLI 验证已下载工件的构建溯源：

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

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

### 使用 @main、@master 或 @latest 更新二进制文件
如果您想按二进制文件控制更新来源，可以使用以下选项：
- `--main` (`-m`)：使用 `@main` 更新（失败时回退到 `@master`）
- `--master`：使用 `@master` 更新
- `--latest`：使用 `@latest` 更新

所选通道会保存到 `gup.json`，并在后续 `gup update` 中复用。
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
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

在非交互式执行中（当 stdin 不是 TTY 时，例如 CI 或管道），`gup remove` 不再阻塞等待确认。它会快速失败并给出清晰的提示；传入 `--force` 可在不确认的情况下移除。

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

### 大量工具时的简洁输出
默认情况下，`check` 和 `update` 会打印每个二进制文件，当您安装了许多工具时这会很嘈杂。传入 `--quiet`（`-q`）可以抑制"已是最新"的行，只显示已更新（或有可用更新）的二进制文件以及失败项，最后附上一行汇总。错误始终写入 STDERR，因此它们会保持可见。当同时给出 `--json` 时，`--quiet` 会被忽略，并打印完整的 JSON 数组。
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

### 机器可读的 JSON 输出（用于脚本 / CI）
`list`、`check` 和 `update` 接受 `--json`，会打印一个 JSON 数组，而不是人类可读的输出（人类可读输出仍是默认）。

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

每个元素都有以下字段：`name`、`import_path`、`module_path`、`channel`（`latest`/`main`/`master`）、`current_version`、`latest_version`（`list` 时为空）、`current_go_version`、`installed_go_version`、`status`、`error`（不存在时省略），以及 `hint`（后续步骤建议，仅当某条建议适用于该错误时才出现）。`status` 可以是 `installed`（list）、`up-to-date`、`update-available`（check）、`updated`（update）或 `error`。

该数组始终是有效的 JSON，包括部分失败的情况（那些包会得到 `"status": "error"`；错误详情也会写入 STDERR，因此 STDOUT 保持为纯 JSON）。退出码保持不变——`check` 报告 `update-available` 时仍然以 `0` 退出。

### 失败诊断 / 后续步骤提示
当 `update` 或 `check` 失败时，gup 会把 Go 工具链晦涩难懂的输出转换成一条简短、可操作的后续步骤，并在错误之后立即打印到 STDERR（使用 `--json` 时则作为 `hint` 字段暴露）：

```shell
$ gup update
gup:ERROR: [1/1] tool: can't install gup.test/moved/cmd/tool:
go: gup.test/moved/cmd/tool@latest: module gup.test/moved@latest found (v1.1.0), but does not contain package gup.test/moved/cmd/tool
gup:HINT : The module no longer provides this command at its import path. The project likely moved to a new major version (e.g. a `/v2` module path) or relocated the command; check its current install instructions and reinstall with the new path.
```

提示涵盖模块重命名/大版本迁移、命令位置变更、`go.mod` 的 `replace` 指令、并非通过 `go install` 安装的二进制文件、缺失的分支/标签、无法解析/私有/已删除的仓库、权限和网络错误，以及过期的 Go 工具链。当 gup 没有可靠的内容可补充时（例如超时，其消息本身已说明补救办法），它会保持沉默。

### 空环境下的行为
空的全局环境（尚未通过 `go install` 安装任何二进制文件）被视为正常的首次运行情况，而非错误：

- `list`、`check` 和 `update` 以 `0` 退出，并打印一条简短的提示信息（使用 `--json` 时为有效的空数组 `[]`）。
- `export` 以 `0` 退出并写入一个空的 `gup.json`。

指定一个未安装的二进制文件，或排除所有二进制文件，仍然是使用错误并以 `1` 退出。

### 导出/导入子命令
如果您想在多个系统中安装相同的 golang 二进制文件，可以使用 export/import 子命令。
`gup.json` 保存 import path、二进制版本和更新通道（`latest` / `main` / `master`）。
`import` 会按文件中记录的版本进行安装。

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

默认行为：
- `gup export` 写入 `$XDG_CONFIG_HOME/gup/gup.json`
- `gup import`、`gup check` 和 `gup update` 按以下顺序自动检测配置文件路径：
  1) `$XDG_CONFIG_HOME/gup/gup.json`（存在时）
  2) `./gup.json`（存在时）

如果用户级 `gup.json` 和 `./gup.json` 同时存在，`import`、`check`、`update` 和 `list --json` 会立即失败并要求您使用 `--file` 消除歧义，而不会静默地选择其中一个。您始终可以通过 `--file`（`-f`）覆盖路径；`list` 可以将 `--file` 与 `--json` 一起使用，以选择提供所报告 `channel` 的配置。

格式错误的 `gup.json`（无效的 JSON 或不受支持的 `schema_version`）也会被视为错误，而不是静默忽略：`check`、`update` 和 `export` 会立即失败并指出有问题的文件，因此已保存的按包通道绝不会因为配置无法解析而被悄悄降级为 `latest`。

`gup export` 始终从规范的用户级 `gup.json` 解析已保存的更新通道；`--file`/`--output` 只改变导出目标，因此导出到新文件绝不会将软件包的通道重置为 `latest`。

```shell
※ 环境 A（例如 ubuntu）
$ gup export
Export /home/nao/.config/gup/gup.json

※ 环境 B（例如 debian）
$ gup import
```

或者，`export` 可通过 `--output` 将 `gup.json` 内容输出到 STDOUT，`import` 可通过 `--file` 指定读取文件。
```shell
※ 环境 A（例如 ubuntu）
$ gup export --output > gup.json

※ 环境 B（例如 debian）
$ gup import --file=gup.json
```

### 将二进制文件迁移到新的 $GOBIN

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` 会将 `BEFORE_PATH` 下的 Go 二进制文件重新安装到 `AFTER_PATH`，使用每个二进制文件构建信息中记录的精确 `import path@version`（它绝不会悄悄升级到 `@latest`）。在内部，它只是将 `GOBIN` 设置为 `AFTER_PATH` 并运行常规的 `go install` 流程，因此这些二进制文件会使用当前正在使用的 Go 工具链重新构建。

#### 为什么这很有用（例如配合 `mise`）

当您使用 [`mise`](https://mise.jdx.dev/) 管理 Go 时，更新 Go 可能会改变每个 Go 版本对应的 `$GOBIN` 实际路径。结果，您在之前的 `$GOBIN` 下安装的工具对新的 Go 不再可见。`gup migrate` 让您可以将相同的 Go 工具集从旧的 `$GOBIN` 重新安装到新的 `$GOBIN`：

```shell
# 将所有 go-install 工具从旧的 GOBIN 重新安装到新的 GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# 仅迁移特定的二进制文件
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` 是仅追加的：

- 它绝不会删除或清理 `AFTER_PATH` 中的文件。
- `AFTER_PATH` 中已存在的二进制文件默认会被跳过。使用 `--force` 可以覆盖重新安装它们。
- 当 `AFTER_PATH` 不存在时会自动创建。
- `BEFORE_PATH` 和 `AFTER_PATH` 必须是不同的目录。

无法解析 import path 或版本的二进制文件，以及开发版构建（`devel` / `(devel)`），会被跳过而不是被升级，因此本地或不可复现的构建永远不会被破坏。

支持的标志：`--dry-run` (`-n`)、`--notify` (`-N`)、`--jobs` (`-j`)、`--force`。

### 生成手册页（适用于 linux、mac）
man 子命令默认在 `/usr/share/man/man1` 下生成手册页。如果设置了 `MANPATH`，gup 会改为写入每个条目下的 `man1` 目录，并在其尚不存在时创建它。无法写入的目标会以清晰的错误退出。
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

### 生成 shell 补全文件（适用于 bash、zsh、fish 和 PowerShell）
传入 shell 名称后，`completion` 会将补全脚本输出到 STDOUT。
如需将 bash/fish/zsh 的补全文件安装到用户环境中，请使用 `--install`。
对于 PowerShell，请将输出重定向到 `.ps1` 文件，并在 profile 中加载它。

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# 自动安装到默认的用户路径
$ gup completion --install
```

`--install` 会写入与您的 shell/配置布局相匹配的路径：bash 遵循 `XDG_DATA_HOME`（回退到 `$HOME/.local/share`），fish 遵循 `XDG_CONFIG_HOME`（回退到 `$HOME/.config`），zsh 则通过 `ZDOTDIR`（回退到 `$HOME`）解析补全文件和 `.zshrc`。它仍然需要设置 `HOME`；当 `HOME` 为空时它会立即失败（不会将文件写入当前目录），并且如果任何补全文件无法写入，则以非零状态退出。重新运行 `--install` 是幂等的，不会在 `.zshrc` 中重复 zsh 初始化片段。

### 桌面通知
如果您使用 --notify 选项运行 gup，gup 命令会在更新完成后通知您桌面更新是成功还是失败。
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)

### 禁用彩色输出
默认情况下 gup 会为其输出着色。要关闭颜色，请传入 `--no-color`，或将 `NO_COLOR` 环境变量设置为非空值（遵循 [NO_COLOR](https://no-color.org/) 约定）。这在通过管道传递输出、CI 日志中，或全局设置了 `NO_COLOR` 时非常有用。
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## gup vs. `go tool`
Go 1.24 内置的 [`go tool`](https://go.dev/doc/modules/managing-dependencies#tools) 管理的是限定于单个项目、并记录在该项目 `go.mod` 中的工具，因此这些工具只存在于该模块内部。gup 管理的是在 `$GOBIN` 下系统级安装的二进制文件，即您可以在任意目录运行的命令。请将 `go tool` 用于按项目的工具链，将 gup 用于您的全局工具箱。

## 基准测试
gup 并行运行更新，因此比一次更新一个二进制文件的工具完成得更快。更新 9 个各自都有新版本可用的二进制文件：

| 工具                                                          | 策略     | 时间 |
| ------------------------------------------------------------- | -------- | ---: |
| gup update                                                    | 并行     | 0.7s |
| [go-global-update](https://github.com/Gelio/go-global-update) | 顺序     | 2.9s |
| `go install` 循环                                             | 顺序     | 2.9s |

在 AMD Ryzen AI Max+ 395（32 核）/ 64 GB 内存 / Ubuntu 26.04 / go 1.26.4 上测量，5 次运行的中位数，Go 模块缓存已预热。时间取决于每个二进制文件的构建时间和您的 CPU。

## 功能对比

| 功能 | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| 并行更新 | 是 | 否 | 手动 |
| 按包设置更新通道（`latest`/`main`/`master`） | 是 | 否 | 手动 |
| 导出/导入工具集 | 是 | 否 | 手动 |
| 将二进制文件迁移到新的 `$GOBIN` | 是 | 否 | 手动 |
| 机器可读的 JSON 输出（`--json`） | 是 | 否 | 否 |
| 生成/安装 shell 补全 | 是 | 否 | 否 |
| `update` 重新安装已是最新的二进制文件 | 否 | 是 | 是 |
| 目标已存在时 `migrate --force` 重新安装 | 是 | 否 | 手动 |
| 失败诊断 / 后续步骤提示 | 是 | 是 | 否 |
| `NO_COLOR` 支持 | 是 | 是 | — |

## FAQ

### `gup` 失败并提示 `fatal: not a git repository`
您很可能正在使用 oh-my-zsh，它自带一个把 `gup` 设为 `git pull --rebase` 的别名，从而遮蔽了本命令（[#16](https://github.com/nao1215/gup/issues/16)、[#204](https://github.com/nao1215/gup/issues/204)）。请移除或重命名该别名，或在 gup 前加一个反斜杠来绕过它：
```shell
$ \gup update
```

## 贡献
首先，感谢您抽出时间来贡献！❤️ 更多信息请查看 [CONTRIBUTING.md](../../CONTRIBUTING.md)。
开发工作流、质量检查清单和工具管理方法记录在 [CONTRIBUTING.md](../../CONTRIBUTING.md) 中。
贡献不仅与开发相关。例如，GitHub Star 激励我进行开发！

### Star 历史记录
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

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
