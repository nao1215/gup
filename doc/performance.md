# Performance investigation (issue #271)

This document records the measurement-driven prioritization of the performance
candidates raised in issue #271. The rule is: **measure first, then implement
only changes with a clear, non-noise payoff; explicitly reject or defer the
rest.**

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
`GetPackageInformation` for typical GOBIN sizes — **97% of the cost at n=3, 60%
at n=30, 38% at n=150**. Binary scanning itself (`buildinfo.ReadFile`, already
parallel) and directory listing are cheap.

## Decision table

| Candidate | Decision | Rationale (data) |
|-----------|----------|------------------|
| Skip `go version` for commands that never read GoVersion (`list`, `export`, `migrate`) | **Selected** | Removes the dominant cost. `GetPackageInformation` drops 68–97%; end-to-end `gup list` over 30 binaries went **4.5ms → 2.6ms/run (~42%)**. Zero behavior change (those commands don't show Go-version data); one small function. |
| Avoid post-install buildinfo reread | **Already done** | `update` already guards `SetLatestVer()` for the common `@latest` path. |
| Memoize `GOBIN`/`GOPATH`/`go version` within a process | **Rejected** | `GoBin` is 90ns (env read; the `go env GOPATH` subprocess only runs when `GOPATH` is unset). `go version` is called once per command. Memoization saves nothing measurable. |
| Completion / `BinaryPathList` scanning on large GOBIN | **Rejected** | 60µs at n=150 — noise relative to everything else. |
| Persist latest-version lookups across process runs | **Deferred** | Potentially large win for repeated `check`/`update`, but introduces staleness (can report a wrong "latest"), needs TTL/invalidation, and a new failure mode. Makes behavior harder to reason about; needs a design before implementation. |
| Batch `go list -m` version resolution | **Rejected (measured regression, no crossover)** | A naive microbenchmark looked compelling — 30 *sequential* `go list -m` calls ~1819ms vs ~88ms for one batched `go list -m -e -json` (~20x). **That comparison is wrong for gup**, which resolves versions **in parallel** across the `-j` worker pool. Measured properly (warm cache, real modules, `xargs -P` simulating the pool), the batched single call is slower at *every* size tested — there is no crossover threshold, because `go list -m` resolves modules sequentially internally while the pool resolves them concurrently. End-to-end `gup check` on 3 distinct modules also regressed (**68ms → 95ms/run**). Rejected and reverted. This is the issue's thesis in action. |
| Split concurrency policy (metadata vs install) | **Deferred** | Marginal expected benefit; a single `-j` is simpler to reason about. Revisit only if network end-to-end data shows a clear win. |

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
