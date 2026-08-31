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
- pushes the winget manifests to `nao1215/winget-pkgs` and opens the pull request against [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs);
- opens a `scoop-gup-<version>` pull request on this repository with the updated [Scoop manifest](../bucket/README.md), which is the one step of a release that is finished by hand.

Packaging that needs nothing from us: the [homebrew-core formula](https://formulae.brew.sh/formula/gup) carries `autobump`, so Homebrew opens its own bump pull request from the new tag, and the AUR packages (`gup`, `gup-bin`) plus the nixpkgs `gogup` attribute are maintained by other people on their own schedule.

## Required secrets
- `GITHUB_TOKEN`: provided automatically; used to create the GitHub Release.
- `TAP_GITHUB_TOKEN`: a token with write access to `nao1215/homebrew-tap`,
  used to push the updated formula.
- `WINGET_GITHUB_TOKEN`: a classic token with the `public_repo` scope, used to commit the winget manifests to `nao1215/winget-pkgs` and open the upstream pull request. A fine-grained token does not work here: it can only be scoped to repositories you own, and opening the pull request is a write against microsoft/winget-pkgs. A failure is logged without failing the release, so a rejected or delayed pull request never blocks a version.
- `SCOOP_GITHUB_TOKEN`: optional. The token that opens the Scoop manifest pull request on this repository; `WINGET_GITHUB_TOKEN` is used when it is unset, and already has the access. It cannot be the workflow's built-in `GITHUB_TOKEN`: GitHub starts no workflow runs for events that token causes, so the pull request would have all eight required checks reported as never-run and could not be merged by anyone. If you narrow it to its own secret, it needs write access to `nao1215/gup` and permission to open pull requests there.

### Creating them

Two of the three are personal access tokens you create by hand. Which *kind* of
token is not a preference:

| Secret | Writes to | Kind | Permissions |
|:--|:--|:--|:--|
| `TAP_GITHUB_TOKEN` | `nao1215/homebrew-tap` | fine-grained | Contents: Read and write |
| `WINGET_GITHUB_TOKEN` | `nao1215/winget-pkgs`, and a pull request on `microsoft/winget-pkgs` | **classic** | `public_repo` |
| `SCOOP_GITHUB_TOKEN` | this repository (a branch, and a pull request) | fine-grained | Contents: Read and write, Pull requests: Read and write |

**A fine-grained token** ([new token](https://github.com/settings/personal-access-tokens/new)):
set *Resource owner* to `nao1215`, *Repository access* to **Only select
repositories** and pick the target, then grant the permissions above under
*Repository permissions*. `Metadata: Read-only` is added for you. One token can
cover several repositories, so the Scoop token can be issued once and reused.

**A classic token** ([new token](https://github.com/settings/tokens/new)): tick
`public_repo` and nothing else.

Only winget needs a classic token, and it is not a shortcut: a fine-grained token
can only be scoped to repositories you own, and opening the pull request is a
write against microsoft/winget-pkgs.

Do **not** add the `workflow` scope to it. The release log carries a warning from
the fork-sync step without it:

> could not sync fork: 422 refusing to allow a Personal Access Token to create or
> update workflow `…` without `workflow` scope

That step is a convenience — GoReleaser pushes the branch and opens the pull
request regardless — while the scope would let a token that lives in CI rewrite
the workflows of every public repository it can write to. The warning is the
cheaper of the two.

### Storing them

```shell
gh secret set SCOOP_GITHUB_TOKEN --repo nao1215/gup
```

With no value on the command line it prompts for one, which keeps the token out
of your shell history and out of `ps`. Prefer that to `--body`.

A personal account has no organization-level Actions secrets, so each repository
needs its own copy. The token itself can be shared between them.

### Checking them before a release needs them

`gh secret list --repo nao1215/gup` shows names and update times; the values
cannot be read back. So a wrong token is not discovered until a tag is pushed,
which is the worst moment to discover it — a publish failure lands *after* the
GitHub Release exists and *before* provenance is attested, and provenance cannot
be added to a published tag afterwards (see [If a release fails](#if-a-release-fails)).

Check the token where you created it, before storing it:

```shell
GH_TOKEN=<value> gh api user --jq .login
```

To prove it can actually write, create a throwaway ref and delete it again:

```shell
SHA=$(gh api repos/nao1215/gup/git/ref/heads/main --jq .object.sha)
GH_TOKEN=<value> gh api -X POST repos/nao1215/gup/git/refs -f ref=refs/heads/token-check -f sha=$SHA
GH_TOKEN=<value> gh api -X DELETE repos/nao1215/gup/git/refs/heads/token-check
```

Pull-request permission has no equally cheap check; confirm it was granted when
the token was created.

### Expiry

A fine-grained token lasts a year at most, and an expired one fails exactly the
way a wrong one does — at publish time, on a release that has already been
created. Give all three the same expiry date and put it in a calendar, so they
are renewed together rather than discovered one at a time.

## The winget pull request
A **stable** tagged release opens a pull request on microsoft/winget-pkgs; a moderator merges it once their validation pipeline passes, usually within a day. A pre-release tag opens nothing: `skip_upload: auto` skips the winget pipe whenever the tag carries a pre-release suffix, because the community repository takes stable versions only.

Until this was wired up a third-party bot submitted gup's manifests on its own schedule, which is why v1.5.1, v1.6.0, and v1.7.0 never reached winget. The bot skips a version that is already present, so publishing from the release simply takes over.

## After releasing
- Merge the `scoop-gup-<version>` pull request. It is the only publishing step a
  human finishes, because `main` is protected and the release job is not exempt.
  Until it merges, `scoop install nao1215/gup` still installs the previous
  version.
- Check the [Releases page](https://github.com/nao1215/gup/releases) for the
  generated notes and artifacts.
- Verify a downloaded artifact as described in
  [Verifying release integrity](../README.md#verifying-release-integrity).
- Confirm `brew upgrade gup` picks up the new version. The tap formula is pushed by the release job; the homebrew-core bump lands separately once Homebrew's autobump pull request merges.
- Confirm the winget pull request was opened, under [pull requests authored by nao1215](https://github.com/microsoft/winget-pkgs/pulls/nao1215). A winget failure is logged without failing the release, so a green release job does not by itself mean the submission happened.
- If no pull request appears, recover it by hand: the manifests were still generated under `dist/winget/manifests/n/nao1215/gup/<version>/`, and the three files can be committed to a `gup-<version>` branch on `nao1215/winget-pkgs` and submitted against `microsoft/winget-pkgs` `master`. A failure at the push step points at the token's scope; a failure only at the pull-request step points at a fine-grained token being used where a classic one is required.

## If a release fails
- Re-run the failed job from the Actions tab once the cause is fixed.
- If the tag itself is wrong, delete it locally and remotely, then tag again:
  ```shell
  git tag -d vX.Y.Z
  git push origin :refs/tags/vX.Y.Z
  ```
