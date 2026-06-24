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
> 📖 Это перевод, который может отставать от [английского README](../../README.md) — основного источника информации.

# gup - Обновляет бинарные файлы, установленные через "go install"

![sample](../img/sample.gif)

gup обновляет и управляет глобальными инструментами командной строки Go в вашем `$GOBIN`. `go install` помещает каждую программу в `$GOBIN` (`$GOPATH/bin`), но больше никогда её не обновляет, не хранит манифест того, что установил, и не предлагает способа удержать инструмент на версии, от которой вы зависите. gup управляет этим набором инструментов: он приводит весь набор в актуальное состояние параллельно, может закреплять (`pin`) выбранные инструменты на точных версиях, а также добавляет команды управления, которых нет у `go install`: `list`/`check` — посмотреть, что установлено, `remove` — удалить бинарные файлы, `export`/`import` — выгрузить набор и воспроизвести его на другой машине, а также `migrate` — перенести его в новый `$GOBIN`. Работает на Windows, macOS и Linux.

## Поддерживаемые ОС (модульное тестирование с GitHub Actions)
- Linux
- Mac
- Windows

## Как установить
gup уже доступен через `winget`, `mise` и `nix` помимо `go install` и Homebrew.

### Использовать "go install"
Если на вашей системе не установлена среда разработки golang, пожалуйста, установите golang с [официального сайта golang](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```
Для сборки из исходного кода требуется Go 1.25 или новее. На более старой версии Go вместо этого установите готовый бинарный файл релиза или пакет (см. ниже).

### Использовать homebrew
```shell
brew install nao1215/tap/gup
```

### Использовать winget (Windows)
```shell
winget install --id nao1215.gup
```

### Использовать mise-en-place
```shell
mise use -g gup@latest
```

### Использовать nix (профиль Nix)
```shell
nix profile install nixpkgs#gogup
```

### Установка из пакета или бинарного файла
[Страница релизов](https://github.com/nao1215/gup/releases) содержит пакеты в форматах .deb, .rpm и .apk. Команда gup использует команду go внутренне, поэтому требуется установка golang.

## Проверка целостности релиза
Каждый релиз поставляется с метаданными цепочки поставок, чтобы вы могли проверить то, что скачиваете:

- Подписанные контрольные суммы: `checksums.txt` подписан с помощью [cosign](https://github.com/sigstore/cosign) (без ключей), создавая `checksums.txt.sigstore.json`.
- SBOM: к каждому архиву релиза прикреплена SPDX Software Bill of Materials.
- Происхождение сборки: происхождение сборки SLSA удостоверяется через GitHub OIDC.

Проверьте подписанные контрольные суммы (затем сверьте свой архив с `checksums.txt`):

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Проверьте происхождение сборки скачанного артефакта с помощью GitHub CLI:

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

## Как использовать
### Обновить все бинарные файлы
Если вы хотите обновить все бинарные файлы, просто выполните `$ gup update`.

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

### Обновить указанный бинарный файл
Если вы хотите обновить только указанные бинарные файлы, укажите несколько имён команд, разделённых пробелом.
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### Исключить бинарные файлы во время gup update
Если вы не хотите обновлять некоторые бинарные файлы, просто укажите бинарные файлы, которые не должны быть обновлены, разделённые ',' без пробелов в качестве разделителя.
Также работает в сочетании с --dry-run
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude или -e, этот пример исключит 'gopls' и 'golangci-lint'
```

### Обновить бинарные файлы с @main, @master или @latest
Если вы хотите управлять источником обновления для каждого бинарного файла, используйте следующие опции:
- `--main` (`-m`): обновление через `@main` (при ошибке откат на `@master`)
- `--master`: обновление через `@master`
- `--latest`: обновление через `@latest`

Выбранный канал сохраняется в `gup.json` и переиспользуется в следующих запусках `gup update`.
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
```

### Закрепить инструмент на определённой версии

Используйте `pin`, когда глобальный инструмент должен оставаться на определённой версии, например, когда он должен совпадать с CI или общей для команды средой разработки.

```shell
$ gup pin golangci-lint v1.62.0
$ gup update
```

Закреплённый инструмент устанавливается с записанной версией (`go install <import_path>@<version>`), а не через `@latest`. `gup update` удерживает его на этой версии и переустанавливает его на ней, если установленная версия отличается; остальной набор инструментов при этом обновляется как обычно. Закрепление фиксирует версию модуля, а не сборку Go, поэтому закреплённый инструмент всё равно пересобирается на закреплённой версии при изменении тулчейна Go (используйте `--ignore-go-update`, чтобы подавить это, точно так же, как и для незакреплённых инструментов). Закрепление сохраняется в `gup.json` с `channel: "pinned"`:

```json
{
  "schema_version": 2,
  "packages": [
    {
      "name": "golangci-lint",
      "import_path": "github.com/golangci/golangci-lint/cmd/golangci-lint",
      "version": "v1.62.0",
      "channel": "pinned"
    }
  ]
}
```

`gup pin` также принимает форму `tool@version` (`gup pin golangci-lint@v1.62.0`). Инструмент уже должен быть установлен в `$GOBIN`. Чтобы снова разрешить инструменту обновляться:

```shell
$ gup unpin golangci-lint
```

`gup check` сообщает о закреплённом инструменте как `pinned`, когда он находится на закреплённой версии и собран с текущим тулчейном Go, или как `pin-mismatch` (с предложением `gup update <name>`), когда установленная версия отличается или ожидается пересборка под тулчейн Go; он никогда не сравнивает закреплённый инструмент с `@latest`.

### Вывести список имён команд с путём пакета и версией в $GOPATH/bin
Подкоманда list выводит информацию о командах в $GOPATH/bin или $GOBIN. Выводимая информация - это имя команды, путь пакета и версия команды.
![sample](../img/list.png)

### Удалить указанный бинарный файл
Если вы хотите удалить команду в $GOPATH/bin или $GOBIN, используйте подкоманду remove. Подкоманда remove спрашивает, хотите ли вы её удалить, перед удалением.
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

Если вы хотите принудительное удаление, используйте опцию --force.
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

### Проверить, является ли бинарный файл последней версией
Если вы хотите знать, является ли бинарный файл последней версией, используйте подкоманду check. Подкоманда check проверяет, является ли бинарный файл последней версией, и отображает имя бинарного файла, который нуждается в обновлении.
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

Как и другие подкоманды, вы можете проверить только указанные бинарные файлы.
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
           $ gup update mimixbox
```

### Тихий вывод для большого набора инструментов
По умолчанию `check` и `update` выводят каждый бинарный файл, что создаёт много шума, когда у вас установлено множество инструментов. Передайте `--quiet` (`-q`), чтобы скрыть строки об актуальных версиях и показать только бинарные файлы, которые были обновлены (или для которых доступно обновление), а также сбои, после чего следует однострочная сводка. Ошибки всегда выводятся в STDERR, поэтому они остаются видимыми. Если также указан `--json`, флаг `--quiet` игнорируется и выводится полный JSON-массив.
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

### Машиночитаемый вывод JSON (для скриптов / CI)
`list`, `check` и `update` принимают `--json`, выводя JSON-массив вместо человекочитаемого вывода (который остаётся по умолчанию).

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

Каждый элемент имеет следующие поля: `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`/`pinned`), `current_version`, `latest_version` (пусто для `list` и для закреплённых пакетов), `pinned_version` (присутствует только для `channel: "pinned"`), `current_go_version`, `installed_go_version`, `status`, `error` (опускается, если отсутствует) и `hint` (подсказка о следующем шаге, присутствует только тогда, когда она применима к ошибке). `status` принимает значения `installed` (list), `up-to-date`, `update-available` (check), `updated` (update), `pinned`/`pin-mismatch` (закреплённый пакет на / вне его закреплённой версии) или `error`.

