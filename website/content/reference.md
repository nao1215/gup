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

`schema_version` is `1` while nothing is pinned and `2` once anything is, so
without pins gup keeps writing files that older releases can read. gup reads
both. A malformed file, an unknown `channel`, an unsupported `schema_version`,
or a `pinned` entry with no concrete version is an error; gup never downgrades
a saved channel to `latest`.

A `pinned` entry's `version` must name one fixed Go module version: a full
version (`v1.62.0`), a prerelease (`v2.0.0-rc.1`), a `+incompatible` version of
a v2+ module, or a pseudo-version (`v0.0.0-20240102150405-abcdef123456`) to pin
a commit. Branch names, commit hashes, abbreviated versions such as `v1` or
`v1.2`, and version queries such as `>=v1.2.0` are rejected by `gup pin` and by
every command that reads the file, because the go command would resolve them
again on every run. The error names the file, the tool and the
value; fix it by setting that entry's `version` to one of the accepted forms, or
its `channel` to `latest` to unpin it.

## Running two gup commands at once

Commands that change state lock each resource they write, so a second command
refuses to start instead of interleaving with the first:

| Command | What it locks |
|:--|:--|
| `update` | `$GOBIN` and the `gup.json` it may write |
| `import` | `$GOBIN` |
| `remove` | `$GOBIN` |
| `migrate` | `BEFORE_PATH` and `AFTER_PATH` |
| `export`, `pin` | `$GOBIN` and the `gup.json` they write |
| `unpin` | the `gup.json` it writes |

Without the lock, two `gup update` runs would both install and both write
`gup.json`, which would then describe only the run that finished last, and
`gup remove` could delete a binary that a concurrent `gup update` is
reinstalling. `export`, `pin` and `migrate` also lock directories they only
read, because their output is derived from what they read there. `unpin` only
edits `gup.json`, so it does not wait on `$GOBIN`. A `$GOBIN` that does not
exist yet is created so that it can be locked.

When two commands overlap, the second exits non-zero and names the process in
the way:

> another gup process is already running (pid 40321 on carbon, running "gup update",
> since 2026-08-29T17:04:11+09:00). gup serializes commands that change your $GOBIN or
> gup.json, so wait for it to finish and run this command again. The lock is held by the
> operating system, not by /home/you/go/bin/.gup.lock, so it is released the moment that
> process ends and there is never a file to delete by hand

Nothing that leaves state unchanged takes a lock: `--dry-run` runs,
`export --output`, and the read-only commands (`list`, `check`, `version`,
`completion`, `man`, `bug-report`). gup replaces `gup.json` with an atomic
rename, so a reader sees either the previous complete file or the next one,
even when the destination is read-only (it is replaced in place). `gup
completion --install` and `gup man` write files without a lock: both write
atomically and produce byte-identical content on every run.

### How the lock works

The lock is the operating system's (`flock` on Linux and macOS, `LockFileEx` on
Windows), taken on a file gup keeps open while it holds the resource. The kernel
releases it when the process ends, however it ends, so a gup killed with
`kill -9` or a machine that lost power leaves no stale lock, and an interrupted
`gup update` can be followed by another one immediately. Ctrl-C does not
release the lock early: the command unwinds and releases it on the way out, so
the resource is not freed while binaries and `gup.json` are still being
written.

The lock files sit next to what they guard: `$GOBIN/.gup.lock` and
`<gup.json>.lock`. A lock kept in the config directory would not work, because
a per-project `XDG_CONFIG_HOME` still shares one `$GOBIN` with every other
project, and two commands given the same `--file` may come from different
config directories. `.gup.lock` is dot-prefixed, so `gup list` never shows it.

Leave the lock files in place. Between commands they are empty and hold
nothing. gup never deletes them, because deleting a file another gup has
already opened would let two processes lock two different files at one path.
For the same reason `gup remove .gup.lock` is refused, as is any other name for
the same file: a hard link, a Windows spelling with a trailing dot, or an 8.3
alias like `GUPLOC~1.LOC`.

