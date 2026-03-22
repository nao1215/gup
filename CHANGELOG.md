## [v1.1.4](https://github.com/nao1215/gup/compare/v1.1.3...v1.1.4) (2026-03-22)

### Build

* Bump minimum Go version from 1.24 to 1.25
* Add Go 1.26 to CI test matrix
* Bump `github.com/fatih/color` from 1.18.0 to 1.19.0

## [v1.1.3](https://github.com/nao1215/gup/compare/v1.1.2...v1.1.3) (2026-02-23)

### Fixes

* Harden browser launch behavior in `bug-report` with command timeout/wait handling
* Make interactive confirmation flow iterative and robust against repeated invalid input
* Strengthen `remove` target validation and normalization (including Windows-specific suffix handling)
* Improve `man` generation success logging and config swap error reporting in failure recovery paths
* Fix Windows `remove` regression test expectation and resolve `-race` failure in `bug-report` fallback output test

### Build

* Bump `goreleaser/goreleaser-action` from v6 to v7

### Docs

* Highlight installation availability via winget, mise, and nix

### Tests

* Add and expand regression tests for browser launcher behavior, remove target edge cases, and fallback output handling
* Keep overall statement coverage from dropping while adding the new hardening changes

## [v1.1.2](https://github.com/nao1215/gup/compare/v1.1.1...v1.1.2) (2026-02-20)

### Fixes

* Treat equal custom Go toolchain versions (e.g. `go1.26.0-X:nodwarf5`) as up-to-date in `check` and `update`
* Normalize custom Go version separators for comparison and apply Go-aware comparison to version colorization

### Tests

* Add regression tests for custom Go toolchain tags across `internal/goutil`, `check`, and `update`, including output color behavior

## [v1.1.1](https://github.com/nao1215/gup/compare/v1.1.0...v1.1.1) (2026-02-16)

### Fixes

* Make config writes atomic and harden replacement flow to avoid data loss on failed updates
* Fix Windows rename/update edge cases when `GOEXE` is unset
* Fix Windows target matching for `update` with case-insensitive name handling
* Fix `remove` behavior on Windows when `GOEXE` is empty and handle `.exe` suffix checks robustly
* Propagate cancellation to running `go install` / `go list` subprocesses
* Unify signal-based cancellation behavior across update/import/check flows

### Performance

* Speed up completion generation script by building once and reusing the binary

### Refactoring

* Centralize Go command availability checks and jobs clamping
* Simplify update internals toward context-aware operation paths

### Docs

* Consolidate contributor guidance and align README/CONTRIBUTING workflow instructions

### Tests

* Add and expand regression tests for atomic config writes, completion comparison, Windows suffix handling, and cancellation behavior

## [v1.1.0](https://github.com/nao1215/gup/compare/v1.0.0...v1.1.0) (2026-02-16)

### Features

* Add PowerShell completion generation via `gup completion powershell`
* Generate `completions/gup.ps1` in `scripts/completions.sh`

### Docs

* Clarify that `completion --install` targets bash/fish/zsh only
* Add PowerShell completion usage examples to README (en/es/fr/ja/ko/ru/zh-cn)

### Tests

* Strengthen completion output tests by capturing `os.Stdout` and verifying PowerShell header output

## [v1.0.0](https://github.com/nao1215/gup/compare/v0.28.3...v1.0.0) (2026-02-15)

### ⚠ BREAKING CHANGES

* Config format changed from plain-text `gup.conf` (`<name> = <import-path>`) to JSON `gup.json` with versioned schema
* `gup import` now installs the exact version recorded in `gup.json`
* `gup import` flag changed from `--input` to `--file`
* `gup export` flag changed from `--output` to `--file`

### Features

* Store update channels (`latest` / `main` / `master`) in `gup.json`
* Auto-adapt to module path changes (detect and follow import path renames)
* Add config path resolution (`$XDG_CONFIG_HOME/gup/gup.json` first, then `./gup.json`)
* Add `--file` option to both import and export
* Always include version in bug-report template

### Fixes

* Persist dry-run temp path for proper cleanup and cancel workers on interrupt signals
* Validate binary names in `removeOldBinaryIfRenamed` to prevent path traversal
* Block path traversal in remove targets
* Warn instead of failing when `gup.json` is corrupt during update
* Surface `gup.json` parse errors during update
* Normalize binary names in `--main`/`--master`/`--latest` flag resolution for Windows
* Remove stale config entries when binary is renamed during update
* Return original importPath when prefix does not match in `replaceImportPathPrefix`
* Trim whitespace in `--exclude` package names
* Require explicit `--install` for completion file writes
* Validate go command availability in import
* Exit with status 1 when command execution fails
* Version coloring: yellow=outdated, green=up-to-date
* Reject malformed `gup.conf` lines early (legacy format migration)
* Normalize devel version to latest during import

