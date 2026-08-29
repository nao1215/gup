---
title: Install
description: Install gup with go install, Homebrew, winget, Scoop, mise, nix, aqua, the AUR, or a prebuilt .deb/.rpm/.apk package, and verify the release signatures.
---

gup runs on Linux, macOS, and Windows. It shells out to `go`, so a Go toolchain
has to be on `PATH` whichever way you install gup itself.

## Supported Go versions

Unit tests run on Go 1.25, 1.26, and 1.27 across Linux, macOS, and Windows, with
a separate job tracking the latest Go release. `go.mod` declares Go 1.25 as the
minimum, so building from source needs Go 1.25 or newer.

The prebuilt release binaries are built with the latest Go 1.27 patch release.
Go 1.27 dropped support for macOS 12 and earlier, so the macOS release binaries
require macOS 13 Ventura or newer; on an older macOS, build from source with a Go
version that still supports it.

## go install

```shell
go install github.com/nao1215/gup@latest
```

Building from source needs Go 1.25 or newer. On an older Go, take a prebuilt
binary or a package below.

## Package managers

gup is in homebrew-core, so Homebrew needs no tap:

```shell
brew install gup
```

The GoReleaser-built formula in `nao1215/tap` remains published as an
alternative. It installs the prebuilt release binary rather than building from
source:

```shell
brew install nao1215/tap/gup
```

```shell
winget install --id nao1215.gup
```

On Windows, [Scoop](https://scoop.sh/) installs gup from the repository's own
bucket:

```shell
scoop bucket add nao1215 https://github.com/nao1215/gup
scoop install nao1215/gup
```

```shell
mise use -g gup@latest
```

```shell
nix profile install nixpkgs#gogup
```

gup is in the [aqua](https://aquaproj.github.io/) standard registry:

```shell
aqua g -i nao1215/gup
```

On Arch Linux, two community-maintained AUR packages exist:
[`gup`](https://aur.archlinux.org/packages/gup) builds from source and
[`gup-bin`](https://aur.archlinux.org/packages/gup-bin) installs the release
binary.

```shell
paru -S gup      # or: yay -S gup
paru -S gup-bin  # prebuilt binary
```

## Prebuilt packages and binaries

[The release page](https://github.com/nao1215/gup/releases) carries `.deb`,
`.rpm`, and `.apk` packages plus `.tar.gz` (Linux, macOS) and `.zip` (Windows)
archives. Every artifact is built for `amd64` and `arm64`.

Download the one matching your distribution and architecture, then:

```shell
sudo dpkg -i gup_1.8.1_linux_amd64.deb                 # Debian, Ubuntu
sudo rpm -Uvh gup_1.8.1_linux_amd64.rpm                # Fedora, RHEL, openSUSE
sudo apk add --allow-untrusted gup_1.8.1_linux_amd64.apk  # Alpine Linux
```

Substitute the release you downloaded for `1.8.1`, and `arm64` for `amd64` where
applicable. The packages also install the bash, fish, and zsh completion files.

## Verify what you downloaded

Every release ships a cosign-signed `checksums.txt`, an SPDX SBOM per archive,
and SLSA build provenance attested through GitHub OIDC.

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/gup/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

```shell
gh attestation verify gup_1.0.0_linux_amd64.tar.gz --repo nao1215/gup
```

## Shell completion

```shell
gup completion --install
```

That writes bash, fish, and zsh completion into the paths your shell already
reads. For PowerShell, redirect a `.ps1` and source it from your profile. See
[Completion and man pages](/cookbook/#completion-and-man-pages).

## If `gup` runs `git pull --rebase`

oh-my-zsh ships a `gup` alias. Remove or rename it, or bypass it once:

```shell
\gup update
```
