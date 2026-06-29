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
> 📖 Ce document est une traduction et peut être en retard par rapport au [README anglais](../../README.md), qui fait foi.

# gup - Mettre à jour les binaires installés par "go install"

![sample](../img/sample.gif)

gup met à jour et gère les outils Go en ligne de commande installés globalement dans votre `$GOBIN`. `go install` place chaque programme dans `$GOBIN` (`$GOPATH/bin`) mais ne le met plus jamais à jour ensuite, ne conserve aucun manifeste de ce qu'il a installé, et n'offre aucun moyen de maintenir un outil à une version dont vous dépendez. gup gère cet ensemble d'outils : il amène l'ensemble complet à la dernière version en parallèle, peut `pin` (épingler) des outils sélectionnés à des versions exactes, et ajoute les commandes de gestion qui manquent à `go install` : `list`/`check` pour savoir ce qui est installé, `remove` pour supprimer des binaires, `export`/`import` pour reproduire l'ensemble sur une autre machine, et `migrate` pour le déplacer vers un nouveau `$GOBIN`. Fonctionne sur Windows, macOS et Linux.

## OS supportés (tests unitaires avec GitHub Actions)
- Linux
- Mac
- Windows

## Comment installer
gup est aussi disponible via `winget`, `mise`, `nix` et `aqua`, en plus de `go install` et Homebrew.

