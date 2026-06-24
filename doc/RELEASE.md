# Release process

This describes how a gup release is cut. It is for maintainers.

## Overview
Releases are driven by Git tags. Pushing a tag that matches `v*` triggers the
[release workflow](../.github/workflows/release.yml), which runs
[GoReleaser](https://goreleaser.com/) using [.goreleaser.yml](../.goreleaser.yml).
There is no manual upload step.

## Versioning
- gup follows [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.
- Release notes are generated from commit messages, so use
  [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
  `perf:`, `docs:`, and `!` for breaking changes). `chore:`, `ci:`, `style:`,
  and `test:` commits are excluded from the notes.

## Before tagging
- Make sure `main` is green (build, unit tests, e2e, lint, gitleaks).
- The release smoke workflow builds the GoReleaser artifacts on every PR and
  push to `main`, so packaging regressions are caught before tagging.
- Locally you can dry-run the build with `goreleaser release --snapshot --clean`.

## Cut a release
```shell
git switch main
git pull --ff-only
git tag vX.Y.Z
git push origin vX.Y.Z
```
The release workflow then:
- builds binaries for linux, macOS, and Windows;
- publishes archives, `deb`/`rpm`/`apk` packages, and `checksums.txt`;
- signs the checksums with cosign (keyless) and attaches an SBOM;
- attests build provenance via GitHub OIDC;
- updates the Homebrew tap (`nao1215/homebrew-tap`).

## Required secrets
- `GITHUB_TOKEN`: provided automatically; used to create the GitHub Release.
- `TAP_GITHUB_TOKEN`: a token with write access to `nao1215/homebrew-tap`,
  used to push the updated formula.

## After releasing
- Check the [Releases page](https://github.com/nao1215/gup/releases) for the
  generated notes and artifacts.
- Verify a downloaded artifact as described in
  [Verifying release integrity](../README.md#verifying-release-integrity).
- Confirm `brew upgrade gup` picks up the new version.

## If a release fails
- Re-run the failed job from the Actions tab once the cause is fixed.
- If the tag itself is wrong, delete it locally and remotely, then tag again:
  ```shell
  git tag -d vX.Y.Z
  git push origin :refs/tags/vX.Y.Z
  ```
