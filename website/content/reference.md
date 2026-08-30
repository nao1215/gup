---
title: Reference
description: Every gup subcommand and flag, the gup.json schema, the --json output fields, and the exit codes.
toc: true
---

Run `gup <command> --help` for the same information in your terminal. Recipes
that use these live in the [cookbook](/cookbook/).

## Commands

| Command | What it does |
|:--|:--|
| `gup update [BINARY...]` | Reinstall binaries at their update channel, in parallel |
| `gup check [BINARY...]` | Report what is out of date; installs nothing |
| `gup list` | List every binary under `$GOBIN` with its import path and version |
| `gup export` | Write the installed set to `gup.json` |
| `gup import` | Install the set recorded in `gup.json` |
| `gup pin TOOL[@VERSION] [VERSION]` | Hold a tool at an exact version |
| `gup unpin TOOL` | Let a pinned tool update again |
| `gup migrate BEFORE_PATH AFTER_PATH [BINARY...]` | Reinstall binaries from one `$GOBIN` into another |
| `gup remove BINARY...` | Delete binaries from `$GOBIN` |
| `gup completion [SHELL]` | Print or install shell completion |
| `gup man` | Generate man pages (Linux, macOS) |
| `gup version` | Print the version, same as `gup --version` |
| `gup bug-report` | Open a pre-filled GitHub issue |

`gup rm` is an alias for `gup remove`. `--no-color` works on every command, as
does the `NO_COLOR` environment variable.

## Flags

| Flag | Commands | Meaning |
|:--|:--|:--|
| `-n`, `--dry-run` | `update`, `import`, `migrate` | Report what would happen, change nothing |
| `-e`, `--exclude` | `update` | Comma-separated binaries to skip |
| `-f`, `--file` | `update`, `check`, `list`, `import`, `export`, `pin`, `unpin` | Use this `gup.json` instead of the auto-detected one |
| `-o`, `--output` | `export` | Print the config to STDOUT instead of writing it |
| `--json` | `update`, `check`, `list` | Machine-readable output |
| `-q`, `--quiet` | `update`, `check` | Drop up-to-date lines; keep changes, failures, and a summary |
| `-j`, `--jobs` | `update`, `check`, `import`, `migrate` | Parallel workers (default: CPU count) |
| `--timeout` | `update`, `check`, `import`, `migrate` | Per-package limit, e.g. `90s`, `5m`; `0` means none |
| `--ignore-go-update` | `update`, `check` | Compare versions only, ignore Go-toolchain rebuilds |
| `-m`, `--main` | `update` | Update these by `@main` (falls back to `@master` only when no `main` branch exists) |
| `--master` | `update` | Update these by `@master` |
| `--latest` | `update` | Update these by `@latest` |
| `-N`, `--notify` | `update`, `import`, `migrate` | Desktop notification when the run finishes |
| `--force` | `remove` (`-f`), `migrate` | Skip the confirmation / overwrite an existing binary |
| `--install` | `completion` | Write completion files to the user shell config paths |
| `--no-color` | all | Disable colorized output |
| `-V`, `--version` | root | Print the version |

`--json` wins over `--quiet` when both are given: you get the full array.

## gup.json

`export` writes it, `import` reads it, and `update`/`check` read the update
channel from it. The path is `$XDG_CONFIG_HOME/gup/gup.json`, or `./gup.json`,
in that order; `--file` overrides both. If both exist and no `--file` is given,
gup fails and asks you to choose rather than picking one.

```json
{
  "schema_version": 2,
  "packages": [
    {
      "name": "gal",
      "import_path": "github.com/nao1215/gal/cmd/gal",
      "version": "v1.1.1",
      "channel": "latest"
    },
    {
      "name": "golangci-lint",
      "import_path": "github.com/golangci/golangci-lint/cmd/golangci-lint",
      "version": "v1.62.0",
      "channel": "pinned"
    }
  ]
}
```

`schema_version` is `1` while nothing is pinned and `2` once anything is, so an
environment with no pins keeps writing files older gup releases can read. gup
reads both. A malformed file, an unknown `channel`, an unsupported
`schema_version`, or a `pinned` entry with no concrete version is an error, not
something to ignore — a saved channel is never quietly downgraded to `latest`.

