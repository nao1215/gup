# Cookbook

Copyable one-liners for the Go tools in your `$GOBIN`. Every recipe runs as
shown; swap the tool names for yours.

`go install` drops a binary in `$GOBIN` and then forgets it. gup reads the build
info stamped into each binary — import path, version, Go toolchain — so the set
you already installed is the manifest, and there is nothing to declare first.

## Find a recipe by task

| I want to | Go to |
|:--|:--|
| See what `go install` left in my `$GOBIN` | [See what you have](#see-what-you-have) |
| Find out what is out of date | [Find what is out of date](#find-what-is-out-of-date) |
| Update the whole set at once | [Update everything](#update-everything) |
| Update some tools and skip others | [Update only some tools](#update-only-some-tools) |
| Keep one tool on an exact version | [Hold a tool at a version](#hold-a-tool-at-a-version) |
| Track `@main` instead of a release | [Follow a branch](#follow-a-branch) |
| Install the same tools on another machine | [Reproduce the set elsewhere](#reproduce-the-set-elsewhere) |
| Move my tools after a Go upgrade | [Move to a new $GOBIN](#move-to-a-new-gobin) |
| Delete a tool I no longer use | [Remove a tool](#remove-a-tool) |
| Drive gup from a script or from CI | [Script it](#script-it) |
| Work out why an update failed | [When an update fails](#when-an-update-fails) |
| Get tab completion and man pages | [Completion and man pages](#completion-and-man-pages) |

## See what you have

Every binary under `$GOBIN`, with the import path and version it was built from:

```shell
gup list
```

![list](/img/list.gif)

The same as JSON, one object per tool:

```shell
gup list --json
```

```json
[
  {
    "name": "gup",
    "import_path": "github.com/nao1215/gup",
    "module_path": "github.com/nao1215/gup",
    "channel": "latest",
    "current_version": "v1.0.0",
    "current_go_version": "go1.26.4",
    "installed_go_version": "go1.26.4",
    "status": "installed"
  }
]
```

Just the names, for the next command in the pipe:

```shell
gup list --json | jq -r '.[].name'
```

A binary that was not installed by `go install` is reported, not hidden — it
simply has no import path to reinstall from.

## Find what is out of date

`check` compares each tool against its module and against the Go toolchain you
are running now. It never installs anything:

```shell
gup check
```

Only the tools that need something, plus a one-line summary:

```shell
gup check --quiet
```

```text
github.com/nao1215/gup (current: v0.7.0, latest: v0.7.1 / go1.26.4)

If you want to update binaries, run the following command.
           $ gup update gup
gup: 1 update available, 8 up-to-date, 0 failed
```

Check two tools instead of the whole set:

```shell
gup check gopls staticcheck
```

The names of everything with an update waiting:

```shell
gup check --json | jq -r '.[] | select(.status == "update-available") | .name'
```

A tool built with an older Go toolchain counts as out of date, because it is.
To ignore that and compare versions only:

```shell
gup check --ignore-go-update
```

`check` exits `0` even when it finds updates — finding them is the job, not a
failure.

## Update everything

```shell
gup update
```

![update](/img/update.gif)

gup runs the updates in parallel, so the whole set finishes in about the time
the slowest build takes.

See what would happen first:

```shell
gup update --dry-run
```

Version updates only, leaving Go-toolchain rebuilds alone:

```shell
gup update --ignore-go-update
```

Fewer parallel builds, and a ceiling on any single one:

```shell
gup update --jobs 4 --timeout 5m
```

Tell me when the long run is done:

```shell
gup update --notify
```

## Update only some tools

Name them:

```shell
gup update gopls staticcheck
```

Or update everything except the ones you name:

```shell
gup update --exclude gopls,golangci-lint
```

`--exclude` combines with `--dry-run`, so you can confirm the skip list before
anything is built:

```shell
gup update --exclude gopls --dry-run
```

## Hold a tool at a version

Pin the tool your CI is pinned to, then keep updating everything else:

```shell
gup pin golangci-lint v1.62.0
gup update
```

The `tool@version` form works too:

```shell
gup pin golangci-lint@v1.62.0
```

A pinned tool is installed as `go install <import_path>@<version>` and is never
resolved to `@latest`. The pin lives in `gup.json` under `channel: "pinned"`, so
it survives `export`/`import`.

Which tools are pinned, and to what:

```shell
gup check --json | jq -r '.[] | select(.channel == "pinned") | "\(.name) \(.pinned_version)"'
```

`gup check` reports a pinned tool as `pinned` when it sits at its version, and
`pin-mismatch` when something moved it. Let it float again with:

```shell
gup unpin golangci-lint
```

## Follow a branch

Some tools ship from a branch, not a tag:

```shell
gup update --main gup
gup update --master sqly
gup update --latest air
```

`--main` falls back to `@master` only when the repository has no `main` branch.
A build failure on `@main` is reported as a build failure; it never silently
installs `@master` instead.

The channel is written to `gup.json` and reused by later runs, so you set it
once:

```shell
gup update --main gup,lazygit --master sqly --latest air
```

After that, a plain `gup update` keeps each tool on the channel you chose.

## Reproduce the set elsewhere

On the machine that has the tools:

```shell
gup export
```

On the machine that wants them:

```shell
gup import
```

`export` writes `$XDG_CONFIG_HOME/gup/gup.json`; `import` reads it (or `./gup.json`),
and installs the exact version recorded for each tool. Pinned tools stay pinned.

Keep the file in your dotfiles instead:

```shell
gup export --output > gup.json
gup import --file gup.json
```

Check what an import would install before it installs it:

```shell
gup import --file gup.json --dry-run
```

## Move to a new $GOBIN

When a Go upgrade changes where `$GOBIN` points — this happens with
[mise](https://mise.jdx.dev/) on every Go version — the old tools are still on
disk, just invisible to the new toolchain. Reinstall them into the new
directory, at the versions they already had:

```shell
gup migrate ~/.local/share/mise/installs/go/1.24.0/bin ~/.local/share/mise/installs/go/1.25.0/bin
```

Only some of them:

```shell
gup migrate /old/gobin /new/gobin gopls air
```

`migrate` is add-only: it never deletes anything in the destination, and it
skips a binary that is already there. Look before you leap, then overwrite on
purpose:

```shell
gup migrate /old/gobin /new/gobin --dry-run
gup migrate /old/gobin /new/gobin --force
```

Local `devel` builds and binaries with no resolvable version are skipped rather
than upgraded, so nothing you built by hand is quietly replaced.

## Remove a tool

`remove` asks before each deletion:

```shell
gup remove subaru ubume
```

```text
gup:CHECK: remove /home/nao/.go/bin/subaru? [Y/n] Y
removed /home/nao/.go/bin/subaru
```

Do not ask:

```shell
gup remove --force gal
```

With no terminal to ask on — a pipe, a CI job — `remove` fails fast instead of
blocking forever on a prompt nobody can answer. Pass `--force` there.

## Script it

`list`, `check`, and `update` all take `--json`, and the array stays valid JSON
even when some packages fail (those get `"status": "error"`):

```shell
gup check --json > check.json
```

Report only the failures, with their message:

```shell
gup update --json | jq -r '.[] | select(.status == "error") | "\(.name): \(.error)"'
```

Fail a CI job when anything is behind:

```shell
test -z "$(gup check --json | jq -r '.[] | select(.status == "update-available") | .name')"
```

Errors always go to STDERR, so STDOUT stays pure JSON and can be piped straight
into `jq`.

For human-readable logs, drop the noise and the escape codes:

```shell
gup update --quiet --no-color
```

```shell
NO_COLOR=1 gup update
```

An empty `$GOBIN` is a normal first run, not an error: `list`, `check`, and
`update` exit `0` (and print `[]` with `--json`).

## When an update fails

gup turns the Go toolchain's output into one next step, printed on STDERR after
the error:

```text
gup:ERROR: [1/1] tool: can't install gup.test/moved/cmd/tool:
go: gup.test/moved/cmd/tool@latest: module gup.test/moved@latest found (v1.1.0), but does not contain package gup.test/moved/cmd/tool
gup:HINT : The module no longer provides this command at its import path. The project likely moved to a new major version (e.g. a `/v2` module path) or relocated the command; check its current install instructions and reinstall with the new path.
```

The same text is the `hint` field under `--json`, so a script can collect them:

```shell
gup update --json | jq -r '.[] | select(.hint) | "\(.name): \(.hint)"'
```

Hints cover major-version moves, relocated commands, `replace` directives,
binaries that never came from `go install`, missing branches and tags,
unreachable or private repositories, permission and network errors, and a Go
toolchain that is too old. When gup has nothing reliable to add, it says
nothing.

A failure is per-package: the other tools in the run still update, and the
process exits non-zero so CI notices.

## Completion and man pages

Install completion for the shells of the platform you are on — bash, fish and zsh
on Linux and macOS, PowerShell on Windows:

```shell
gup completion --install
```

On Windows that writes `gup.completion.ps1` beside your PowerShell profile and
adds one guarded dot-source line to the profile, inside a marked block, so
nothing else in the profile changes and re-running never duplicates the entry.
Reload with `. $PROFILE` or open a new window.

Or print it and place it yourself:

```shell
gup completion bash > gup.bash
gup completion zsh > _gup
gup completion fish > gup.fish
gup completion powershell > gup.ps1
```

Completion is not just subcommands: `gup update`, `gup remove`, and `gup pin`
complete the binary names actually present in your `$GOBIN`.

Man pages, on Linux and macOS:

```shell
sudo gup man
```

`man` honors `MANPATH` when it is set, and fails with a clear error instead of a
stack trace when the target directory is not writable.
