# Scoop bucket

This directory is gup's [Scoop](https://scoop.sh/) bucket. Scoop accepts any git
repository that has a `bucket/` directory, so the manifest lives here rather
than in a second repository whose only content would be one generated file:

```shell
scoop bucket add nao1215 https://github.com/nao1215/gup
scoop install nao1215/gup
```

`gup.json` is written by GoReleaser on every tagged release. It carries that
release's Windows `amd64`/`arm64` archive URLs and their SHA-256 hashes, so it
cannot be edited by hand without going stale at the next tag. Change
[`.goreleaser.yml`](../.goreleaser.yml) instead.

Scoop and [winget](https://github.com/microsoft/winget-pkgs) are both published
from the same release: winget carries the `nao1215.gup` package identifier, this
bucket carries the `nao1215/gup` Scoop app. Installing through one does not
register the other, so pick whichever package manager you already use.

The manifest does not exist until the first release that generates it.
