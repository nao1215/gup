<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-13-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![reviewdog](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/reviewdog.yml)
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/gup/coverage.svg)
[![gosec](https://github.com/nao1215/gup/actions/workflows/security.yml/badge.svg)](https://github.com/nao1215/gup/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nao1215/gup.svg)](https://pkg.go.dev/github.com/nao1215/gup)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/gup)](https://goreportcard.com/report/github.com/nao1215/gup)
![GitHub](https://img.shields.io/github/license/nao1215/gup)

[日本語](../ja/README.md) | [Русский](../ru/README.md) | [中文](../zh-cn/README.md) | [한국어](../ko/README.md) | [Español](../es/README.md) | [English](../../README.md)

# gup - Mettre à jour les binaires installés par "go install"

![sample](../img/sample.png)

La commande **gup** met à jour les binaires installés par "go install" vers la dernière version. gup met à jour tous les binaires en parallèle, donc très rapidement. Elle fournit également des sous-commandes pour manipuler les binaires sous \$GOPATH/bin (\$GOBIN). C'est un logiciel multiplateforme qui fonctionne sur Windows, Mac et Linux.

Si vous utilisez oh-my-zsh, alors gup a un alias configuré. L'alias est `gup - git pull --rebase`. Par conséquent, assurez-vous que l'alias oh-my-zsh est désactivé (par exemple $ \gup update).


## OS supportés (tests unitaires avec GitHub Actions)
- Linux
- Mac
- Windows

## Comment installer
### Utiliser "go install"
Si vous n'avez pas l'environnement de développement golang installé sur votre système, veuillez installer golang depuis le [site officiel golang](https://go.dev/doc/install).
```
go install github.com/nao1215/gup@latest
```

### Utiliser homebrew
```shell
brew install nao1215/tap/gup
```

### Installer depuis un package ou un binaire
[La page de release](https://github.com/nao1215/gup/releases) contient des packages aux formats .deb, .rpm et .apk. La commande gup utilise la commande go en interne, donc l'installation de golang est requise.


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

### Mettre à jour les binaires avec @main ou @master
Si vous voulez mettre à jour les binaires avec @master ou @main, vous pouvez spécifier l'option -m ou --master.
```shell
$ gup update --main=gup,lazygit,sqly
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
### Sous-commandes Export／Import
Vous utilisez les sous-commandes export／import si vous voulez installer les mêmes binaires golang sur plusieurs systèmes. Par défaut, la sous-commande export exporte le fichier vers $XDG_CONFIG_HOME/gup/gup.conf. Si vous voulez connaître la [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html), consultez ce lien. Après avoir placé gup.conf dans la même hiérarchie de chemins sur un autre système, vous exécutez la sous-commande import. gup commence l'installation
selon le contenu de gup.conf.

```shell
※ Environnement A (par exemple ubuntu)
$ gup export
Export /home/nao/.config/gup/gup.conf

※ Environnement B (par exemple debian)
$ ls /home/nao/.config/gup/gup.conf
/home/nao/.config/gup/gup.conf
$ gup import
```

Alternativement, la sous-commande export affiche les informations de package (c'est la même chose que gup.conf) que vous voulez exporter sur STDOUT si vous utilisez l'option --output. La sous-commande import peut aussi spécifier le chemin du fichier gup.conf si vous utilisez l'option --input.
```shell
※ Environnement A (par exemple ubuntu)
$ gup export --output > gup.conf

※ Environnement B (par exemple debian)
$ gup import --input=gup.conf
```

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
Generate /usr/share/man/man1/gup-remove.1.gz
Generate /usr/share/man/man1/gup-update.1.gz
Generate /usr/share/man/man1/gup-version.1.gz
Generate /usr/share/man/man1/gup.1.gz
```

### Générer le fichier d'autocomplétion shell (pour bash, zsh, fish)
La sous-commande completion génère des fichiers d'autocomplétion shell pour bash, zsh et fish. Si le fichier d'autocomplétion shell n'existe pas dans le système, le processus de génération commencera. Pour activer la fonctionnalité d'autocomplétion, redémarrez le shell.

```shell
$ gup completion
create bash-completion file: /home/nao/.bash_completion
create fish-completion file: /home/nao/.config/fish/completions/gup.fish
create zsh-completion file: /home/nao/.zsh/completion/_gup
```

### Notification de bureau
Si vous utilisez gup avec l'option --notify, la commande gup vous notifie sur votre bureau si la mise à jour a réussi ou échoué après la fin de la mise à jour.
```shell
$ gup update --notify
```
![success](../img/notify_success.png)
![warning](../img/notify_warning.png)


## Contribuer
Tout d'abord, merci de prendre le temps de contribuer ! ❤️  Voir [CONTRIBUTING.md](../../CONTRIBUTING.md) pour plus d'informations.
Les contributions ne sont pas seulement liées au développement. Par exemple, GitHub Star me motive à développer !

### Historique des étoiles
[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/gup&type=Date)](https://star-history.com/#nao1215/gup&Date)

### Pour les développeurs
Lors de l'ajout de nouvelles fonctionnalités ou de la correction de bugs, veuillez écrire des tests unitaires. Le sqly est testé unitairement pour tous les packages comme le montre la carte arborescente des tests unitaires ci-dessous.

![treemap](../img/cover-tree.svg)

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