Массив всегда является корректным JSON, включая случаи частичных сбоев (такие пакеты получают `"status": "error"`; детали ошибки также выводятся в STDERR, поэтому STDOUT остаётся чистым JSON). Коды завершения не изменяются — `check`, сообщающий `update-available`, по-прежнему завершается с кодом `0`.

### Поведение в пустом окружении
Пустое глобальное окружение (ещё ни один бинарный файл не установлен через `go install`) рассматривается как нормальная ситуация первого запуска, а не как ошибка:

- `list`, `check` и `update` завершаются с кодом `0`, выводя короткое информационное сообщение (или валидный пустой массив `[]` с `--json`).
- `export` завершается с кодом `0` и записывает пустой `gup.json`.

Указание неустановленного бинарного файла или исключение всех бинарных файлов по-прежнему является ошибкой использования и завершается с кодом `1`.

### Подкоманда Export／Import
Используйте export/import, если хотите установить одинаковые бинарные файлы golang на нескольких системах.
`gup.json` хранит import path каждого инструмента, записанную версию бинарного файла (`version`) и его канал обновления (`channel`) (`latest` / `main` / `master` / `pinned`). Для `channel: "pinned"` `version` — это точная целевая версия, на которой удерживается инструмент; для остальных каналов это версия, записанная на момент экспорта. `import` устанавливает точно ту версию, которая записана в файле, и закреплённый пакет остаётся закреплённым после импорта.

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

