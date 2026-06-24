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

gup actualiza y gestiona las herramientas de línea de comandos globales de Go en tu `$GOBIN`. `go install` coloca cada programa en `$GOBIN` (`$GOPATH/bin`) pero nunca lo vuelve a actualizar, no mantiene ningún manifiesto de lo que instaló y no ofrece forma de fijar una herramienta en una versión de la que dependes. gup gestiona ese conjunto de herramientas: pone todo el conjunto al día en paralelo, puede `pin` (fijar) herramientas seleccionadas en versiones exactas y añade los comandos de gestión que le faltan a `go install`: `list`/`check` para ver qué hay instalado, `remove` para eliminar binarios, `export`/`import` para reproducir el conjunto en otra máquina y `migrate` para trasladarlo a un nuevo `$GOBIN`. Se ejecuta en Windows, macOS y Linux.

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
Compilar desde el código fuente requiere Go 1.25 o más reciente. En una versión anterior de Go, instala en su lugar un binario de release precompilado o un paquete (ver más abajo).

### Usar homebrew
```shell
brew install nao1215/tap/gup
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

- Checksums firmados: `checksums.txt` se firma con [cosign](https://github.com/sigstore/cosign) (sin clave), produciendo `checksums.txt.sigstore.json`.
- SBOM: se adjunta un SPDX Software Bill of Materials a cada archivo del release.
- Procedencia de la compilación: la procedencia de compilación SLSA se atestigua mediante GitHub OIDC.

Verifica los checksums firmados (luego comprueba tu archivo contra `checksums.txt`):

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
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

### Fijar una herramienta en una versión específica

Usa `pin` cuando una herramienta global deba permanecer en una versión específica, por ejemplo cuando necesita coincidir con CI o con un entorno de desarrollo común de todo el equipo.

```shell
$ gup pin golangci-lint v1.62.0
$ gup update
```

Una herramienta fijada se instala con la versión registrada (`go install <import_path>@<version>`), nunca `@latest`. `gup update` la mantiene en esa versión y la reinstala allí si la versión instalada difiere; el resto del conjunto de herramientas sigue actualizándose como de costumbre. La fijación bloquea la versión del módulo, no la compilación de Go, por lo que una herramienta fijada se sigue recompilando en la versión fijada cuando cambia el toolchain de Go (usa `--ignore-go-update` para suprimir eso, exactamente igual que con las herramientas no fijadas). La fijación se almacena en `gup.json` con `channel: "pinned"`:

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

`gup pin` también acepta la forma `tool@version` (`gup pin golangci-lint@v1.62.0`). La herramienta ya debe estar instalada bajo `$GOBIN`. Para permitir que la herramienta vuelva a actualizarse:

```shell
$ gup unpin golangci-lint
```

`gup check` reporta una herramienta fijada como `pinned` cuando está en la versión fijada y compilada con el toolchain de Go actual, o como `pin-mismatch` (con una sugerencia `gup update <name>`) cuando la versión instalada difiere o hay pendiente una recompilación por el toolchain de Go; nunca compara una herramienta fijada contra `@latest`.

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

### Salida silenciosa para conjuntos grandes de herramientas
`check` y `update` imprimen cada binario por defecto, lo cual genera mucho ruido cuando tienes muchas herramientas instaladas. Pasa `--quiet` (`-q`) para suprimir las líneas de los binarios que están al día y mostrar solo los binarios que se actualizaron (o que tienen una actualización disponible) junto con los fallos, seguido de un resumen de una línea. Los errores siempre se escriben en STDERR, por lo que permanecen visibles. Cuando también se pasa `--json`, se ignora `--quiet` y se imprime el array JSON completo.
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

### Salida JSON legible por máquinas (para scripting / CI)
`list`, `check` y `update` aceptan `--json`, imprimiendo un array JSON en lugar de la salida legible por humanos (que sigue siendo la opción por defecto).

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

Cada elemento tiene estos campos: `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`/`pinned`), `current_version`, `latest_version` (vacío para `list` y para paquetes fijados), `pinned_version` (presente solo para `channel: "pinned"`), `current_go_version`, `installed_go_version`, `status`, `error` (se omite cuando no existe) y `hint` (una sugerencia del siguiente paso, presente solo cuando alguna aplica al error). `status` es `installed` (list), `up-to-date`, `update-available` (check), `updated` (update), `pinned`/`pin-mismatch` (un paquete fijado en / fuera de su versión fijada) o `error`.

El array siempre es JSON válido, incluso con fallos parciales (esos paquetes obtienen `"status": "error"`; el detalle del error también va a STDERR para que STDOUT siga siendo JSON puro). Los códigos de salida no cambian: `check` reportando `update-available` sigue saliendo con `0`.

### Comportamiento en un entorno vacío
Un entorno global vacío (sin binarios instalados aún por `go install`) se trata como una condición normal de primera ejecución, no como un error:

- `list`, `check` y `update` terminan con `0`, mostrando una breve nota informativa (o un array vacío válido `[]` con `--json`).
- `export` termina con `0` y escribe un `gup.json` vacío.

Nombrar un binario que no está instalado, o excluir todos los binarios, sigue siendo un error de uso y termina con `1`.

### Subcomando Export／Import
Usa export/import si quieres instalar los mismos binarios de golang en múltiples sistemas.
`gup.json` guarda el import path, la `version` registrada del binario y su `channel` de actualización (`latest` / `main` / `master` / `pinned`) de cada herramienta. Para `channel: "pinned"`, `version` es la versión objetivo exacta en la que se mantiene la herramienta; para los demás canales es la versión que se registró en el momento de la exportación. `import` instala exactamente la versión escrita en el archivo, y un paquete fijado permanece fijado después de la importación.

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
- `gup import`, `gup check` y `gup update` detectan automáticamente la ruta en este orden:
  1) `$XDG_CONFIG_HOME/gup/gup.json` (si existe)
  2) `./gup.json` (si existe)

Si existen ambos archivos `gup.json` (el de nivel de usuario y `./gup.json`), `import`, `check` y `update` fallan de inmediato y te piden desambiguar con `--file`, en lugar de elegir uno en silencio. Siempre puedes sobrescribir la ruta con `--file` (`-f`).

`schema_version` es `1` para configuraciones sin paquetes fijados y `2` una vez que algún paquete está fijado, de modo que un entorno que no usa fijaciones sigue produciendo el formato `1` que las versiones más antiguas de gup pueden leer. gup lee tanto `1` como `2`. El canal `pinned` solo es válido bajo `schema_version: 2`; una entrada `pinned` bajo `schema_version: 1`, un paquete fijado sin una versión concreta, un valor de canal desconocido o un `schema_version` no soportado se rechaza.

Un `gup.json` mal formado o inválido (JSON inválido, un canal desconocido, un `schema_version` no soportado o una fijación insegura) se trata como un error en lugar de ignorarse en silencio: `check`, `update` y `export` fallan de inmediato y nombran el archivo problemático, de modo que los canales por paquete guardados nunca se degradan silenciosamente a `latest` porque la configuración no se pudo parsear. Un canal desconocido nunca se normaliza a `latest`.

`gup export` siempre resuelve los canales de actualización guardados desde el `gup.json` canónico de nivel de usuario; `--file`/`--output` solo cambian el destino de exportación, por lo que exportar a un archivo nuevo nunca restablece el canal de un paquete a `latest`.

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

`gup migrate` reinstala los binarios de Go bajo `BEFORE_PATH` en `AFTER_PATH`, usando el `import path@version` exacto registrado en la build info de cada binario (nunca actualiza silenciosamente a `@latest`). Internamente solo establece `GOBIN` en `AFTER_PATH` y ejecuta el flujo normal de `go install`, por lo que los binarios se recompilan con el toolchain de Go actualmente en uso.

#### Por qué es útil (p. ej. con `mise`)

Cuando gestionas Go con [`mise`](https://mise.jdx.dev/), actualizar Go puede cambiar la ruta real de `$GOBIN` por cada versión de Go. Como resultado, las herramientas que instalaste bajo el `$GOBIN` anterior dejan de ser visibles para el nuevo Go. `gup migrate` te permite reinstalar el mismo conjunto de herramientas de Go desde el `$GOBIN` antiguo en el nuevo:

```shell
# Reinstalar todas las herramientas go-install del GOBIN antiguo en el GOBIN nuevo
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrar solo binarios específicos
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` es solo de adición (add-only):

- Nunca elimina ni limpia archivos en `AFTER_PATH`.
- Los binarios que ya existen en `AFTER_PATH` se omiten por defecto. Usa `--force` para reinstalarlos por encima.
- `AFTER_PATH` se crea automáticamente cuando no existe.
- `BEFORE_PATH` y `AFTER_PATH` deben ser directorios diferentes.

Los binarios cuyo import path o versión no se puede resolver, y las compilaciones de desarrollo (`devel` / `(devel)`), se omiten en lugar de actualizarse, de modo que las compilaciones locales o no reproducibles nunca se rompen.

Flags soportados: `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`), `--force`.

### Generar páginas de manual (para linux, mac)
El subcomando man genera páginas de manual bajo `/usr/share/man/man1` de forma predeterminada. Si `MANPATH` está definido, gup escribe en el directorio `man1` bajo cada entrada, creándolo cuando aún no existe. Un destino donde no se puede escribir termina con un error claro.
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

`--install` requiere que `HOME` esté definida; falla de inmediato (sin escribir archivos en el directorio actual) cuando `HOME` está vacía, y termina con un código distinto de cero si algún archivo de completado no se puede escribir.

### Notificación de escritorio
Si usas gup con la opción --notify, el comando gup te notificará en tu escritorio si la actualización fue exitosa o no después de que termine la actualización.
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)

### Desactivar la salida con colores
gup colorea su salida por defecto. Para desactivar los colores, pasa `--no-color` o establece la variable de entorno `NO_COLOR` con un valor no vacío (siguiendo la convención [NO_COLOR](https://no-color.org/)). Esto es útil al canalizar la salida, en logs de CI o con `NO_COLOR` establecido globalmente.
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## gup vs. `go tool`
El [`go tool`](https://go.dev/doc/modules/managing-dependencies#tools) integrado en Go 1.24 gestiona herramientas con alcance a un único proyecto y registradas en el `go.mod` de ese proyecto, por lo que esas herramientas solo existen dentro de ese módulo. gup gestiona los binarios instalados a nivel de sistema bajo `$GOBIN`, los comandos que ejecutas desde cualquier directorio y que mantienes junto a tus dotfiles, fijados opcionalmente en las versiones de las que dependes. Usa `go tool` para las herramientas específicas de cada proyecto y gup para tu caja de herramientas global.

## Comparación de características

| Característica | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| Actualización en paralelo | Sí | No | Manual |
| Tiempo de actualización (9 binarios) | 0.7s | 2.9s | 2.9s |
| Canales de actualización por paquete (`latest`/`main`/`master`) | Sí | No | Manual |
| Fijado / bloqueo de versión | Sí | No | Manual |
| Exportar/importar conjunto de herramientas | Sí | No | Manual |
| Migrar binarios a un nuevo `$GOBIN` | Sí | No | Manual |
| Salida JSON legible por máquinas (`--json`) | Sí | No | No |
| Generación/instalación de autocompletado de shell | Sí | No | No |
| `update` reinstala binarios que están al día | No | Sí | Sí |
| `migrate --force` reinstala cuando el destino ya existe | Sí | No | Manual |
| Diagnóstico de fallos / sugerencias del siguiente paso | Sí | Sí | No |
| Soporte de `NO_COLOR` | Sí | Sí | — |

*Tiempo de actualización: 9 binarios, cada uno con una versión más reciente disponible; gup en paralelo, los demás en secuencia. AMD Ryzen AI Max+ 395 / go 1.26.4, mediana de 5 ejecuciones con caché de módulos en caliente; los tiempos dependen del tiempo de compilación y la CPU.*

## FAQ

### `gup` falla con `fatal: not a git repository`
Probablemente estés usando oh-my-zsh, que incluye un alias `gup` para `git pull --rebase` que oculta este comando ([#16](https://github.com/nao1215/gup/issues/16), [#204](https://github.com/nao1215/gup/issues/204)). Elimina o renombra ese alias, o ejecuta gup con una barra invertida inicial para evitarlo:
```shell
$ \gup update
```

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
