# Performance investigation (issue #271)

This document records the measurement-driven prioritization of the performance
candidates raised in issue #271. The rule is: measure first, then implement
only changes with a clear, non-noise payoff; explicitly reject or defer the
rest.

## How to reproduce

- Package-level benchmarks (local-only, stable):

  ```sh
  go test ./internal/goutil/ -run '^$' -bench 'Benchmark(GetPackageInformation|BinaryPathList|GetInstalledGoVersion|GoBin)' -benchmem
  ```

- Command-level harness (builds gup, synthesizes a GOBIN, times `list` /
  `check` / `update --dry-run` for several sizes and `-j` values):

  ```sh
  sh scripts/perf.sh                 # defaults: sizes 3 30 150, 10 runs
  RUNS=20 SIZES="3 50 200" CMDS="list" sh scripts/perf.sh
  ```

  `check`/`update` resolve versions over the network, so treat their absolute
  numbers as relative, same-machine measurements. `list` is local-only.

## Baseline measurements

Linux, warm caches, `-benchtime=30x -count=3` (representative medians):

| Function                               |   n=3 |  n=30 |  n=150 |
|----------------------------------------|------:|------:|-------:|
| `GetPackageInformation` (with go ver)  | 1.90ms| 2.59ms| 3.76ms |
| `GetPackageInformationWithoutGoVersion`| 0.05ms| 0.41ms| 1.20ms |
| `GetInstalledGoVersion` (`go version`) |        ~1.53ms (per call)      |||
| `BinaryPathList`                       | 12µs  | 27µs  | 60µs   |
| `GoBin`                                |        90ns (env read, no subprocess) |||

Key finding: the one-shot `go version` subprocess (~1.5ms) dominates
`GetPackageInformation` for typical GOBIN sizes — 97% of the cost at n=3, 60%
at n=30, 38% at n=150. Binary scanning itself (`buildinfo.ReadFile`, already
parallel) and directory listing are cheap.

## Decision table

