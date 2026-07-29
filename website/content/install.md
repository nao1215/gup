---
title: Install
description: Install gup with go install, Homebrew, winget, mise, nix, aqua, or a prebuilt .deb/.rpm/.apk package, and verify the release signatures.
---

gup runs on Linux, macOS, and Windows. It shells out to `go`, so a Go toolchain
has to be on `PATH` whichever way you install gup itself.

## go install

```shell
go install github.com/nao1215/gup@latest
```

Building from source needs Go 1.25 or newer. On an older Go, take a prebuilt
binary or a package below.

## Package managers

```shell
brew install nao1215/tap/gup
```

```shell
winget install --id nao1215.gup
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

## Prebuilt packages and binaries

[The release page](https://github.com/nao1215/gup/releases) carries `.deb`,
`.rpm`, and `.apk` packages plus archives for every supported platform.

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
