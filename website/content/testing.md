---
title: Testing
description: The test techniques gup uses, what each one catches, where the tests live, and the CI job that runs them.
toc: true
---

gup reads the build information of binaries it did not build, rewrites the
files in your `$GOBIN`, and drives `go install` for every tool you have. Each
technique below targets one way that can go wrong.

## Unit tests

Every package has table-driven tests next to its source. CI runs them with the
race detector on Linux, macOS and Windows, on every supported Go minor and on
the latest Go release (`.github/workflows/unit_test.yml`).

```shell
go test -race ./...
```

## End-to-end tests and snapshots

The specs in [`e2e/atago`](https://github.com/nao1215/gup/tree/main/e2e/atago)
run the real `gup` binary and the real `go` command against an offline module
proxy in a throwaway directory, using [atago](https://github.com/nao1215/atago).
Golden-output snapshots pin what `gup` prints, a pseudo terminal drives the
interactive `gup remove` prompt, and `e2e/os_matrix.tsv` records which specs
run on which operating system and why. CI runs them on Linux, macOS and
Windows for every pull request and every day (`.github/workflows/e2e.yml`).

```shell
go run ./e2e/runner
```

## Property-based tests

[`internal/goutil/property_test.go`](https://github.com/nao1215/gup/blob/main/internal/goutil/property_test.go)
checks that version comparison is a total order (reflexive, antisymmetric,
consistent with the parser) over generated module and Go versions.
[`internal/config/property_test.go`](https://github.com/nao1215/gup/blob/main/internal/config/property_test.go)
writes generated tool sets to `gup.json` and reads them back unchanged.

## Differential tests

[`internal/goutil/differential_test.go`](https://github.com/nao1215/gup/blob/main/internal/goutil/differential_test.go)
reads the same files with gup and with `go version -m`, the go command's own
reader of embedded build information, and fails when they disagree on whether
a binary is manageable, its import path, its module, its version or the Go that
built it. The files are the committed ELF and PE fixtures, files that are not
Go binaries, the toolchain's `go` and `gofmt`, and binaries built during the
test for Linux, macOS (Mach-O) and Windows. `go test -short` skips the builds.

The same file compares gup's Go toolchain ordering, which decides whether a
binary counts as built with an old Go, with `go/version.Compare` from the
standard library over release names generated with
[rapid](https://pkg.go.dev/pgregory.net/rapid). Every `go test` draws 100
cases; the nightly `rapid properties` job in `.github/workflows/fuzz.yml`
draws 100,000.

```shell
go test ./internal/goutil -run agreesWith -rapid.checks=100000
```

## Fuzzing

`FuzzParseConf` (gup.json), `FuzzParsePinArgs` (`gup pin` arguments) and
`FuzzValidatePinnedVersion` (pinned versions) run their seed corpus in every
`go test`. `.github/workflows/fuzz.yml` searches for new inputs every night.

```shell
go test -run '^$' -fuzz '^FuzzParseConf$' -fuzztime 1m ./internal/config
```

## Mutation testing

[gremlins](https://github.com/go-gremlins/gremlins) rewrites one expression at
a time and checks that some test fails. `.github/workflows/mutation.yml` mutates
the lines a pull request changes and the whole module every week; the report is
uploaded as the `mutation-report` artifact.

```shell
gremlins unleash --workers 2 --timeout-coefficient 10 .
```

Run it from a clean checkout: gremlins mutates every Go file under the
directory, including the copies `go-licenses` leaves in the ignored
`third_party_licenses/`.

Baseline, measured on 2026-11-19 with gremlins v0.6.0 over the whole module:
800 killed, 77 lived, 222 not covered, 4 timed out. Test efficacy, killed /
(killed + lived), is 91.22%; mutator coverage is 79.80%. Lived mutants include
equivalent ones, so the score is tracked against this baseline rather than held
to 100%.

## Benchmarks

[himorime](https://github.com/nao1215/himorime) measures the `gup` binary in
[`bench/`](https://github.com/nao1215/gup/tree/main/bench) and compares each
pull request with its base in the same job (`.github/workflows/bench.yml`).

```shell
make bench
```