| Candidate | Decision | Rationale (data) |
|-----------|----------|------------------|
| Skip `go version` for commands that never read GoVersion (`list`, `export`, `migrate`) | Selected | Removes the dominant cost. `GetPackageInformation` drops 68–97%; end-to-end `gup list` over 30 binaries went 4.5ms → 2.6ms/run (~42%). Zero behavior change (those commands don't show Go-version data); one small function. |
| Avoid post-install buildinfo reread | Already done | `update` already guards `SetLatestVer()` for the common `@latest` path. |
| Memoize `GOBIN`/`GOPATH`/`go version` within a process | Rejected | `GoBin` is 90ns (env read; the `go env GOPATH` subprocess only runs when `GOPATH` is unset). `go version` is called once per command. Memoization saves nothing measurable. |
| Completion / `BinaryPathList` scanning on large GOBIN | Rejected | 60µs at n=150 — noise relative to everything else. |
| Persist latest-version lookups across process runs | Deferred | Potentially large win for repeated `check`/`update`, but introduces staleness (can report a wrong "latest"), needs TTL/invalidation, and a new failure mode. Makes behavior harder to reason about; needs a design before implementation. |
| Batch `go list -m` version resolution | Rejected (measured regression, no crossover) | A naive microbenchmark looked compelling — 30 *sequential* `go list -m` calls ~1819ms vs ~88ms for one batched `go list -m -e -json` (~20x). That comparison is wrong for gup, which resolves versions in parallel across the `-j` worker pool. Measured properly (warm cache, real modules, `xargs -P` simulating the pool), the batched single call is slower at *every* size tested — there is no crossover threshold, because `go list -m` resolves modules sequentially internally while the pool resolves them concurrently. End-to-end `gup check` on 3 distinct modules also regressed (68ms → 95ms/run). Rejected and reverted. This is the issue's thesis in action. |
| Split concurrency policy (metadata vs install) | Deferred | Marginal expected benefit; a single `-j` is simpler to reason about. Revisit only if network end-to-end data shows a clear win. |

### Batched vs parallel `go list -m` (why batching was rejected)

Warm module cache, real modules from `go.sum`, `xargs -P<j>` simulating gup's
worker pool versus one batched `go list -m -e -json` call. Lower is better.

| Modules (N) | -j | parallel per-module | batched 1 call |
|------------:|---:|--------------------:|---------------:|
| 4  | 8 |  65ms |  80ms |
| 8  | 8 |  69ms | 179ms |
| 16 | 8 | 133ms | 311ms |
| 32 | 8 | 258ms | 563ms |
| 4  | 4 |  68ms | 108ms |
| 8  | 4 | 127ms | 153ms |
| 16 | 4 | 242ms | 307ms |
| 32 | 4 | 486ms | 547ms |

The parallel pool wins at every size: `go list -m` resolves the listed modules
sequentially, so one batched call is effectively serial, while gup already runs
the per-module calls concurrently. There is no N at which batching catches up in
the tested range, so there is no useful threshold to gate on.

## `gup update` investigation (real-install)

The optimizations above target `list`/`export`/`migrate`. This section targets
`update` itself, measured with real installs (not just `--dry-run`).

### Harness

`scripts/perf_update.sh` is offline and reproducible: it serves synthetic
modules (each published at v1.0.0 and v1.0.1) from a local file `GOPROXY`,
installs v1.0.0 into a temp GOBIN, then times `gup update` actually compiling
and installing v1.0.1. Real network is avoided so numbers are stable.

```sh
sh scripts/perf_update.sh
SIZES="30" JOBS="8 0" RUNS=7 sh scripts/perf_update.sh   # -j 0 = gup default (NumCPU)
```

### Measurement table

Linux (NumCPU=32), warm module cache, median of 3 runs, milliseconds. "real
install" upgrades every binary v1.0.0 -> v1.0.1; "no install" is a second
`update` where everything is already current (resolution only).

| Scenario (modules) | -j=1 | -j=4 | -j=8 | -j=default | no install |
|--------------------|-----:|-----:|-----:|-----------:|-----------:|
| 3                  |  204 |   70 |   68 |         63 |          8 |
| 30                 | 1710 |  474 |  261 |        187 |         34 |
| 150                | 8790 | 2260 | 1238 |        822 |         72 |

### Findings

- `update` is install-bound. For 150 modules, the real-install pass is 822ms vs
  72ms when nothing needs installing — `go install` (compilation) is ~91% of the
  time. These synthetic modules are trivial; real tools compile slower, so the
  install share is even higher in practice.
- `go install` parallelizes well across `-j`, and the default `-j=NumCPU` is the
  fastest at every size, so the common path is already near-optimal.
- Version resolution (`go list -m`, run concurrently and deduped per module) is a
  small fraction, and the all-up-to-date pass is already fast.

### Selected changes

None. No candidate produced a reproducible end-to-end win above noise.

### Rejected changes (with data)

- Batch `go list -m` resolution: slower than the existing parallel pool at every
  size (see the batching table above); also off the critical path for real
  installs.
- Install the exact resolved version instead of `@latest`: measured 248ms vs
  245ms for 30 modules (~1%, noise) — with a warm cache, `go install @latest`
  resolves from cache.
- Skip `go version` under `--ignore-go-update`, memoize `GOBIN`/`GOPATH`: each is
  well under ~2ms and not the dominant cost.
- "Fast mode" (install first, diff later): the install is the cost, so doing it
  first cannot remove it; it only loses the up-to-date skip.

### Deferred changes

- Persist latest-version lookups across runs: only helps the already-fast
  resolution-only pass (34–72ms), not the install-bound common case, and adds
  staleness/correctness risk. Not worth it on this data.
- Batch multiple binaries of the same module into one `go install`: only helps
  users with several binaries from one module (uncommon) and the harness uses
  one binary per module, so no representative measurement exists. Revisit with a
  multi-binary fixture.
- Split resolve vs install concurrency: install is already at the optimal `-j`
  and resolution overlaps with it, so there is no measured headroom.

### Risks

None — this investigation added only the harness and this document; no
behavior-affecting code changed for `update`.

## Implemented change

`GetPackageInformationWithoutGoVersion` was added alongside `GetPackageInformation`,
sharing a private `collectPackageInformation` core. `list`, `export`, and
`migrate` now use it; `check` and `update` keep `GetPackageInformation` because
they compare Go toolchain versions.

Measured impact:

| Metric | Before | After | Δ |
|--------|-------:|------:|---|
| `GetPackageInformation` n=3 | 1.90ms | 0.05ms | −97% |
| `GetPackageInformation` n=30 | 2.59ms | 0.41ms | −84% |
| `GetPackageInformation` n=150 | 3.76ms | 1.20ms | −68% |
| end-to-end `gup list` (n=30) | 4.5ms/run | 2.6ms/run | −42% |