По умолчанию:
- `gup export` записывает в `$XDG_CONFIG_HOME/gup/gup.json`
- `gup import`, `gup check` и `gup update` автоматически ищут файл в следующем порядке:
  1) `$XDG_CONFIG_HOME/gup/gup.json` (если существует)
  2) `./gup.json` (если существует)

Если существуют оба файла `gup.json` (на уровне пользователя и `./gup.json`), `import`, `check` и `update` сразу завершаются с ошибкой и просят устранить неоднозначность через `--file`, вместо того чтобы молча выбрать один. Путь всегда можно переопределить через `--file` (`-f`).

`schema_version` равен `1` для конфигураций без закреплённых пакетов и `2`, как только какой-либо пакет закреплён, поэтому окружение, не использующее закрепления, продолжает создавать формат `1`, который могут читать более старые релизы gup. gup читает как `1`, так и `2`. Канал `pinned` допустим только при `schema_version: 2`; запись `pinned` при `schema_version: 1`, закреплённый пакет без конкретной версии, неизвестное значение канала или неподдерживаемый `schema_version` отклоняются.

Некорректный или невалидный `gup.json` (невалидный JSON, неизвестный канал, неподдерживаемый `schema_version` или небезопасное закрепление) рассматривается как ошибка, а не молча игнорируется: `check`, `update` и `export` сразу завершаются с ошибкой и называют проблемный файл, поэтому сохранённые каналы для каждого пакета никогда не понижаются молча до `latest` из-за того, что конфигурацию не удалось разобрать. Неизвестный канал никогда не нормализуется в `latest`.

`gup export` всегда определяет сохранённые каналы обновления из канонического `gup.json` на уровне пользователя; `--file`/`--output` меняют только место назначения экспорта, поэтому экспорт в новый файл никогда не сбрасывает канал пакета обратно на `latest`.

```shell
※ Окружение A (например, ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.json

※ Окружение B (например, debian)
$ gup import
```

Также `export` может вывести содержимое `gup.json` в STDOUT через `--output`. `import` может читать конкретный файл через `--file`.
```shell
※ Окружение A (например, ubuntu)
$ gup export --output > gup.json

※ Окружение B (например, debian)
$ gup import --file=gup.json
```

### Перенос бинарных файлов в новый $GOBIN

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` переустанавливает Go-бинарные файлы из `BEFORE_PATH` в `AFTER_PATH`, используя точный `import path@version`, записанный в build info каждого бинарного файла (он никогда не выполняет тихое обновление до `@latest`). Внутренне он просто устанавливает `GOBIN` в `AFTER_PATH` и запускает обычный путь `go install`, поэтому бинарные файлы пересобираются с текущим используемым тулчейном Go.

#### Чем это полезно (например, с `mise`)

Когда вы управляете Go с помощью [`mise`](https://mise.jdx.dev/), обновление Go может изменить реальный путь `$GOBIN` для каждой версии Go. В результате инструменты, установленные под предыдущим `$GOBIN`, больше не видны новому Go. `gup migrate` позволяет переустановить тот же набор Go-инструментов из старого `$GOBIN` в новый:

```shell
# Переустановить все go-install инструменты из старого GOBIN в новый GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Перенести только указанные бинарные файлы
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` работает только на добавление:

- Он никогда не удаляет и не очищает файлы в `AFTER_PATH`.
- Бинарные файлы, которые уже существуют в `AFTER_PATH`, по умолчанию пропускаются. Используйте `--force`, чтобы переустановить их поверх.
- `AFTER_PATH` создаётся автоматически, если он не существует.
- `BEFORE_PATH` и `AFTER_PATH` должны быть разными каталогами.

Бинарные файлы, чей import path или версию невозможно определить, а также сборки для разработки (`devel` / `(devel)`), пропускаются, а не обновляются, поэтому локальные или невоспроизводимые сборки никогда не ломаются.

Поддерживаемые флаги: `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`), `--force`.

### Генерировать man-страницы (для linux, mac)
Подкоманда man по умолчанию генерирует man-страницы в `/usr/share/man/man1`. Если задан `MANPATH`, gup записывает в каталог `man1` под каждой записью, создавая его, если он ещё не существует. Недоступный для записи путь завершается с понятной ошибкой.
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

### Генерация файла автодополнения оболочки (для bash, zsh, fish и PowerShell)
`completion` выводит скрипты автодополнения в STDOUT, когда вы передаёте имя оболочки.
Чтобы установить файлы автодополнения в пользовательское окружение для bash/fish/zsh, используйте `--install`.
Для PowerShell перенаправьте вывод в файл `.ps1` и подключите его из профиля.

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# Автоматически установить файлы в стандартные пользовательские пути
$ gup completion --install
```

