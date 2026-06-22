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

La commande **gup** met à jour les binaires installés par "go install" vers la dernière version. gup met à jour tous les binaires en parallèle, donc très rapidement. Elle fournit également des sous-commandes pour manipuler les binaires sous \$GOPATH/bin (\$GOBIN). C'est un logiciel multiplateforme qui fonctionne sur Windows, Mac et Linux.

Si vous utilisez oh-my-zsh, alors gup a un alias configuré. L'alias est `gup - git pull --rebase`. Par conséquent, assurez-vous que l'alias oh-my-zsh est désactivé (par exemple $ \gup update).

## OS supportés (tests unitaires avec GitHub Actions)
- Linux
- Mac
- Windows

## Comment installer
gup est aussi disponible via `winget`, `mise` et `nix`, en plus de `go install` et Homebrew.

### Utiliser "go install"
Si vous n'avez pas l'environnement de développement golang installé sur votre système, veuillez installer golang depuis le [site officiel golang](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```

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

### Installer depuis un package ou un binaire
[La page de release](https://github.com/nao1215/gup/releases) contient des packages aux formats .deb, .rpm et .apk. La commande gup utilise la commande go en interne, donc l'installation de golang est requise.

## Vérifier l'intégrité de la release
Chaque release est accompagnée de métadonnées de chaîne d'approvisionnement afin que vous puissiez vérifier ce que vous téléchargez :

- **Sommes de contrôle signées** — `checksums.txt` est signé avec [cosign](https://github.com/sigstore/cosign) (sans clé), produisant `checksums.txt.sig` et `checksums.txt.pem`.
- **SBOM** — un SPDX Software Bill of Materials est joint à chaque archive de release.
- **Provenance de build** — la provenance de build SLSA est attestée via GitHub OIDC.

Vérifiez les sommes de contrôle signées (puis comparez votre archive à `checksums.txt`) :

```shell
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
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

Chaque élément possède les champs suivants : `name`, `import_path`, `module_path`, `channel` (`latest`/`main`/`master`), `current_version`, `latest_version` (vide pour `list`), `current_go_version`, `installed_go_version`, `status` et `error` (omis lorsqu'il est absent). `status` vaut `installed` (list), `up-to-date`, `update-available` (check), `updated` (update) ou `error`.

Le tableau est toujours du JSON valide, y compris en cas d'échecs partiels (ces packages obtiennent `"status": "error"` ; le détail de l'erreur est également envoyé sur STDERR afin que STDOUT reste du JSON pur). Les codes de sortie sont inchangés : `check` signalant `update-available` se termine toujours avec `0`.

### Sous-commandes Export／Import
Utilisez export/import si vous voulez installer les mêmes binaires golang sur plusieurs systèmes.
`gup.json` stocke l'import path, la version du binaire et le canal de mise à jour (`latest` / `main` / `master`).
`import` installe exactement la version écrite dans le fichier.

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
- `gup import` détecte automatiquement le chemin du fichier dans cet ordre :
  1) `$XDG_CONFIG_HOME/gup/gup.json` (s'il existe)
  2) `./gup.json` (s'il existe)

Vous pouvez toujours forcer le chemin avec `--file`.

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

`gup migrate` réinstalle les binaires Go situés sous `BEFORE_PATH` dans `AFTER_PATH`,
en utilisant le `import path@version` exact enregistré dans la build info de chaque
binaire (il ne met jamais silencieusement à niveau vers `@latest`). En interne, il
définit simplement `GOBIN` sur `AFTER_PATH` et exécute le flux normal de `go install`,
de sorte que les binaires sont recompilés avec la toolchain Go actuellement utilisée.

#### Pourquoi c'est utile (par exemple avec `mise`)

Lorsque vous gérez Go avec [`mise`](https://mise.jdx.dev/), mettre à jour Go peut
changer le chemin réel de `$GOBIN` pour chaque version de Go. Par conséquent, les
outils que vous avez installés sous le `$GOBIN` précédent ne sont plus visibles pour
le nouveau Go. `gup migrate` vous permet de réinstaller le même ensemble d'outils Go
de l'ancien `$GOBIN` vers le nouveau :

```shell
# Réinstaller tous les outils go-install de l'ancien GOBIN vers le nouveau GOBIN
$ gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin

# Migrer uniquement des binaires spécifiques
$ gup migrate /old/gobin /new/gobin gopls air
```

`migrate` fonctionne uniquement en ajout (add-only) :

- Il ne supprime ni ne nettoie jamais les fichiers dans `AFTER_PATH`.
- Les binaires qui existent déjà dans `AFTER_PATH` sont ignorés par défaut. Utilisez
  `--force` pour les réinstaller par-dessus.
- `AFTER_PATH` est créé automatiquement lorsqu'il n'existe pas.
- `BEFORE_PATH` et `AFTER_PATH` doivent être des répertoires différents.

Les binaires dont l'import path ou la version ne peut pas être résolu, ainsi que les
builds de développement (`devel` / `(devel)`), sont ignorés au lieu d'être mis à
niveau, de sorte que les builds locaux ou non reproductibles ne sont jamais cassés.

Flags pris en charge : `--dry-run` (`-n`), `--notify` (`-N`), `--jobs` (`-j`),
`--force`.

### Générer les pages de manuel (pour linux, mac)
La sous-commande man génère les pages de manuel sous /usr/share/man/man1.
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


## Benchmark
gup exécute les mises à jour en parallèle, il termine donc plus vite que les outils qui mettent à jour les binaires un par un. Mise à jour de 9 binaires pour lesquels une version plus récente était disponible :

| Outil                                                         | Stratégie    | Temps |
| ------------------------------------------------------------- | ------------ | ----: |
| gup update                                                    | parallèle    |  0.7s |
| [go-global-update](https://github.com/Gelio/go-global-update) | séquentielle |  2.9s |
| boucle `go install`                                           | séquentielle |  2.9s |

Mesuré sur AMD Ryzen AI Max+ 395 (32 cœurs) / 64 Go de RAM / Ubuntu 26.04 / go 1.26.4, médiane de 5 exécutions avec un cache de modules Go chaud. Les temps dépendent du temps de build de chaque binaire et de votre CPU.

## Comparaison des fonctionnalités

| Fonctionnalité | gup | [go-global-update](https://github.com/Gelio/go-global-update) | `go install` loop |
| --- | :-: | :-: | :-: |
| Mise à jour en parallèle | Oui | Non | Manuel |
| Canaux de mise à jour par package (`latest`/`main`/`master`) | Oui | Non | Manuel |
| Export/import de l'ensemble d'outils | Oui | Non | Manuel |
| Migration des binaires vers un nouveau `$GOBIN` | Oui | Non | Manuel |
| Sortie JSON lisible par machine (`--json`) | Oui | Non | Non |
| Génération/installation de l'autocomplétion shell | Oui | Non | Non |
| `update` réinstalle les binaires déjà à jour | Non | Oui | Oui |
| `migrate --force` réinstalle lorsque la cible existe déjà | Oui | Non | Manuel |
| Diagnostics d'échec / suggestions d'étapes suivantes | Non | Oui | Non |
| Prise en charge de `NO_COLOR` | Oui | Oui | — |
| Aucun outil supplémentaire requis (toolchain officielle uniquement) | Non | Non | Oui |

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