A lock belongs to a file, not to the path that names it. Two arguments that
reach one directory (`gup migrate ~/go/bin ~/bin` where `~/bin` is a symlink to
`~/go/bin`, or a `$GOBIN` spelled two ways on the case-insensitive filesystems
of macOS and Windows) take one lock, so gup never waits for itself. When a
command locks several resources, it takes them in the order of their filesystem
identities, which every gup agrees on however it spelled the paths.

For the same reason gup resolves `--file` to the file's one canonical name
before deriving the lock from it. On Windows, NTFS also accepts an 8.3 alias
like `GUP~1.JSO`, and Win32 strips trailing dots and spaces, so
`--file gup.json.` writes `gup.json`; both once produced a lock file of their
own (`GUP~1.JSO.lock`, `gup.json..lock`). A `gup.json` reached through a
symlink is locked at the file the write lands on.

A hard link has no canonical name, so gup refuses a `--file` with a second
name. gup could not rewrite such a file correctly anyway: the rewrite renames a
temporary file over `gup.json`, which leaves the other name on the old
contents.

> can not lock /home/you/.config/gup/gup.json: the file has a second name (a hard link), so gup
> can not tell which lock protects it: remove the extra name while no gup is running, or point
> gup at a file that has only one

gup truncates its lock file to record who holds it, so it refuses a lock path
that is a symlink, which could point at a file that is not gup's. A hard link
passes every symlink check, so gup also refuses a lock file with more than one
name; the files it creates have exactly one. Otherwise a `.gup.lock` made into
a second name for a binary in `$GOBIN` would get that binary truncated.

> can not open the gup lock file /home/you/go/bin/.gup.lock: the lock path is a hard link
> to another file, and gup will not truncate a file it does not own: delete the lock file while no
> gup is running, or point gup at a directory it owns

If gup cannot work out what to lock, it stops instead of running unlocked. A
directory it could not create (a parent that refuses it, a read-only
filesystem, a full disk) is treated as an error, because the command could run
there while another gup does. A path that cannot be a directory, such as a
regular file where `$GOBIN` or `AFTER_PATH` should be, is left to the command,
which reports it and writes nothing.

### Where the lock does not reach

On a network filesystem (NFS, SMB, sshfs), what `flock` and `LockFileEx` do is
up to the mount: some map them to a server-side lock, some keep them local, some
ignore them. Two gups on one machine are still serialized; two on different
machines sharing the mount may not be.

Deleting a lock file while gup runs does not stop the running command, whose
lock is on an open handle, but it frees the name. The next gup then creates a
new file there and locks it, and two commands change one `$GOBIN`.

gup v1.8.1 and earlier take no lock, so an older gup on your `PATH` does not
wait for a current one.

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
| `installed_go_version` | Go toolchain on this machine; for an `updated` binary, the Go it was rebuilt with |
| `status` | `installed`, `up-to-date`, `update-available`, `updated`, `pinned`, `pin-mismatch`, `error` |
| `error` | Omitted when absent |
| `hint` | Next step for the error, when gup has one |

The array is valid JSON even on partial failure, and errors are also written to
STDERR so STDOUT stays parseable.

## Exit codes

| Code | When |
|:--|:--|
| `0` | The command did its job, including `check` finding updates, `list`, `export`, and `check` or `update` without named binaries on an empty `$GOBIN`, `update --exclude` leaving nothing to update, and `import` of a `gup.json` with no packages |
| `1` | A usage error, a config error, or at least one package failed |

Naming a binary that is not installed is a usage error, and gup suggests the
closest installed name when there is one. A name in `--exclude` that is not
installed is ignored (with a
"did you mean" warning on STDERR when it looks like a typo of an installed
binary), and excluding everything leaves nothing to do.
