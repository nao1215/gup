# Benchmarks

gup is measured the way a user runs it, one process per call from start to exit, with [himorime](https://github.com/nao1215/himorime). himorime builds gup, runs each command in interleaved rounds, and reports latency, CPU time and peak RSS.

```console
$ go install github.com/nao1215/himorime@latest
$ make bench            # himorime run bench
$ make bench-compare    # himorime compare --against main bench
$ make bench-docs       # gup against go-global-update and a go install loop, written below
```

No benchmark touches the network. `check` and `update` resolve and install synthetic modules from a file `GOPROXY` in the benchmark's own directory: `bench/proxygen` publishes `example.test/mNNNN` at v1.0.0 and v1.0.1, and `gen.sh` installs them, so a binary installed at v1.0.0 has an update. The module and build caches are in that directory too, so a run neither reads nor fills yours.

## Regression suite: himorime.yaml

`make bench-compare` checks main out into a temporary Git worktree, builds it and your working tree, and measures both in the same rounds, so a background job slows both revisions instead of one. `BASE=v1.9.2 make bench-compare` compares against another revision, and `himorime run --filter '^update' bench` measures the update benchmarks only.

On a pull request, `.github/workflows/bench.yml` runs `himorime ci bench` the same way, with the base of the pull request as the base, on one runner. The job fails when a command is slower, uses more CPU time or more memory than the base beyond the tolerance in `himorime.yaml` with 95% confidence. A difference too close to call is reported as inconclusive and does not fail the job. The comparison is on the job summary page and the JSON report is kept as an artifact.

| Benchmark | Commands | Measures |
|-----------|----------|----------|
| `version` | `gup version` | starting gup |
| `list 3`, `list 150` | `gup list` | reading the build information of 3 and 150 binaries, in parallel |
| `check 3` | `gup check` | one `go version` and a version lookup per module |
| `check 30` | `gup check` with `-j 1`, `-j 4` and the default | looking up 30 modules with one worker, four, and one per CPU |
| `update 30` | `gup update` with `-j 1`, `-j 4`, `-j 8` and the default | compiling and installing 30 updates; the GOBIN is put back to v1.0.0 before every run, outside the measured time |
| `update 150` | `gup update` | 150 updates with the default number of workers |
| `update 30 up to date` | `gup update` on binaries already at v1.0.1, and `gup update --dry-run` | resolving versions without installing anything |
| `check error` | `gup check no-such-binary` | the error path, exit 1 |

Numbers from different machines are not comparable; compare revisions on one machine, as `make bench-compare` and CI do.

## Comparison suite: compare/himorime.yaml

`compare/himorime.yaml` updates the same 30 binaries from v1.0.0 to v1.0.1 with `gup update`, [go-global-update](https://github.com/Gelio/go-global-update) and a `go install` loop, from the same local proxy. Before measuring, a setup step runs each tool once and checks that every binary ended at v1.0.1. gup updates in parallel; the other two install one binary after another. The modules are tiny and the build cache is warm, so the times are mostly `go install`'s own work, not the network.

It is not a CI gate: CI only validates it. `make bench-docs` needs go-global-update on PATH (`go install github.com/Gelio/go-global-update@v0.2.5`), runs the suite, and replaces what is between the markers below, with the machine and the versions under the tables.

<!-- himorime:begin comparison -->

### gup and other updaters of go install binaries

Updating 30 binaries installed by go install from v1.0.0 to v1.0.1, compiling and installing each.

#### Latency

| Benchmark | Command | Median | P95 | Mean | Stddev | Min | Max | Runs | Relative |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| update 30 | gup | 196.35ms | 213.42ms | 197.87ms | 11.03ms | 184.90ms | 220.16ms | 10 | 0.11x |
| update 30 | go-global-update | 1.75s | 1.84s | 1.76s | 58.91ms | 1.68s | 1.85s | 10 | 0.98x |
| update 30 | go-install-loop | 1.79s | 1.82s | 1.77s | 44.53ms | 1.69s | 1.83s | 10 | 1.00x |

Relative is the median divided by the baseline command's median, or by the fastest command's.

#### CPU

| Benchmark | Command | User | System | Total | Total p95 | Utilization |
|---|---|--:|--:|--:|--:|--:|
| update 30 | gup | 3.11s | 951.93ms | 4.06s | 4.14s | 2063.0% |
| update 30 | go-global-update | 2.26s | 957.43ms | 3.21s | 3.37s | 183.2% |
| update 30 | go-install-loop | 2.25s | 871.92ms | 3.15s | 3.22s | 176.5% |

CPU values are medians over runs of the process tree. Utilization is CPU time divided by wall-clock time; above 100% means more than one CPU was busy. Process tree: the command plus every descendant its parent waited for (rusage); a descendant left running or reaped by init is not counted.

#### Memory

| Benchmark | Command | Peak RSS (median) | Peak RSS (max) |
|---|---|--:|--:|
| update 30 | gup | 45.25MiB | 46.04MiB |
| update 30 | go-global-update | 44.28MiB | 45.48MiB |
| update 30 | go-install-loop | 44.62MiB | 45.23MiB |

Peak RSS is a resident set size, not the heap size of a language runtime. Peak RSS is the largest peak of any single process of the tree (rusage ru_maxrss), not the combined memory of processes running at the same time.

Measured with himorime v0.2.0 on linux/amd64, AMD RYZEN AI MAX+ 395 w/ Radeon 8060S (32 logical CPUs), head d67bb9151c7a, seed 3376384279169423.

- go-global-update: go-global-update version v0.2.5
- go: go version go1.26.4 linux/amd64

<!-- himorime:end comparison -->