`--install` требует, чтобы была задана `HOME`; при пустой `HOME` она сразу завершается с ошибкой (не записывая файлы в текущий каталог) и завершается с ненулевым кодом, если какой-либо файл автодополнения не удаётся записать.

### Уведомления на рабочем столе
Если вы используете gup с опцией --notify, команда gup уведомляет вас на рабочем столе о том, было ли обновление успешным или неуспешным после завершения обновления.
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)

### Отключить цветной вывод
По умолчанию gup раскрашивает свой вывод. Чтобы отключить цвета, передайте `--no-color` или установите переменной окружения `NO_COLOR` непустое значение (следуя соглашению [NO_COLOR](https://no-color.org/)). Это полезно при перенаправлении вывода через конвейер, в логах CI или при глобально установленной `NO_COLOR`.
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## gup vs. `go tool`
Встроенный в Go 1.24 [`go tool`](https://go.dev/doc/modules/managing-dependencies#tools) управляет инструментами в рамках одного проекта, записанными в `go.mod` этого проекта, поэтому такие инструменты существуют только внутри этого модуля. gup управляет бинарными файлами, установленными системно в `$GOBIN`, — командами, которые вы запускаете из любого каталога и храните рядом со своими dotfiles, при желании закреплёнными на версиях, от которых вы зависите. Используйте `go tool` для инструментов уровня проекта, а gup — для вашего глобального набора инструментов.

## Сравнение возможностей

| Возможность | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| Параллельное обновление | Да | Нет | Вручную |
| Время обновления (9 бинарников) | 0.7s | 2.9s | 2.9s |
| Каналы обновления для каждого пакета (`latest`/`main`/`master`) | Да | Нет | Вручную |
| Закрепление / блокировка версии | Да | Нет | Вручную |
| Экспорт/импорт набора инструментов | Да | Нет | Вручную |
| Перенос бинарных файлов в новый `$GOBIN` | Да | Нет | Вручную |
| Машиночитаемый вывод JSON (`--json`) | Да | Нет | Нет |
| Генерация/установка автодополнения оболочки | Да | Нет | Нет |
| `update` переустанавливает актуальные бинарные файлы | Нет | Да | Да |
| `migrate --force` переустанавливает, когда цель уже существует | Да | Нет | Вручную |
| Диагностика сбоев / подсказки по дальнейшим шагам | Да | Да | Нет |
| Поддержка `NO_COLOR` | Да | Да | — |

*Время обновления: 9 бинарников, для каждого доступна более новая версия; gup — параллельно, остальные — последовательно. AMD Ryzen AI Max+ 395 / go 1.26.4, медиана из 5 запусков с прогретым кешем модулей; время зависит от времени сборки и CPU.*

## FAQ

### `gup` завершается с ошибкой `fatal: not a git repository`
Вероятно, вы используете oh-my-zsh, который поставляет алиас `gup` для `git pull --rebase`, перекрывающий эту команду ([#16](https://github.com/nao1215/gup/issues/16), [#204](https://github.com/nao1215/gup/issues/204)). Удалите или переименуйте этот алиас либо запускайте gup с ведущим обратным слешем, чтобы обойти его:
```shell
$ \gup update
```

## Участие в проекте
Прежде всего, спасибо за то, что уделяете время участию! ❤️  Смотрите [CONTRIBUTING.md](../../CONTRIBUTING.md) для получения дополнительной информации.
Рабочий процесс разработки, чек-лист качества и управление инструментами описаны в [CONTRIBUTING.md](../../CONTRIBUTING.md).
Вклады связаны не только с разработкой. Например, GitHub Star мотивирует меня к разработке!

### История звёзд
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

## Контакты
Если вы хотите отправить комментарии, такие как "найден баг" или "запрос дополнительных функций" разработчику, пожалуйста, используйте один из следующих контактов.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

Вы можете использовать подкоманду bug-report для отправки отчёта о баге.
```
$ gup bug-report
※ Откроется страница GitHub issue в вашем браузере по умолчанию
```

## ЛИЦЕНЗИЯ
Проект gup лицензирован на условиях [Apache License 2.0](../../LICENSE).


## Участники ✨

Спасибо этим замечательным людям ([ключ emoji](https://allcontributors.org/docs/en/emoji-key)):

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

Этот проект следует спецификации [all-contributors](https://github.com/all-contributors/all-contributors). Приветствуются вклады любого рода!