### Utiliser "go install"
Si vous n'avez pas l'environnement de développement golang installé sur votre système, veuillez installer golang depuis le [site officiel golang](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```
La compilation depuis les sources nécessite Go 1.25 ou une version plus récente. Sur une version plus ancienne de Go, installez plutôt un binaire de release précompilé ou un package (voir ci-dessous).

### Utiliser homebrew
```shell
brew install nao1215/tap/gup
```

### Utiliser winget (Windows)
```shell
winget install --id nao1215.gup
```

### Utiliser mise-en-place
```shell
mise use -g gup@latest
```

### Utiliser nix (profil Nix)
```shell
nix profile install nixpkgs#gogup
```

### Utiliser aqua
gup est enregistré dans le registre standard d'[aqua](https://aquaproj.github.io/). Ajoutez-le à votre `aqua.yaml` :
```shell
aqua g -i nao1215/gup
```

### Installer depuis un package ou un binaire
[La page de release](https://github.com/nao1215/gup/releases) contient des packages aux formats .deb, .rpm et .apk. La commande gup utilise la commande go en interne, donc l'installation de golang est requise.

## Vérifier l'intégrité de la release
Chaque release est accompagnée de métadonnées de chaîne d'approvisionnement afin que vous puissiez vérifier ce que vous téléchargez :

- Sommes de contrôle signées : `checksums.txt` est signé avec [cosign](https://github.com/sigstore/cosign) (sans clé), produisant `checksums.txt.sigstore.json`.
- SBOM : un SPDX Software Bill of Materials est joint à chaque archive de release.
- Provenance de build : la provenance de build SLSA est attestée via GitHub OIDC.

Vérifiez les sommes de contrôle signées (puis comparez votre archive à `checksums.txt`) :

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Vérifiez la provenance de build d'un artefact téléchargé avec la CLI GitHub :

```shell
gh attestation verify gup_<version>_<os>_<arch>.tar.gz --repo nao1215/gup
```

## Comment utiliser
### Mettre à jour tous les binaires
Si vous voulez mettre à jour tous les binaires, exécutez simplement `$ gup update`.

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

### Mettre à jour le binaire spécifié
Si vous voulez mettre à jour seulement les binaires spécifiés, vous spécifiez plusieurs noms de commandes séparés par un espace.
```shell
$ gup update subaru gup ubume
update binary under $GOPATH/bin or $GOBIN
[1/3] github.com/nao1215/gup (v0.7.0 to v0.7.1, go1.20.1 to go1.22.4)
[2/3] github.com/nao1215/subaru (Already up-to-date: v1.0.2 / go1.22.4)
[3/3] github.com/nao1215/ubume/cmd/ubume (Already up-to-date: v1.4.1 / go1.22.4)
```

### Exclure des binaires pendant gup update
Si vous ne voulez pas mettre à jour certains binaires, spécifiez simplement les binaires qui ne doivent pas être mis à jour en les séparant par ',' sans espaces comme délimiteur.
Fonctionne aussi en combinaison avec --dry-run
```shell
$ gup update --exclude=gopls,golangci-lint    //--exclude or -e, cet exemple exclura 'gopls' et 'golangci-lint'
```

### Mettre à jour les binaires avec @main, @master ou @latest
Si vous voulez contrôler la source de mise à jour par binaire, utilisez les options suivantes :
- `--main` (`-m`) : met à jour avec `@main` (repli sur `@master` en cas d'échec)
- `--master` : met à jour avec `@master`
- `--latest` : met à jour avec `@latest`

Le canal sélectionné est enregistré dans `gup.json` et réutilisé lors des prochaines exécutions de `gup update`.
```shell
$ gup update --main=gup,lazygit --master=sqly --latest=air
```

### Épingler un outil à une version spécifique

Utilisez `pin` lorsqu'un outil global doit rester sur une version spécifique, par exemple lorsqu'il doit correspondre à la CI ou à un environnement de développement partagé par toute une équipe.

```shell
$ gup pin golangci-lint v1.62.0
$ gup update
```

Un outil épinglé est installé avec la version enregistrée (`go install <import_path>@<version>`), jamais `@latest`. `gup update` le maintient à cette version et le réinstalle à cette version si la version installée diffère ; le reste de l'ensemble d'outils continue de se mettre à jour comme d'habitude. L'épinglage verrouille la version du module, pas le build Go, donc un outil épinglé est tout de même recompilé à la version épinglée lorsque la toolchain Go change (utilisez `--ignore-go-update` pour supprimer ce comportement, exactement comme pour les outils non épinglés). L'épinglage est stocké dans `gup.json` avec `channel: "pinned"` :

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

`gup pin` accepte également la forme `tool@version` (`gup pin golangci-lint@v1.62.0`). L'outil doit déjà être installé sous `$GOBIN`. Pour autoriser de nouveau l'outil à se mettre à jour :

```shell
$ gup unpin golangci-lint
```

`gup check` signale un outil épinglé comme `pinned` lorsqu'il est à la version épinglée et compilé avec la toolchain Go actuelle, ou `pin-mismatch` (avec une suggestion `gup update <name>`) lorsque la version installée diffère ou qu'une recompilation due à la toolchain Go est en attente ; il ne compare jamais un outil épinglé à `@latest`.

### Lister le nom de commande avec le chemin de package et la version sous $GOPATH/bin
La sous-commande list affiche les informations de commande sous $GOPATH/bin ou $GOBIN. Les informations affichées sont le nom de la commande, le chemin du package et la version de la commande.
![sample](../img/list.png)

### Supprimer le binaire spécifié
Si vous voulez supprimer une commande sous $GOPATH/bin ou $GOBIN, utilisez la sous-commande remove. La sous-commande remove demande si vous voulez la supprimer avant de la supprimer.
```shell
$ gup remove subaru gal ubume
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
gup:CHECK: remove /home/nao/.go/bin/gal? [Y/n] n
cancel removal /home/nao/.go/bin/gal
gup:CHECK: remove /home/nao/.go/bin/ubume? [Y/n] Y
removed /home/nao/.go/bin/ubume
```

Si vous voulez forcer la suppression, utilisez l'option --force.
```shell
$ gup remove --force gal
removed /home/nao/.go/bin/gal
```

### Vérifier si le binaire est la dernière version
Si vous voulez savoir si le binaire est la dernière version, utilisez la sous-commande check. La sous-commande check vérifie si le binaire est la dernière version et affiche le nom du binaire qui doit être mis à jour.
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

Comme les autres sous-commandes, vous pouvez seulement vérifier les binaires spécifiés.
```shell
$ gup check lazygit mimixbox
check binary under $GOPATH/bin or $GOBIN
[1/2] github.com/jesseduffield/lazygit (Already up-to-date: v0.32.2 / go1.22.4)
[2/2] github.com/nao1215/mimixbox (current: v0.32.1, latest: v0.33.2 / go1.22.4)

If you want to update binaries, the following command.
           $ gup update mimixbox
```

### Sortie silencieuse pour les grands ensembles d'outils
Par défaut, `check` et `update` affichent chaque binaire, ce qui devient bruyant lorsque vous avez beaucoup d'outils installés. Passez `--quiet` (`-q`) pour supprimer les lignes des binaires déjà à jour et n'afficher que les binaires qui ont été mis à jour (ou pour lesquels une mise à jour est disponible) ainsi que les échecs, suivis d'un résumé sur une seule ligne. Les erreurs sont toujours écrites sur STDERR, elles restent donc visibles. Lorsque `--json` est également fourni, `--quiet` est ignoré et le tableau JSON complet est affiché.
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

### Sortie JSON lisible par machine (pour le scripting / CI)
`list`, `check` et `update` acceptent `--json`, qui affiche un tableau JSON au lieu de la sortie lisible par un humain (qui reste le comportement par défaut).

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

Chaque élément possède les champs suivants : `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`/`pinned`), `current_version`, `latest_version` (vide pour `list` et pour les packages épinglés), `pinned_version` (présent uniquement pour `channel: "pinned"`), `current_go_version`, `installed_go_version`, `status`, `error` (omis lorsqu'il est absent) et `hint` (une suggestion d'étape suivante, présente uniquement lorsqu'une s'applique à l'erreur). `status` vaut `installed` (list), `up-to-date`, `update-available` (check), `updated` (update), `pinned`/`pin-mismatch` (un package épinglé à / éloigné de sa version épinglée) ou `error`.

Le tableau est toujours du JSON valide, y compris en cas d'échecs partiels (ces packages obtiennent `"status": "error"` ; le détail de l'erreur est également envoyé sur STDERR afin que STDOUT reste du JSON pur). Les codes de sortie sont inchangés : `check` signalant `update-available` se termine toujours avec `0`.

### Comportement dans un environnement vide
Un environnement global vide (aucun binaire encore installé par `go install`) est traité comme une condition normale de première exécution, et non comme une erreur :

- `list`, `check` et `update` se terminent avec `0`, en affichant une brève note d'information (ou un tableau vide valide `[]` avec `--json`).
- `export` se termine avec `0` et écrit un `gup.json` vide.

Nommer un binaire qui n'est pas installé, ou exclure tous les binaires, reste une erreur d'utilisation et se termine avec `1`.

### Sous-commandes Export／Import
Utilisez export/import si vous voulez installer les mêmes binaires golang sur plusieurs systèmes.
`gup.json` stocke l'import path de chaque outil, la `version` enregistrée du binaire et son `channel` de mise à jour (`latest` / `main` / `master` / `pinned`). Pour `channel: "pinned"`, `version` est la version cible exacte à laquelle l'outil est maintenu ; pour les autres canaux, c'est la version qui a été enregistrée au moment de l'export. `import` installe exactement la version écrite dans le fichier, et un package épinglé reste épinglé après l'import.

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

Par défaut :
- `gup export` écrit dans `$XDG_CONFIG_HOME/gup/gup.json`
- `gup import`, `gup check` et `gup update` détectent automatiquement le chemin du fichier dans cet ordre :
  1) `$XDG_CONFIG_HOME/gup/gup.json` (s'il existe)
  2) `./gup.json` (s'il existe)

Si les deux fichiers `gup.json` (celui au niveau utilisateur et `./gup.json`) existent, `import`, `check` et `update` échouent immédiatement et vous demandent de lever l'ambiguïté avec `--file`, au lieu d'en choisir un silencieusement. Vous pouvez toujours forcer le chemin avec `--file` (`-f`).

`schema_version` vaut `1` pour les configurations sans package épinglé et `2` dès qu'un package est épinglé, de sorte qu'un environnement qui n'utilise aucun épinglage continue de produire le format `1` que les anciennes versions de gup peuvent lire. gup lit à la fois `1` et `2`. Le canal `pinned` n'est valide que sous `schema_version: 2` ; une entrée `pinned` sous `schema_version: 1`, un package épinglé sans version concrète, une valeur de canal inconnue, ou un `schema_version` non pris en charge sont rejetés.

Un `gup.json` malformé ou invalide (JSON invalide, un canal inconnu, un `schema_version` non pris en charge, ou un épinglage non sûr) est traité comme une erreur plutôt que silencieusement ignoré : `check`, `update` et `export` échouent immédiatement et nomment le fichier fautif, de sorte que les canaux enregistrés par package ne sont jamais discrètement rétrogradés vers `latest` parce que la configuration n'a pas pu être analysée. Un canal inconnu n'est jamais normalisé en `latest`.

`gup export` résout toujours les canaux de mise à jour enregistrés à partir du `gup.json` canonique au niveau utilisateur ; `--file`/`--output` ne changent que la destination d'export, donc exporter vers un nouveau fichier ne réinitialise jamais le canal d'un paquet à `latest`.

```shell
※ Environnement A (par exemple ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.json

※ Environnement B (par exemple debian)
$ gup import
```

Alternativement, `export` peut afficher le contenu de `gup.json` sur STDOUT avec `--output`. `import` peut lire un fichier spécifique avec `--file`.
```shell
※ Environnement A (par exemple ubuntu)
$ gup export --output > gup.json

※ Environnement B (par exemple debian)
$ gup import --file=gup.json
```

### Migrer les binaires vers un nouveau $GOBIN

```shell
gup migrate BEFORE_PATH AFTER_PATH [BINARY...]
```

`gup migrate` réinstalle les binaires Go situés sous `BEFORE_PATH` dans `AFTER_PATH`, en utilisant le `import path@version` exact enregistré dans la build info de chaque binaire (il ne met jamais silencieusement à niveau vers `@latest`). En interne, il définit simplement `GOBIN` sur `AFTER_PATH` et exécute le flux normal de `go install`, de sorte que les binaires sont recompilés avec la toolchain Go actuellement utilisée.

#### Pourquoi c'est utile (par exemple avec `mise`)

Lorsque vous gérez Go avec [`mise`](https://mise.jdx.dev/), mettre à jour Go peut changer le chemin réel de `$GOBIN` pour chaque version de Go. Par conséquent, les outils que vous avez installés sous le `$GOBIN` précédent ne sont plus visibles pour le nouveau Go. `gup migrate` vous permet de réinstaller le même ensemble d'outils Go de l'ancien `$GOBIN` vers le nouveau :

```shell
# Réinstaller tous les outils go-install de l'ancien GOBIN vers le nouveau GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrer uniquement des binaires spécifiques
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` fonctionne uniquement en ajout (add-only) :

- Il ne supprime ni ne nettoie jamais les fichiers dans `AFTER_PATH`.
- Les binaires qui existent déjà dans `AFTER_PATH` sont ignorés par défaut. Utilisez `--force` pour les réinstaller par-dessus.
- `AFTER_PATH` est créé automatiquement lorsqu'il n'existe pas.
- `BEFORE_PATH` et `AFTER_PATH` doivent être des répertoires différents.

Les binaires dont l'import path ou la version ne peut pas être résolu, ainsi que les builds de développement (`devel` / `(devel)`), sont ignorés au lieu d'être mis à niveau, de sorte que les builds locaux ou non reproductibles ne sont jamais cassés.

Flags pris en charge : `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`), `--force`.

### Générer les pages de manuel (pour linux, mac)
La sous-commande man génère les pages de manuel sous `/usr/share/man/man1` par défaut. Si `MANPATH` est défini, gup écrit dans le répertoire `man1` sous chaque entrée, le créant lorsqu'il n'existe pas encore. Une destination non inscriptible se termine par une erreur claire.
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

### Générer le fichier d'autocomplétion shell (pour bash, zsh, fish et PowerShell)
`completion` affiche les scripts d'autocomplétion sur STDOUT quand vous fournissez un nom de shell.
Pour installer les fichiers d'autocomplétion dans votre environnement utilisateur pour bash/fish/zsh, utilisez `--install`.
Pour PowerShell, redirigez la sortie vers un fichier `.ps1` et chargez-le depuis votre profil.

```shell
$ gup completion bash > gup.bash
$ gup completion zsh > _gup
$ gup completion fish > gup.fish
$ gup completion powershell > gup.ps1

# Installer automatiquement les fichiers dans les chemins utilisateur par défaut
$ gup completion --install
```

`--install` nécessite que `HOME` soit définie ; elle échoue immédiatement (sans écrire de fichiers dans le répertoire courant) lorsque `HOME` est vide, et se termine avec un code non nul si un fichier de complétion ne peut pas être écrit.

### Notification de bureau
Si vous utilisez gup avec l'option --notify, la commande gup vous notifie sur votre bureau si la mise à jour a réussi ou échoué après la fin de la mise à jour.
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)

### Désactiver la sortie colorée
gup colore sa sortie par défaut. Pour désactiver les couleurs, passez `--no-color` ou définissez la variable d'environnement `NO_COLOR` sur une valeur non vide (en suivant la convention [NO_COLOR](https://no-color.org/)). C'est utile lorsque vous redirigez la sortie, dans les journaux de CI, ou avec `NO_COLOR` défini globalement.
```shell
$ gup update --no-color
$ NO_COLOR=1 gup update
```


## gup vs. `go tool`
Le [`go tool`](https://go.dev/doc/modules/managing-dependencies#tools) intégré à Go 1.24 gère les outils limités à un seul projet et enregistrés dans le `go.mod` de ce projet ; ces outils n'existent donc qu'à l'intérieur de ce module. gup gère les binaires installés à l'échelle du système sous `$GOBIN`, les commandes que vous exécutez depuis n'importe quel répertoire et que vous conservez aux côtés de vos dotfiles, éventuellement épinglées aux versions dont vous dépendez. Utilisez `go tool` pour l'outillage propre à chaque projet et gup pour votre boîte à outils globale.

## Comparaison des fonctionnalités

| Fonctionnalité | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| Mise à jour en parallèle | Oui | Non | Manuel |
| Temps de mise à jour (9 binaires) | 0.7s | 2.9s | 2.9s |
| Canaux de mise à jour par package (`latest`/`main`/`master`) | Oui | Non | Manuel |
| Épinglage / verrouillage de version | Oui | Non | Manuel |
| Export/import de l'ensemble d'outils | Oui | Non | Manuel |
| Migration des binaires vers un nouveau `$GOBIN` | Oui | Non | Manuel |
| Sortie JSON lisible par machine (`--json`) | Oui | Non | Non |
| Génération/installation de l'autocomplétion shell | Oui | Non | Non |
| `update` réinstalle les binaires déjà à jour | Non | Oui | Oui |
| `migrate --force` réinstalle lorsque la cible existe déjà | Oui | Non | Manuel |
| Diagnostics d'échec / suggestions d'étapes suivantes | Oui | Oui | Non |
| Prise en charge de `NO_COLOR` | Oui | Oui | — |

*Temps de mise à jour : 9 binaires, chacun avec une version plus récente disponible ; gup en parallèle, les autres en séquentiel. AMD Ryzen AI Max+ 395 / go 1.26.4, médiane de 5 exécutions avec un cache de modules chaud ; les temps dépendent du temps de build et du CPU.*

## FAQ

### `gup` échoue avec `fatal: not a git repository`
Vous êtes probablement sur oh-my-zsh, qui fournit un alias `gup` pour `git pull --rebase` masquant cette commande ([#16](https://github.com/nao1215/gup/issues/16), [#204](https://github.com/nao1215/gup/issues/204)). Supprimez ou renommez cet alias, ou exécutez gup avec une barre oblique inverse en préfixe pour le contourner :
```shell
$ \gup update
```

## Contribuer
Tout d'abord, merci de prendre le temps de contribuer ! ❤️  Voir [CONTRIBUTING.md](../../CONTRIBUTING.md) pour plus d'informations.
Le workflow de développement, la checklist qualité et la gestion des outils sont documentés dans [CONTRIBUTING.md](../../CONTRIBUTING.md).
Les contributions ne sont pas seulement liées au développement. Par exemple, GitHub Star me motive à développer !

### Historique des étoiles
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

## Contact
Si vous souhaitez envoyer des commentaires tels que "trouvé un bug" ou "demande de fonctionnalités supplémentaires" au développeur, veuillez utiliser l'un des contacts suivants.

- [GitHub Issue](https://github.com/nao1215/gup/issues)

Vous pouvez utiliser la sous-commande bug-report pour envoyer un rapport de bug.
```
$ gup bug-report
※ Open GitHub issue page by your default browser
```

## LICENCE
Le projet gup est sous licence selon les termes de [la Licence Apache 2.0](../../LICENSE).


## Contributeurs ✨

Merci à ces personnes formidables ([clé des emojis](https://allcontributors.org/docs/en/emoji-key)) :

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

Ce projet suit la spécification [all-contributors](https://github.com/all-contributors/all-contributors). Les contributions de toute sorte sont les bienvenues !