## Running two gup commands at once

The commands that change state take a lock on each resource they write, so a
second one refuses to start instead of interleaving:

| Command | What it locks |
|:--|:--|
| `update` | `$GOBIN` and the `gup.json` it may write |
| `import` | `$GOBIN` |
| `remove` | `$GOBIN` |
| `migrate` | `AFTER_PATH` |
| `export`, `pin` | `$GOBIN` and the `gup.json` they write |
| `unpin` | the `gup.json` it writes |

The lock files sit next to what they guard: `$GOBIN/.gup.lock` and
`<gup.json>.lock`. `$GOBIN` and your config directory move independently, so a
per-project `XDG_CONFIG_HOME` still shares one `$GOBIN` with every other
project, and two commands given the same `--file` may come from different config
directories — a lock kept in the config directory would serialize neither.
`.gup.lock` is dot-prefixed, so `gup list` never shows it.

> another gup process is already running (pid 40321 on carbon, running "gup update",
> since 2026-08-29T17:04:11+09:00). gup serializes commands that change your $GOBIN or
> gup.json, so wait for it to finish and run this command again. If that process is
> gone, gup reclaims /home/you/go/bin/.gup.lock by itself

Two `gup update` runs at once would both install and then both write
`gup.json`, so the file would end up describing only whichever finished last;
`gup remove` deleting a binary a concurrent `gup update` is reinstalling is the
same collision with a worse result.

`export` and `pin` lock `$GOBIN` although they never write to it: what they
write describes it, so a `gup remove` deleting a binary halfway through would
record a tool that is no longer installed. `unpin` only names an entry in
`gup.json`, so it does not wait behind one.

Nothing that changes no state is blocked. `--dry-run` runs and `export --output`
take no lock, and neither do the read-only commands (`list`, `check`, `version`,
`completion`, `man`, `bug-report`): gup replaces `gup.json` with an atomic
rename, so a reader sees either the previous complete file or the next one -
including when the destination is read-only, which is replaced in place rather
than moved aside.

A lock left behind by a killed gup does not wedge the tool. The lock file
records the owning process, so one whose process is gone is reclaimed at once. A
lock gup cannot attribute that way — one written by another machine on a shared
home directory — is refreshed while its owner works and reclaimed once that
stops. A live local owner keeps its lock while it is suspended, so pausing an
update with Ctrl-Z does not let a second gup in — up to about an hour, after
which the heartbeat decides again, because a recycled PID would otherwise report
a long-dead gup as still running forever.

Ctrl-C does not release the lock; the process holding it does, by ending.
Deleting the file from a signal handler would free it while the command is still
installing binaries and rewriting `gup.json` on its way out. An interrupted
`gup update` stops its work, unwinds, and removes the lock file itself; a
command killed outright leaves the file behind for the next gup to reclaim.

## JSON output fields

| Field | Notes |
|:--|:--|
| `name` | Binary name in `$GOBIN` |
| `import_path` | What `go install` would be given |
| `module_path` | Module that provides it |
| `channel` | `latest`, `main`, `master`, or `pinned` |
| `current_version` | Version of the installed binary |
| `latest_version` | Empty for `list` and for pinned packages |
| `pinned_version` | Only for `channel: "pinned"` |
| `current_go_version` | Go toolchain the binary was built with |
| `installed_go_version` | Go toolchain on this machine |
| `status` | `installed`, `up-to-date`, `update-available`, `updated`, `pinned`, `pin-mismatch`, `error` |
| `error` | Omitted when absent |
| `hint` | Next step for the error, when gup has one |

The array is valid JSON even on partial failure, and errors are also written to
STDERR so STDOUT stays parseable.

## Exit codes

| Code | When |
|:--|:--|
| `0` | The command did its job — including `check` finding updates, and any command on an empty `$GOBIN` |
| `1` | A usage error, a config error, or at least one package failed |

Naming a binary that is not installed, or excluding every binary, is a usage
error.
