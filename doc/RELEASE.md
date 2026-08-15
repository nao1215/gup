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
- updates the Homebrew tap (`nao1215/homebrew-tap`);
- pushes the winget manifests to `nao1215/winget-pkgs` and opens the pull request against [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs).

## Required secrets
- `GITHUB_TOKEN`: provided automatically; used to create the GitHub Release.
- `TAP_GITHUB_TOKEN`: a token with write access to `nao1215/homebrew-tap`,
  used to push the updated formula.
- `WINGET_GITHUB_TOKEN`: a classic token with the `public_repo` scope, used to commit the winget manifests to `nao1215/winget-pkgs` and open the upstream pull request. A fine-grained token does not work here: it can only be scoped to repositories you own, and opening the pull request is a write against microsoft/winget-pkgs. A failure is logged without failing the release, so a rejected or delayed pull request never blocks a version.

## The winget pull request
A **stable** tagged release opens a pull request on microsoft/winget-pkgs; a moderator merges it once their validation pipeline passes, usually within a day. A pre-release tag opens nothing: `skip_upload: auto` skips the winget pipe whenever the tag carries a pre-release suffix, because the community repository takes stable versions only.

Until this was wired up a third-party bot submitted gup's manifests on its own schedule, which is why v1.5.1, v1.6.0, and v1.7.0 never reached winget. The bot skips a version that is already present, so publishing from the release simply takes over.

## After releasing
- Check the [Releases page](https://github.com/nao1215/gup/releases) for the
  generated notes and artifacts.
- Verify a downloaded artifact as described in
  [Verifying release integrity](../README.md#verifying-release-integrity).
- Confirm `brew upgrade gup` picks up the new version.
- Confirm the winget pull request was opened, under [pull requests authored by nao1215](https://github.com/microsoft/winget-pkgs/pulls/nao1215). A winget failure is logged without failing the release, so a green release job does not by itself mean the submission happened.
- If no pull request appears, recover it by hand: the manifests were still generated under `dist/winget/manifests/n/nao1215/gup/<version>/`, and the three files can be committed to a `gup-<version>` branch on `nao1215/winget-pkgs` and submitted against `microsoft/winget-pkgs` `master`. A failure at the push step points at the token's scope; a failure only at the pull-request step points at a fine-grained token being used where a classic one is required.

## If a release fails
- Re-run the failed job from the Actions tab once the cause is fixed.
- If the tag itself is wrong, delete it locally and remotely, then tag again:
  ```shell
  git tag -d vX.Y.Z
  git push origin :refs/tags/vX.Y.Z
  ```