### Performance

* Use a fixed worker-pool implementation for package processing
* Deduplicate `GetLatestVer` calls and parallelize binary info collection
* Filter binary completions by typed prefix

### Refactoring

* Replace `golang.org/x/exp/slices` with standard `slices` package
* Share config file writing logic across commands
* Remove unused update wrapper function and unused first argument from `shouldPersistChannels`

### Tests

* Add tests to increase coverage from 79.6% to 88.7%
* Stub update operations in root command tests

## [v0.28.3](https://github.com/nao1215/gup/compare/v0.28.2...v0.28.3) (2026-02-15)

* Fix bug fixes and CI improvements [#230](https://github.com/nao1215/gup/pull/230) ([nao1215](https://github.com/nao1215))
* Fix address review findings across codebase [#229](https://github.com/nao1215/gup/pull/229) ([nao1215](https://github.com/nao1215))
* Fix issue #206 [#228](https://github.com/nao1215/gup/pull/228) ([nao1215](https://github.com/nao1215))
* Remove gorky package [#227](https://github.com/nao1215/gup/pull/227) ([nao1215](https://github.com/nao1215))
* Bump pointer v1.4.0 [#226](https://github.com/nao1215/gup/pull/226) ([nao1215](https://github.com/nao1215))

## [v0.28.2](https://github.com/nao1215/gup/compare/v0.28.1...v0.28.2) (2025-12-23)

* docs: add mise alternate installation instructions (en/fr) and fix shell quoting in README [#222](https://github.com/nao1215/gup/pull/222) ([jylenhof](https://github.com/jylenhof))
* Refactor bug report URL construction to fix the bug-report command [#221](https://github.com/nao1215/gup/pull/221) ([shogo82148](https://github.com/shogo82148))
* docs: add shogo82148 to all-contributors [#220](https://github.com/nao1215/gup/pull/220) ([nao1215](https://github.com/nao1215))
* docs: fix typo in README [#219](https://github.com/nao1215/gup/pull/219) ([shogo82148](https://github.com/shogo82148))
* Bump golang.org/x/sync from 0.18.0 to 0.19.0 [#217](https://github.com/nao1215/gup/pull/217) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/hashicorp/go-version from 1.7.0 to 1.8.0 [#214](https://github.com/nao1215/gup/pull/214) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/checkout from 5 to 6 [#213](https://github.com/nao1215/gup/pull/213) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.28.1](https://github.com/nao1215/gup/compare/v0.28.0...v0.28.1) (2025-11-16)

* Use MANPATH when installing man pages and fix lints [#211](https://github.com/nao1215/gup/pull/211) ([nao1215](https://github.com/nao1215))
* docs: update contributors [#210](https://github.com/nao1215/gup/pull/210) ([nao1215](https://github.com/nao1215))
* Return the original error when latest version lookup fails [#205](https://github.com/nao1215/gup/pull/205) ([peczenyj](https://github.com/peczenyj))
* Bump golang.org/x/sync from 0.17.0 to 0.18.0 [#207](https://github.com/nao1215/gup/pull/207) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.28.0](https://github.com/nao1215/gup/compare/v0.27.9...v0.28.0) (2025-10-27)

* Add --ignore-go-update flag and refine updater error handling [#201](https://github.com/nao1215/gup/pull/201) ([iTrooz](https://github.com/iTrooz))
* Refactor tests to isolate environments and add helpers [#203](https://github.com/nao1215/gup/pull/203) ([iTrooz](https://github.com/iTrooz))
* Bump golang.org/x/sync from 0.16.0 to 0.17.0 [#200](https://github.com/nao1215/gup/pull/200) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/setup-go from 5 to 6 [#199](https://github.com/nao1215/gup/pull/199) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.9](https://github.com/nao1215/gup/compare/v0.27.8...v0.27.9) (2025-09-04)

* docs: add README translations (es, fr, ko, ru, zh-cn) [#195](https://github.com/nao1215/gup/pull/195) ([nao1215](https://github.com/nao1215))
* Trim potential GOEXPERIMENT flag in build info [#197](https://github.com/nao1215/gup/pull/197) ([mcha-forks](https://github.com/mcha-forks))
* Update contributors [#198](https://github.com/nao1215/gup/pull/198) ([nao1215](https://github.com/nao1215))
* Support Go 1.24 or later [#188](https://github.com/nao1215/gup/pull/188) ([nao1215](https://github.com/nao1215))
* Bump github.com/spf13/cobra from 1.9.1 to 1.10.1 [#196](https://github.com/nao1215/gup/pull/196) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/checkout from 4 to 5 [#194](https://github.com/nao1215/gup/pull/194) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/gen2brain/beeep [#192](https://github.com/nao1215/gup/pull/192) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.15.0 to 0.16.0 [#193](https://github.com/nao1215/gup/pull/193) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.14.0 to 0.15.0 [#190](https://github.com/nao1215/gup/pull/190) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.13.0 to 0.14.0 [#189](https://github.com/nao1215/gup/pull/189) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.8](https://github.com/nao1215/gup/compare/v0.27.7...v0.27.8) (2025-03-12)

* Bump golang.org/x/sync from 0.11.0 to 0.12.0 [#185](https://github.com/nao1215/gup/pull/185) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.7](https://github.com/nao1215/gup/compare/v0.27.6...v0.27.7) (2025-02-25)

* Add Go 1.24 to CI and fix unit tests [#182](https://github.com/nao1215/gup/pull/182) ([nao1215](https://github.com/nao1215))
* Bump github.com/google/go-cmp from 0.6.0 to 0.7.0 [#184](https://github.com/nao1215/gup/pull/184) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/spf13/cobra from 1.8.1 to 1.9.1 [#183](https://github.com/nao1215/gup/pull/183) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.10.0 to 0.11.0 [#181](https://github.com/nao1215/gup/pull/181) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.6](https://github.com/nao1215/gup/compare/v0.27.5...v0.27.6) (2025-01-13)

* Bump github.com/mattn/go-colorable from 0.1.13 to 0.1.14 [#180](https://github.com/nao1215/gup/pull/180) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.9.0 to 0.10.0 [#179](https://github.com/nao1215/gup/pull/179) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.8.0 to 0.9.0 [#178](https://github.com/nao1215/gup/pull/178) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/adrg/xdg from 0.5.2 to 0.5.3 [#177](https://github.com/nao1215/gup/pull/177) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/adrg/xdg from 0.5.1 to 0.5.2 [#176](https://github.com/nao1215/gup/pull/176) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.17.0 to 1.18.0 [#175](https://github.com/nao1215/gup/pull/175) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/adrg/xdg from 0.5.0 to 0.5.1 [#174](https://github.com/nao1215/gup/pull/174) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.5](https://github.com/nao1215/gup/compare/v0.27.4...v0.27.5) (2024-09-10)

* Fix: check sub command prints incorrect path [#172](https://github.com/nao1215/gup/pull/172) ([nao1215](https://github.com/nao1215))
* Add go 1.23 [#170](https://github.com/nao1215/gup/pull/170) ([nao1215](https://github.com/nao1215))

## [v0.27.4](https://github.com/nao1215/gup/compare/v0.27.3...v0.27.4) (2024-08-10)

* Feat: Integrate completions into Homebrew formula (Issue #168) [#169](https://github.com/nao1215/gup/pull/169) ([nao1215](https://github.com/nao1215))
* Bump golang.org/x/sync from 0.7.0 to 0.8.0 [#167](https://github.com/nao1215/gup/pull/167) ([dependabot[bot]](https://github.com/apps/dependabot))
* Specify Language for Fenced Code Blocks [#166](https://github.com/nao1215/gup/pull/166) ([nao1215](https://github.com/nao1215))
* Bump github.com/adrg/xdg from 0.4.0 to 0.5.0 [#165](https://github.com/nao1215/gup/pull/165) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.27.3](https://github.com/nao1215/gup/compare/v0.27.2...v0.27.3) (2024-06-27)

* Output current rather than latest version in up-to-date messages [#164](https://github.com/nao1215/gup/pull/164) ([scop](https://github.com/scop))

## [v0.27.2](https://github.com/nao1215/gup/compare/v0.27.1...v0.27.2) (2024-06-24)

* Update: change version compare logic [#162](https://github.com/nao1215/gup/pull/162) ([nao1215](https://github.com/nao1215))
* Update README output wrt added Go versions [#161](https://github.com/nao1215/gup/pull/161) ([scop](https://github.com/scop))

## [v0.27.1](https://github.com/nao1215/gup/compare/v0.27.0...v0.27.1) (2024-06-19)

* Remove deprecated option [#158](https://github.com/nao1215/gup/pull/158) ([nao1215](https://github.com/nao1215))

## [v0.27.0](https://github.com/nao1215/gup/compare/v0.26.2...v0.27.0) (2024-06-19)

* Output and consider Go toolchain version, too [#156](https://github.com/nao1215/gup/pull/156) ([scop](https://github.com/scop))
* Bump github.com/spf13/cobra from 1.8.0 to 1.8.1 [#155](https://github.com/nao1215/gup/pull/155) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump goreleaser/goreleaser-action from 5 to 6 [#153](https://github.com/nao1215/gup/pull/153) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.26.2](https://github.com/nao1215/gup/compare/v0.26.1...v0.26.2) (2024-05-22)

* Fix goreleaser quoting [#151](https://github.com/nao1215/gup/pull/151) ([nao1215](https://github.com/nao1215))
* Update GitHub Actions [#150](https://github.com/nao1215/gup/pull/150) ([nao1215](https://github.com/nao1215))

## [v0.26.1](https://github.com/nao1215/gup/compare/v0.26.0...v0.26.1) (2024-05-15)

* Update project rules [#149](https://github.com/nao1215/gup/pull/149) ([nao1215](https://github.com/nao1215))
* Bump github.com/fatih/color from 1.16.0 to 1.17.0 [#148](https://github.com/nao1215/gup/pull/148) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.26.0](https://github.com/nao1215/gup/compare/v0.25.2...v0.26.0) (2024-05-08)

* Fix issue148: Confirmation with [Y/n] should default to "yes" [#147](https://github.com/nao1215/gup/pull/147) ([nao1215](https://github.com/nao1215))

## [v0.25.2](https://github.com/nao1215/gup/compare/v0.25.1...v0.25.2) (2024-05-01)

* Auto generate homebrew tap [#145](https://github.com/nao1215/gup/pull/145) ([nao1215](https://github.com/nao1215))

## [v0.25.1](https://github.com/nao1215/gup/compare/v0.25.0...v0.25.1) (2024-04-11)

* Argument validation and completion improvements [#144](https://github.com/nao1215/gup/pull/144) ([scop](https://github.com/scop))
* docs: add scop as a contributor for code [#143](https://github.com/nao1215/gup/pull/143) ([allcontributors[bot]](https://github.com/apps/allcontributors))

## [v0.25.0](https://github.com/nao1215/gup/compare/v0.24.3...v0.25.0) (2024-04-09)

* Generate cobra's v2 bash completions [#141](https://github.com/nao1215/gup/pull/141) ([scop](https://github.com/scop))
* Various completion improvements [#140](https://github.com/nao1215/gup/pull/140) ([scop](https://github.com/scop))
* Update bash completion path [#139](https://github.com/nao1215/gup/pull/139) ([scop](https://github.com/scop))
* Add ability to output completions to stdout [#138](https://github.com/nao1215/gup/pull/138) ([scop](https://github.com/scop))
* Bump golang.org/x/sync from 0.6.0 to 0.7.0 [#137](https://github.com/nao1215/gup/pull/137) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.24.3](https://github.com/nao1215/gup/compare/v0.24.2...v0.24.3) (2024-03-24)

* Polishing [#136](https://github.com/nao1215/gup/pull/136) ([rkscv](https://github.com/rkscv))

## [v0.24.2](https://github.com/nao1215/gup/compare/v0.24.1...v0.24.2) (2024-03-23)

* Update README and unit test [#135](https://github.com/nao1215/gup/pull/135) ([nao1215](https://github.com/nao1215))
* Get `GOPATH` with `go env` [#133](https://github.com/nao1215/gup/pull/133) ([rkscv](https://github.com/rkscv))
* Use buildinfo [#132](https://github.com/nao1215/gup/pull/132) ([rkscv](https://github.com/rkscv))
* Drop go 1.18, support go 1.19 to 1.22 [#134](https://github.com/nao1215/gup/pull/134) ([nao1215](https://github.com/nao1215))
* Bump k1LoW/octocov-action from 0 to 1 [#131](https://github.com/nao1215/gup/pull/131) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.5.0 to 0.6.0 [#130](https://github.com/nao1215/gup/pull/130) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add all contributors [#129](https://github.com/nao1215/gup/pull/129) ([nao1215](https://github.com/nao1215))
* docs: add nao1215 as a contributor for code [#128](https://github.com/nao1215/gup/pull/128) ([allcontributors[bot]](https://github.com/apps/allcontributors))
* Introduce all contributors section [#127](https://github.com/nao1215/gup/pull/127) ([nao1215](https://github.com/nao1215))
* Bump actions/setup-go from 4 to 5 [#126](https://github.com/nao1215/gup/pull/126) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.24.1](https://github.com/nao1215/gup/compare/v0.24.0...v0.24.1) (2023-12-05)

* Add remove subcommand alias: rm [#125](https://github.com/nao1215/gup/pull/125) ([nao1215](https://github.com/nao1215))
* introduce hottest action [#124](https://github.com/nao1215/gup/pull/124) ([nao1215](https://github.com/nao1215))
* Good bye BSD family [#123](https://github.com/nao1215/gup/pull/123) ([nao1215](https://github.com/nao1215))

## [v0.24.0](https://github.com/nao1215/gup/compare/v0.23.0...v0.24.0) (2023-10-17)

* Downgrade BSD test and add unit test [#114](https://github.com/nao1215/gup/pull/114) ([nao1215](https://github.com/nao1215))
* chore: improve windows ux [#113](https://github.com/nao1215/gup/pull/113) ([lincolnthalles](https://github.com/lincolnthalles))
* (auto merged) Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 [#112](https://github.com/nao1215/gup/pull/112) ([dependabot[bot]](https://github.com/apps/dependabot))
* (auto merged) Bump cross-platform-actions/action from 0.19.0 to 0.19.1 [#111](https://github.com/nao1215/gup/pull/111) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/sync from 0.3.0 to 0.4.0 [#109](https://github.com/nao1215/gup/pull/109) ([dependabot[bot]](https://github.com/apps/dependabot))
* Update auto-merged.yml [#110](https://github.com/nao1215/gup/pull/110) ([nao1215](https://github.com/nao1215))
* auto merged github actions for pr that created dependabot [#108](https://github.com/nao1215/gup/pull/108) ([nao1215](https://github.com/nao1215))
* Bump cross-platform-actions/action from 0.15.0 to 0.19.0 [#107](https://github.com/nao1215/gup/pull/107) ([dependabot[bot]](https://github.com/apps/dependabot))
* NetBSD unit test [#106](https://github.com/nao1215/gup/pull/106) ([nao1215](https://github.com/nao1215))
* Change codecov to octocov [#105](https://github.com/nao1215/gup/pull/105) ([nao1215](https://github.com/nao1215))
* Add github actions for dragonfly [#103](https://github.com/nao1215/gup/pull/103) ([nao1215](https://github.com/nao1215))
* Bump actions/checkout from 2 to 4 [#99](https://github.com/nao1215/gup/pull/99) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/setup-go from 3 to 4 [#98](https://github.com/nao1215/gup/pull/98) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump codecov/codecov-action from 1 to 4 [#101](https://github.com/nao1215/gup/pull/101) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump cross-platform-actions/action from 0.15.0 to 0.19.0 [#100](https://github.com/nao1215/gup/pull/100) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.23.0](https://github.com/nao1215/gup/compare/v0.22.0...v0.23.0) (2023-09-15)

* Bump goreleaser/goreleaser-action from 2 to 5 [#102](https://github.com/nao1215/gup/pull/102) ([dependabot[bot]](https://github.com/apps/dependabot))
* Support FreeBSD, OpenBSD [#97](https://github.com/nao1215/gup/pull/97) ([nao1215](https://github.com/nao1215))
* Create FUNDING.yml [#96](https://github.com/nao1215/gup/pull/96) ([nao1215](https://github.com/nao1215))
* Update README.md [#95](https://github.com/nao1215/gup/pull/95) ([nao1215](https://github.com/nao1215))
* Bump golang.org/x/sync from 0.2.0 to 0.3.0 [#94](https://github.com/nao1215/gup/pull/94) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add git-leak [#93](https://github.com/nao1215/gup/pull/93) ([nao1215](https://github.com/nao1215))

## [v0.22.0](https://github.com/nao1215/gup/compare/v0.21.1...v0.22.0) (2023-05-27)

* Add --main option and --main-all option [#91](https://github.com/nao1215/gup/pull/91) ([nao1215](https://github.com/nao1215))

## [v0.21.1](https://github.com/nao1215/gup/compare/v0.21.0...v0.21.1) (2023-05-05)

* Bump golang.org/x/sync from 0.1.0 to 0.2.0 [#90](https://github.com/nao1215/gup/pull/90) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add default openBrowser function [#89](https://github.com/nao1215/gup/pull/89) ([memreflect](https://github.com/memreflect))

## [v0.21.0](https://github.com/nao1215/gup/compare/v0.20.1...v0.21.0) (2023-05-02)

* Refactor exclude option [#87](https://github.com/nao1215/gup/pull/87) ([nao1215](https://github.com/nao1215))
* * added option to exclude binaries from gup update [#86](https://github.com/nao1215/gup/pull/86) ([hueuebi](https://github.com/hueuebi))
* Bump github.com/spf13/cobra from 1.6.1 to 1.7.0 [#84](https://github.com/nao1215/gup/pull/84) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.20.1](https://github.com/nao1215/gup/compare/v0.20.0...v0.20.1) (2023-03-21)

* Delete internal file package. use gorky/file pacakage instead [#83](https://github.com/nao1215/gup/pull/83) ([nao1215](https://github.com/nao1215))
* Ignore hidden file in $GOBIN [#82](https://github.com/nao1215/gup/pull/82) ([nao1215](https://github.com/nao1215))

## [v0.20.0](https://github.com/nao1215/gup/compare/v0.19.0...v0.20.0) (2023-03-14)

* Bump github.com/fatih/color from 1.14.1 to 1.15.0 [#79](https://github.com/nao1215/gup/pull/79) ([dependabot[bot]](https://github.com/apps/dependabot))
* add bug-report command [#80](https://github.com/nao1215/gup/pull/80) ([nao1215](https://github.com/nao1215))
* Fix gosec and update go version to 1.20 [#78](https://github.com/nao1215/gup/pull/78) ([nao1215](https://github.com/nao1215))

## [v0.19.0](https://github.com/nao1215/gup/compare/v0.18.0...v0.19.0) (2023-03-05)

* Fix Issue #76: Limit the number of goroutines [#77](https://github.com/nao1215/gup/pull/77) ([nao1215](https://github.com/nao1215))

## [v0.18.0](https://github.com/nao1215/gup/compare/v0.17.1...v0.18.0) (2023-02-26)

* Delete update-go subcommand [#75](https://github.com/nao1215/gup/pull/75) ([nao1215](https://github.com/nao1215))
* fix: Use canonical name for bash_completion.d [#74](https://github.com/nao1215/gup/pull/74) ([jlec](https://github.com/jlec))

## [v0.17.1](https://github.com/nao1215/gup/compare/v0.17.0...v0.17.1) (2023-02-23)

* Issue #72: Change bash completion file path [#73](https://github.com/nao1215/gup/pull/73) ([nao1215](https://github.com/nao1215))
* Add download progress bar [#70](https://github.com/nao1215/gup/pull/70) ([nao1215](https://github.com/nao1215))

## [v0.17.0](https://github.com/nao1215/gup/compare/v0.16.0...v0.17.0) (2023-02-20)

* Add update-go subcommand [#69](https://github.com/nao1215/gup/pull/69) ([nao1215](https://github.com/nao1215))

## [v0.16.0](https://github.com/nao1215/gup/compare/v0.15.1...v0.16.0) (2023-02-13)

* Support XDG_CONFIG_HOME for configuration files path [#68](https://github.com/nao1215/gup/pull/68) ([nao1215](https://github.com/nao1215))
* Bump github.com/fatih/color from 1.13.0 to 1.14.1 [#65](https://github.com/nao1215/gup/pull/65) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/spf13/cobra from 1.6.0 to 1.6.1 [#63](https://github.com/nao1215/gup/pull/63) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.15.1](https://github.com/nao1215/gup/compare/v0.15.0...v0.15.1) (2022-10-22)

* Update dbus version [#61](https://github.com/nao1215/gup/pull/61) ([nao1215](https://github.com/nao1215))
* Add description for --notify option at README [#58](https://github.com/nao1215/gup/pull/58) ([nao1215](https://github.com/nao1215))

## [v0.15.0](https://github.com/nao1215/gup/compare/v0.14.0...v0.15.0) (2022-10-18)

* Bump github.com/spf13/cobra from 1.5.0 to 1.6.0 [#55](https://github.com/nao1215/gup/pull/55) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add an option to enable notifications (-N, --notify) and disable them by default. [#57](https://github.com/nao1215/gup/pull/57) ([nao1215](https://github.com/nao1215))
* Add awesome go badge [#54](https://github.com/nao1215/gup/pull/54) ([nao1215](https://github.com/nao1215))

## [v0.14.0](https://github.com/nao1215/gup/compare/v0.13.0...v0.14.0) (2022-10-02)

* Add completion subcommand [#53](https://github.com/nao1215/gup/pull/53) ([nao1215](https://github.com/nao1215))

## [v0.13.0](https://github.com/nao1215/gup/compare/v0.12.0...v0.13.0) (2022-09-24)

* Detailed error message when go install fails. [#52](https://github.com/nao1215/gup/pull/52) ([nao1215](https://github.com/nao1215))
* Add --input option to import subcommand [#51](https://github.com/nao1215/gup/pull/51) ([nao1215](https://github.com/nao1215))

## [v0.12.0](https://github.com/nao1215/gup/compare/v0.11.0...v0.12.0) (2022-09-24)

* Added output option to Export subcommand [#50](https://github.com/nao1215/gup/pull/50) ([nao1215](https://github.com/nao1215))
* Add test for gotuil.GetPackageVersion() function [#49](https://github.com/nao1215/gup/pull/49) ([KEINOS](https://github.com/KEINOS))

## [v0.11.0](https://github.com/nao1215/gup/compare/v0.10.5...v0.11.0) (2022-09-19)

* Update contributors [#48](https://github.com/nao1215/gup/pull/48) ([nao1215](https://github.com/nao1215))
* Add unit test for goutil package [#41](https://github.com/nao1215/gup/pull/41) ([KEINOS](https://github.com/KEINOS))
* update unit test status badge [#46](https://github.com/nao1215/gup/pull/46) ([nao1215](https://github.com/nao1215))
* Add platform test workflow [#45](https://github.com/nao1215/gup/pull/45) ([nao1215](https://github.com/nao1215))
* Fixed unit-test that produced different test results on mac and linux [#43](https://github.com/nao1215/gup/pull/43) ([nao1215](https://github.com/nao1215))
* Add unit test for all subcommand [#42](https://github.com/nao1215/gup/pull/42) ([nao1215](https://github.com/nao1215))
* Add unit test for check subcommand [#40](https://github.com/nao1215/gup/pull/40) ([nao1215](https://github.com/nao1215))
* Add unit test [#38](https://github.com/nao1215/gup/pull/38) ([nao1215](https://github.com/nao1215))
* fix: avoid fall back if current is newer than latest version (issue #36) [#39](https://github.com/nao1215/gup/pull/39) ([KEINOS](https://github.com/KEINOS))
* Add coverage workflow [#37](https://github.com/nao1215/gup/pull/37) ([nao1215](https://github.com/nao1215))
* Bump github.com/mattn/go-colorable from 0.1.12 to 0.1.13 [#35](https://github.com/nao1215/gup/pull/35) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.10.5](https://github.com/nao1215/gup/compare/v0.10.4...v0.10.5) (2022-08-11)

* FIx remove subcomand for windows [#34](https://github.com/nao1215/gup/pull/34) ([nao1215](https://github.com/nao1215))

## [v0.10.4](https://github.com/nao1215/gup/compare/v0.10.3...v0.10.4) (2022-08-08)

* Changed to get version information from ldflags [#33](https://github.com/nao1215/gup/pull/33) ([nao1215](https://github.com/nao1215))
* Update readme and more [#32](https://github.com/nao1215/gup/pull/32) ([nao1215](https://github.com/nao1215))
* Bump github.com/spf13/cobra from 1.4.0 to 1.5.0 [#31](https://github.com/nao1215/gup/pull/31) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.10.3](https://github.com/nao1215/gup/compare/v0.10.2...v0.10.3) (2022-05-03)


## [v0.10.2](https://github.com/nao1215/gup/compare/v0.10.1...v0.10.2) (2022-05-03)


## [v0.10.1](https://github.com/nao1215/gup/compare/v0.10.0...v0.10.1) (2022-04-17)


## [v0.10.0](https://github.com/nao1215/gup/compare/v0.9.4...v0.10.0) (2022-04-17)

* Auto-generate shell completion file [#26](https://github.com/nao1215/gup/pull/26) ([nao1215](https://github.com/nao1215))

## [v0.9.4](https://github.com/nao1215/gup/compare/v0.9.3...v0.9.4) (2022-04-16)


## [v0.9.3](https://github.com/nao1215/gup/compare/v0.9.2...v0.9.3) (2022-04-16)

* Parallelized check subcommand process [#25](https://github.com/nao1215/gup/pull/25) ([nao1215](https://github.com/nao1215))
* Improved error messages [#24](https://github.com/nao1215/gup/pull/24) ([nao1215](https://github.com/nao1215))
* Faster update speeds due to parallel processing [#23](https://github.com/nao1215/gup/pull/23) ([nao1215](https://github.com/nao1215))

## [v0.9.2](https://github.com/nao1215/gup/compare/v0.9.1...v0.9.2) (2022-03-20)


## [v0.9.1](https://github.com/nao1215/gup/compare/v0.9.0...v0.9.1) (2022-03-19)


## [v0.9.0](https://github.com/nao1215/gup/compare/v0.8.0...v0.9.0) (2022-03-18)

* Added desktop notification [#22](https://github.com/nao1215/gup/pull/22) ([nao1215](https://github.com/nao1215))
* Add check subcommand. [#21](https://github.com/nao1215/gup/pull/21) ([nao1215](https://github.com/nao1215))

## [v0.8.0](https://github.com/nao1215/gup/compare/v0.7.4...v0.8.0) (2022-03-18)


## [v0.7.4](https://github.com/nao1215/gup/compare/v0.7.3...v0.7.4) (2022-03-13)


## [v0.7.3](https://github.com/nao1215/gup/compare/v0.7.2...v0.7.3) (2022-03-11)

* Bump github.com/spf13/cobra from 1.3.0 to 1.4.0 [#20](https://github.com/nao1215/gup/pull/20) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.7.2](https://github.com/nao1215/gup/compare/v0.7.1...v0.7.2) (2022-03-06)

* Add version info [#19](https://github.com/nao1215/gup/pull/19) ([nao1215](https://github.com/nao1215))
* Add version info [#18](https://github.com/nao1215/gup/pull/18) ([nao1215](https://github.com/nao1215))

## [v0.7.1](https://github.com/nao1215/gup/compare/v0.7.0...v0.7.1) (2022-03-05)


## [v0.7.0](https://github.com/nao1215/gup/compare/v0.6.1...v0.7.0) (2022-03-04)

* Improve from "Suggestions for improvements #2" [#17](https://github.com/nao1215/gup/pull/17) ([nao1215](https://github.com/nao1215))

## [v0.6.1](https://github.com/nao1215/gup/compare/v0.6.0...v0.6.1) (2022-02-26)


## [v0.6.0](https://github.com/nao1215/gup/compare/v0.5.0...v0.6.0) (2022-02-26)

* Add review dog [#15](https://github.com/nao1215/gup/pull/15) ([nao1215](https://github.com/nao1215))

## [v0.5.0](https://github.com/nao1215/gup/compare/v0.4.4...v0.5.0) (2022-02-22)

* Add list subcommand: List up command name with package path and version under $GOPATH/bin or $GOBIN [#13](https://github.com/nao1215/gup/pull/13) ([nao1215](https://github.com/nao1215))

## [v0.4.4](https://github.com/nao1215/gup/compare/v0.4.3...v0.4.4) (2022-02-22)

* Use strings.HasPrefix instead of regular expression [#11](https://github.com/nao1215/gup/pull/11) ([matsuyoshi30](https://github.com/matsuyoshi30))
* --file option: Specified update target [#12](https://github.com/nao1215/gup/pull/12) ([nao1215](https://github.com/nao1215))

## [v0.4.3](https://github.com/nao1215/gup/compare/v0.4.2...v0.4.3) (2022-02-22)


## [v0.4.2](https://github.com/nao1215/gup/compare/v0.4.0...v0.4.2) (2022-02-22)


## [v0.4.0](https://github.com/nao1215/gup/compare/v0.2.1...v0.4.0) (2022-02-22)

* Revert "Use buildinfo" because  debug/buildinfo is not released [#9](https://github.com/nao1215/gup/pull/9) ([nao1215](https://github.com/nao1215))
* Use buildinfo [#6](https://github.com/nao1215/gup/pull/6) ([mattn](https://github.com/mattn))
* Add export subcommand [#7](https://github.com/nao1215/gup/pull/7) ([nao1215](https://github.com/nao1215))
* Add --dry-run option. [#5](https://github.com/nao1215/gup/pull/5) ([nao1215](https://github.com/nao1215))

## [v0.2.1](https://github.com/nao1215/gup/compare/v0.2.0...v0.2.1) (2022-02-22)

* Improve help message [#3](https://github.com/nao1215/gup/pull/3) ([nao1215](https://github.com/nao1215))

## [v0.2.0](https://github.com/nao1215/gup/compare/v0.1.1...v0.2.0) (2022-02-21)

* Add import command and Change logic [#1](https://github.com/nao1215/gup/pull/1) ([nao1215](https://github.com/nao1215))

## [v0.1.1](https://github.com/nao1215/gup/compare/v0.1.0...v0.1.1) (2022-02-21)


## [v0.1.0](https://github.com/nao1215/gup/compare/17a4faec4b36...v0.1.0) (2022-02-20)
