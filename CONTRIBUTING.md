## Contributing to gup
Thank you for building gup with us.
Every report, patch, test, and review directly improves the daily workflow of Go developers.
Let's keep gup fast, safe, and reliable together.

## Contributing as a Developer
### 1. Start with clear communication
- Bug report: Use the issue template and include reproducible steps, expected behavior, and actual behavior.
- New feature: Open an issue first so we can agree on direction before implementation.
- Bug fix or improvement: Open a PR with a clear problem statement and solution summary.

### 2. Keep the quality bar high
- Add or update unit tests when you add features or fix bugs.
- Avoid regressions on supported OSes (Linux, macOS, Windows).
- Keep CLI behavior and error messages clear and consistent.

### 3. Run checks before opening a PR
```shell
make test
make vet
make fmt
make coverage-tree
```

`coverage-tree` generates the test treemap shown below.

![treemap](./doc/img/cover-tree.svg)

### 4. Check for known vulnerabilities
`govulncheck` scans gup against the official Go vulnerability database and
reports only advisories that are actually reachable from gup's code. It answers a
different question from the `gosec` linter golangci-lint already runs (insecure
code gup writes), so both are kept.

```shell
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
govulncheck ./...
```

CI runs the same pinned version against every supported Go version on pull
requests, on pushes to main, and daily
(`.github/workflows/govulncheck.yml`) — the schedule is what catches an advisory
published against a dependency gup already ships.

### 5. Run the end-to-end tests (optional but recommended for CLI changes)
gup has an offline end-to-end suite that exercises the real `gup` binary and the
real `go` toolchain against a self-contained module proxy, all inside a throwaway
temp tree. It never touches your real `$HOME`, `~/.config/gup`, or `$GOBIN`, and
needs no network access. The tests are plain-YAML specs run by
[atago](https://github.com/nao1215/atago).

The suite uses atago **v0.3.4+** features (a real-PTY step for the interactive
`gup remove` prompt, and golden-output snapshots), so an older atago will fail to
parse those specs. CI pins v0.21.0 via setup-atago.

```shell
# Install atago once, at the version CI pins, so local runs and CI agree
go install github.com/nao1215/atago@v0.21.0

# Run the specs classified for this operating system
make e2e

# Run one spec, or pass any atago flag through
go run ./e2e/runner --filter update

# Refresh golden snapshots after an intentional output change
go run ./e2e/runner --update-snapshots
```

The harness lives under `e2e/`. `e2e/runner` builds gup, starts the offline
module proxy (`e2e/testproxy`), warms a shared module cache, and runs the atago
specs in `e2e/atago/`. It is a Go program rather than a shell script because the
suite runs on Windows too, where a bash bootstrap would make the leg depend on
Git for Windows being installed. `e2e/run.sh` still works; it now just calls the
runner.

`e2e/os_matrix.tsv` classifies every spec by operating system. A spec that does
not run everywhere must say why, and `TestOSMatrix_classifiesEverySpec` fails
when a spec is added without a decision — that is what keeps the macOS and
Windows legs from quietly shrinking into an unstated subset. When you add a spec,
add its row.

CI (`.github/workflows/e2e.yml`) runs Linux, macOS and Windows on every pull
request, and again on a daily schedule.

### 6. Manage developer tools with Go tool declarations
gup manages helper tools via `go.mod` `tool` entries.
Use the command below to add or update tool dependencies:

```shell
make update-tools
```

## Documentation
`README.md` (English) is the only user-facing documentation file in the
repository; the [documentation website](https://nao1215.github.io/gup/) is built
from `website/`. gup used to carry translated READMEs under `doc/<lang>/`, but
they drifted behind English faster than they could be maintained, so they were
removed — English is the single source of truth.

When you change `README.md`:

- Keep the first-class sections intact. A CI test (`doc_sync_test.go`) enforces
  that `README.md` keeps its required sections, that each section still carries
  its commands and links, and that the install commands are current.
- Run `make test` so `doc_sync_test.go` runs before you open the PR.

## Releasing
Maintainers cut releases by pushing a `v*` tag. The process is documented in
[doc/RELEASE.md](./doc/RELEASE.md).

## Need help?
See [SUPPORT.md](./.github/SUPPORT.md) for where to ask questions and report problems.

## Contributing Outside of Coding
You can still make a huge impact even if you are not writing code:

- Give gup a GitHub Star
- Share gup with your team and community
- Open issues with clear reproduction steps
- Sponsor the project
