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
> 📖 Este documento es una traducción y puede estar desactualizado respecto al [README en inglés](../../README.md), que es la fuente de verdad.

# gup - Actualizar binarios instalados por "go install"

![sample](../img/sample.gif)

El comando **gup** actualiza los binarios instalados por "go install" a la versión más reciente. gup actualiza todos los binarios en paralelo, por lo que es muy rápido. También proporciona subcomandos para manipular binarios bajo $GOPATH/bin ($GOBIN). Es un software multiplataforma que se ejecuta en Windows, Mac y Linux.

Si estás usando oh-my-zsh, entonces gup tiene un alias configurado. El alias es `gup - git pull --rebase`. Por lo tanto, asegúrate de que el alias de oh-my-zsh esté deshabilitado (e.g. $ \gup update).

## Cambio incompatible (v1.0.0)
- El formato de configuración cambió de `gup.conf` a `gup.json`.
- `gup import` ya no lee `gup.conf`.
- El canal de actualización por paquete (`latest` / `main` / `master`) se guarda en `gup.json`.


## OS Soportados (pruebas unitarias con GitHub Actions)
- Linux
- Mac
- Windows

## Cómo instalar
gup también está disponible mediante `winget`, `mise` y `nix`, además de `go install` y Homebrew.

### Usar "go install"
Si no tienes el entorno de desarrollo de golang instalado en tu sistema, por favor instala golang desde el [sitio web oficial de golang](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```

### Usar homebrew
```shell
brew install nao1215/gup
```

### Usar winget (Windows)
```shell
winget install --id nao1215.gup
```

### Usar mise-en-place
```shell
mise use -g gup@latest
```

### Usar nix (perfil de Nix)
```shell
nix profile install nixpkgs#gogup
```

### Instalar desde Paquete o Binario
[La página de releases](https://github.com/nao1215/gup/releases) contiene paquetes en formatos .deb, .rpm y .apk. El comando gup usa el comando go internamente, por lo que se requiere la instalación de golang.

## Verificar la integridad del release
Cada release incluye metadatos de cadena de suministro para que puedas verificar lo que descargas:

- **Checksums firmados** — `checksums.txt` se firma con [cosign](https://github.com/sigstore/cosign) (sin clave), produciendo `checksums.txt.sig` y `checksums.txt.pem`.
- **SBOM** — se adjunta un SPDX Software Bill of Materials a cada archivo del release.
- **Procedencia de la compilación** — la procedencia de compilación SLSA se atestigua mediante GitHub OIDC.

Verifica los checksums firmados (luego comprueba tu archivo contra `checksums.txt`):

```shell
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Verifica la procedencia de compilación de un artefacto descargado con la CLI de GitHub:

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

## Cómo usar
### Actualizar todos los binarios
Si quieres actualizar todos los binarios, simplemente ejecuta `$ gup update`.

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

### Actualizar el binario especificado
Si quieres actualizar solo los binarios especificados, especifica múltiples nombres de comandos separados por espacio.
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### Excluir binarios durante gup update
Si no quieres actualizar algunos binarios, simplemente especifica los binarios que no deben actualizarse separados usando ',' sin espacios como delimitador.
También funciona en combinación con --dry-run
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude o -e, este ejemplo excluirá 'gopls' y 'golangci-lint'
```

### Actualizar binarios con @main, @master o @latest
Si quieres controlar la fuente de actualización por binario, usa estas opciones:
- `--main` (`-m`): actualiza con `@main` (si falla, usa `@master`)
- `--master`: actualiza con `@master`
- `--latest`: actualiza con `@latest`

El canal seleccionado se guarda en `gup.json` y se reutiliza en futuros `gup update`.
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
```

### Listar el nombre del comando con ruta del paquete y versión bajo $GOPATH/bin
El subcomando list imprime información de comandos bajo $GOPATH/bin o $GOBIN. La información de salida es el nombre del comando, ruta del paquete y versión del comando.
![sample](../img/list.png)

### Eliminar el binario especificado
Si quieres eliminar un comando bajo $GOPATH/bin o $GOBIN, usa el subcomando remove. El subcomando remove pregunta si quieres eliminarlo antes de eliminarlo.
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

Si quieres forzar la eliminación, usa la opción --force.
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

### Verificar si el binario es la versión más reciente
Si quieres saber si el binario es la versión más reciente, usa el subcomando check. El subcomando check verifica si el binario es la versión más reciente y muestra el nombre del binario que necesita ser actualizado.
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

Como otros subcomandos, solo puedes verificar los binarios especificados.
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
           $ gup update mimixbox
```
### Subcomando Export／Import
Usa export/import si quieres instalar los mismos binarios de golang en múltiples sistemas.
`gup.json` guarda el import path, la versión del binario y el canal de actualización (`latest` / `main` / `master`).
`import` instala exactamente la versión escrita en el archivo.

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

Por defecto:
- `gup export` escribe en `$XDG_CONFIG_HOME/gup/gup.json`
- `gup import` detecta automáticamente la ruta en este orden:
  1) `$XDG_CONFIG_HOME/gup/gup.json` (si existe)
  2) `./gup.json` (si existe)

Puedes sobrescribir la ruta con `--file`.

```shell
※ Entorno A (e.g. ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.json

※ Entorno B (e.g. debian)
$ gup import
```

Alternativamente, `export` puede imprimir el contenido de `gup.json` en STDOUT con `--output`. `import` puede leer un archivo específico con `--file`.
```shell
※ Entorno A (e.g. ubuntu)
$ gup export --output > gup.json

※ Entorno B (e.g. debian)
$ gup import --file=gup.json
```

### Migrar binarios a un nuevo $GOBIN

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` reinstala los binarios de Go bajo `BEFORE_PATH` en `AFTER_PATH`,
usando el `import path@version` exacto registrado en la build info de cada binario
(nunca actualiza silenciosamente a `@latest`). Internamente solo establece `GOBIN`
en `AFTER_PATH` y ejecuta el flujo normal de `go install`, por lo que los binarios
se recompilan con el toolchain de Go actualmente en uso.

#### Por qué es útil (p. ej. con `mise`)

Cuando gestionas Go con [`mise`](https://mise.jdx.dev/), actualizar Go puede cambiar
la ruta real de `$GOBIN` por cada versión de Go. Como resultado, las herramientas que
instalaste bajo el `$GOBIN` anterior dejan de ser visibles para el nuevo Go.
`gup migrate` te permite reinstalar el mismo conjunto de herramientas de Go desde el
`$GOBIN` antiguo en el nuevo:

```shell
# Reinstalar todas las herramientas go-install del GOBIN antiguo en el GOBIN nuevo
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrar solo binarios específicos
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` es solo de adición (add-only):

- Nunca elimina ni limpia archivos en `AFTER_PATH`.
- Los binarios que ya existen en `AFTER_PATH` se omiten por defecto. Usa
  `--force` para reinstalarlos por encima.
- `AFTER_PATH` se crea automáticamente cuando no existe.
- `BEFORE_PATH` y `AFTER_PATH` deben ser directorios diferentes.

Los binarios cuyo import path o versión no se puede resolver, y las compilaciones de
desarrollo (`devel` / `(devel)`), se omiten en lugar de actualizarse, de modo que las
compilaciones locales o no reproducibles nunca se rompen.

Flags soportados: `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`),
`--force`.

### Generar páginas de manual (para linux, mac)
El subcomando man genera páginas de manual bajo /usr/share/man/man1.
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

### Generar archivo de autocompletado de shell (para bash, zsh, fish y PowerShell)
`completion` imprime scripts de autocompletado en STDOUT cuando pasas un nombre de shell.
Para instalar archivos de autocompletado en tu entorno de usuario para bash/fish/zsh, usa `--install`.
Para PowerShell, redirige la salida a un archivo `.ps1` y cárgalo desde tu perfil.

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# Instalar archivos automáticamente en rutas predeterminadas del usuario
$ gup completion --install
```

### Notificación de escritorio
Si usas gup con la opción --notify, el comando gup te notificará en tu escritorio si la actualización fue exitosa o no después de que termine la actualización.
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)


## Benchmark
gup ejecuta las actualizaciones en paralelo, por lo que termina más rápido que las herramientas que actualizan los binarios de uno en uno. Actualizando 9 binarios que tenían una versión más reciente disponible:

| Herramienta                                                   | Estrategia | Tiempo |
| ------------------------------------------------------------- | ---------- | -----: |
| gup update                                                    | paralelo   |   0.7s |
| [go-global-update](https://github.com/Gelio/go-global-update) | secuencial |   2.9s |
| bucle `go install`                                            | secuencial |   2.9s |

Medido en AMD Ryzen AI Max+ 395 (32 núcleos) / 64 GB RAM / Ubuntu 26.04 / go 1.26.4, mediana de 5 ejecuciones con la caché de módulos de Go en caliente. Los tiempos dependen del tiempo de compilación de cada binario y de tu CPU.

## Contribuir
En primer lugar, ¡gracias por tomarte el tiempo de contribuir! ❤️  Ve [CONTRIBUTING.md](../../CONTRIBUTING.md) para más información.
El flujo de desarrollo, la lista de comprobación de calidad y la gestión de herramientas están documentados en [CONTRIBUTING.md](../../CONTRIBUTING.md).
Las contribuciones no solo están relacionadas con el desarrollo. Por ejemplo, ¡GitHub Star me motiva a desarrollar!

### Historial de Estrellas
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

## Contacto
Si quieres enviar comentarios como "encontré un error" o "solicitud de características adicionales" al desarrollador, por favor usa uno de los siguientes contactos.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

Puedes usar el subcomando bug-report para enviar un reporte de error.
```
$ gup bug-report
※ Open GitHub issue page by your default browser
```

## LICENCIA
El proyecto gup está licenciado bajo los términos de [la Licencia Apache 2.0](../../LICENSE).


## Colaboradores ✨

Gracias a estas maravillosas personas ([clave de emoji](https://allcontributors.org/docs/en/emoji-key)):

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

Este proyecto sigue la especificación [all-contributors](https://github.com/all-contributors/all-contributors). ¡Contribuciones de cualquier tipo son bienvenidas!
