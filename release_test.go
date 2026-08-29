package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// workflowDir holds the GitHub Actions workflows the tests below inspect.
const workflowDir = ".github/workflows"

// These tests guard the release pipeline configuration so the supply-chain and
// release-notes guarantees promised in the README and issues #283/#285 cannot
// silently regress. They parse the committed YAML rather than running the
// release, which keeps them fast and offline.

func readYAMLFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // path is a fixed in-repo config file
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s is not valid YAML: %v", path, err)
	}
	return doc
}

// Test_goreleaser_isVersion2 guards the GoReleaser v2 schema declaration so the
// config keeps parsing under the v2 toolchain used in CI.
func Test_goreleaser_isVersion2(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".goreleaser.yml")
	if doc["version"] != 2 {
		t.Errorf("`.goreleaser.yml` must declare `version: 2`, got %v", doc["version"])
	}
}

// Test_goreleaser_curatedChangelog verifies issue #283: release notes are
// grouped by user-facing categories instead of a raw commit dump.
func Test_goreleaser_curatedChangelog(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".goreleaser.yml")

	changelog, ok := doc["changelog"].(map[string]any)
	if !ok {
		t.Fatal("changelog section is missing in .goreleaser.yml")
	}
	groupsRaw, ok := changelog["groups"].([]any)
	if !ok || len(groupsRaw) == 0 {
		t.Fatal("changelog.groups is missing; release notes would be a raw commit dump")
	}

	titles := make(map[string]bool)
	for _, g := range groupsRaw {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if title, ok := group["title"].(string); ok {
			titles[strings.ToLower(title)] = true
		}
	}

	for _, want := range []string{
		"breaking changes",
		"features",
		"bug fixes",
		"performance",
		"documentation",
		"others",
	} {
		if !hasTitleContaining(titles, want) {
			t.Errorf("changelog.groups is missing a %q category; got titles %v", want, keys(titles))
		}
	}
}

// Test_goreleaser_supplyChain verifies issue #285: SBOM generation and artifact
// signing are configured.
func Test_goreleaser_supplyChain(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".goreleaser.yml")

	if _, ok := doc["sboms"]; !ok {
		t.Error("sboms section is missing in .goreleaser.yml (no SBOM published)")
	}

	signsRaw, ok := doc["signs"].([]any)
	if !ok || len(signsRaw) == 0 {
		t.Fatal("signs section is missing in .goreleaser.yml (artifacts are not signed)")
	}
	usesCosign := false
	for _, s := range signsRaw {
		sign, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := sign["cmd"].(string); ok && strings.Contains(cmd, "cosign") {
			usesCosign = true
		}
	}
	if !usesCosign {
		t.Error("signs section does not use cosign")
	}
}

// Test_releaseWorkflow_provenanceAndSigning verifies issue #285 at the workflow
// level: keyless signing and provenance attestation require id-token permission,
// the cosign installer, and an attestation step.
func Test_releaseWorkflow_provenanceAndSigning(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".github/workflows/release.yml")

	perms, ok := doc["permissions"].(map[string]any)
	if !ok {
		t.Fatal("release workflow is missing a permissions block")
	}
	if perms["id-token"] != "write" {
		t.Errorf("release workflow needs 'id-token: write' for keyless signing/provenance, got %v", perms["id-token"])
	}
	if perms["attestations"] != "write" {
		t.Errorf("release workflow needs 'attestations: write' for provenance, got %v", perms["attestations"])
	}

	// Validate the structured jobs.release.steps[*].uses rather than raw text so
	// a substring elsewhere in the file cannot make this pass by accident.
	uses := releaseStepUses(t, doc)
	if !anyHasPrefix(uses, "sigstore/cosign-installer") {
		t.Errorf("release workflow does not install cosign; steps use: %v", uses)
	}
	if !anyHasPrefix(uses, "actions/attest-build-provenance") {
		t.Errorf("release workflow does not attest build provenance; steps use: %v", uses)
	}
}

// releaseStepUses returns every `uses:` value of the jobs.release.steps entries.
func releaseStepUses(t *testing.T, doc map[string]any) []string {
	t.Helper()
	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no jobs block")
	}
	release, ok := jobs["release"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no 'release' job")
	}
	steps, ok := release["steps"].([]any)
	if !ok {
		t.Fatal("release job has no steps")
	}
	uses := make([]string, 0, len(steps))
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if u, ok := step["uses"].(string); ok {
			uses = append(uses, u)
		}
	}
	return uses
}

func anyHasPrefix(values []string, prefix string) bool {
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func hasTitleContaining(titles map[string]bool, want string) bool {
	for title := range titles {
		if strings.Contains(title, want) {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Test_workflows_pinActionsToCommitSHA asserts that every third-party action in
// every workflow is pinned to a full 40-character commit SHA with the version it
// tracks in a trailing comment. A mutable tag (`@v3`) hands whoever can move that
// tag the ability to run arbitrary code in this repository's CI - including the
// release job, which holds the tokens that publish to Homebrew, winget, and the
// Scoop bucket. Most workflows were already pinned; website.yml was not, and
// nothing made that visible. This test is what keeps a newly added step from
// re-opening the hole.
func Test_workflows_pinActionsToCommitSHA(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", workflowDir, err)
	}

	// A full commit SHA followed by a "# vX.Y.Z" comment naming the version it
	// pins, so a reader can tell what the opaque hash actually is.
	pinned := regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}\s+#\s*\S`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		path := filepath.Join(workflowDir, entry.Name())
		raw, err := os.ReadFile(path) //nolint:gosec // path is an in-repo workflow file
		if err != nil {
			t.Errorf("failed to read %s: %v", path, err)
			continue
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
			if !strings.HasPrefix(trimmed, "uses:") {
				continue
			}
			ref := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
			// A local composite action (./.github/actions/...) has no upstream
			// owner to pin against; it is this repository's own content.
			if strings.HasPrefix(ref, "./") {
				continue
			}
			if !pinned.MatchString(ref) {
				t.Errorf("%s:%d pins an action by tag or branch, not by commit SHA with a version comment: %q",
					path, i+1, ref)
			}
		}
	}
}

// Test_workflows_haveGovulncheck asserts the vulnerability scan exists and stays
// wired to pull requests, main, and a schedule. The scheduled leg is the point:
// the advisory database changes without gup changing, so a scan that only ran on
// pull requests would report a clean repository right up until someone happened
// to open one.
func Test_workflows_haveGovulncheck(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, filepath.Join(workflowDir, "govulncheck.yml"))

	// yaml.v3 follows the YAML 1.2 core schema, so the `on:` key stays the string
	// "on" rather than being folded into the boolean true the way YAML 1.1 did.
	triggers, ok := doc["on"].(map[string]any)
	if !ok {
		t.Fatal("govulncheck workflow has no trigger block")
	}
	for _, want := range []string{"pull_request", "push", "schedule", "workflow_dispatch"} {
		if _, ok := triggers[want]; !ok {
			t.Errorf("govulncheck workflow is missing the %q trigger", want)
		}
	}

	perms, ok := doc["permissions"].(map[string]any)
	if !ok {
		t.Fatal("govulncheck workflow is missing a permissions block")
	}
	if perms["contents"] != "read" {
		t.Errorf("govulncheck workflow should run with 'contents: read', got %v", perms["contents"])
	}
	if len(perms) != 1 {
		t.Errorf("govulncheck workflow grants more than read access to the checkout: %v", perms)
	}
}

// Test_goreleaser_explicitArchitectures asserts the build matrix is stated, not
// inherited. GoReleaser's default goarch list includes 386, so leaving the key
// out published a 32-bit artifact gup never claimed to support, never smoke
// tested, and never documented. The OS and arch sets here are what README.md,
// website/content/install.md, and scripts/smoke_artifacts.sh are all written
// against.
func Test_goreleaser_explicitArchitectures(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".goreleaser.yml")

	builds, ok := doc["builds"].([]any)
	if !ok || len(builds) == 0 {
		t.Fatal("builds section is missing in .goreleaser.yml")
	}
	build, ok := builds[0].(map[string]any)
	if !ok {
		t.Fatal("builds[0] is not a mapping in .goreleaser.yml")
	}

	for key, want := range map[string][]string{
		"goos":   {"linux", "windows", "darwin"},
		"goarch": {"amd64", "arm64"},
	} {
		got := stringSlice(build[key])
		if len(got) == 0 {
			t.Errorf("builds[0].%s is not declared; GoReleaser would fall back to its defaults", key)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("builds[0].%s = %v, want exactly %v", key, got, want)
			continue
		}
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("builds[0].%s = %v, missing %q", key, got, w)
			}
		}
	}
}

// Test_goreleaser_scoopBucket asserts the Scoop manifest is published into this
// repository's own bucket/ directory. Pointing it at a separate repository would
// need a cross-repository token the release workflow does not have, so the
// release would fail at publish time - after the GitHub Release already exists.
func Test_goreleaser_scoopBucket(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, ".goreleaser.yml")

	scoops, ok := doc["scoops"].([]any)
	if !ok || len(scoops) == 0 {
		t.Fatal("scoops section is missing in .goreleaser.yml (no Scoop manifest is published)")
	}
	scoop, ok := scoops[0].(map[string]any)
	if !ok {
		t.Fatal("scoops[0] is not a mapping in .goreleaser.yml")
	}
	if scoop["directory"] != "bucket" {
		t.Errorf("scoops[0].directory = %v, want \"bucket\" (the in-repo Scoop bucket)", scoop["directory"])
	}
	repo, ok := scoop["repository"].(map[string]any)
	if !ok {
		t.Fatal("scoops[0].repository is missing in .goreleaser.yml")
	}
	if repo["owner"] != "nao1215" || repo["name"] != "gup" {
		t.Errorf("scoops[0].repository = %v/%v, want nao1215/gup so the built-in GITHUB_TOKEN can publish it",
			repo["owner"], repo["name"])
	}
	if _, ok := repo["token"]; ok {
		t.Error("scoops[0].repository declares a token; publishing into this same repository needs only the workflow's GITHUB_TOKEN")
	}
	// The bucket has to be a real directory in the repository, or `scoop bucket
	// add` clones something Scoop cannot read.
	if _, err := os.Stat("bucket/README.md"); err != nil {
		t.Errorf("bucket/README.md is missing: %v", err)
	}
}

// stringSlice converts a YAML sequence of scalars into []string.
func stringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Test_releaseWorkflow_gatesPublishOnArtifactSmoke asserts the publish job runs
// only after the artifact smoke tests pass, on all three operating systems. A
// broken artifact cannot be taken back once a tag is published, and the smoke
// job is only a gate if `needs` says so - a smoke job that merely runs in
// parallel with the release lets the release win the race.
func Test_releaseWorkflow_gatesPublishOnArtifactSmoke(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, filepath.Join(workflowDir, "release.yml"))

	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no jobs block")
	}
	release, ok := jobs["release"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no 'release' job")
	}

	needs := stringSlice(release["needs"])
	if single, ok := release["needs"].(string); ok {
		needs = []string{single}
	}
	for _, want := range []string{"smoke", "smoke-cross"} {
		if _, ok := jobs[want]; !ok {
			t.Errorf("release workflow has no %q job", want)
		}
		if !slices.Contains(needs, want) {
			t.Errorf("the release job does not depend on %q (needs = %v); publishing could proceed past a failed smoke test",
				want, needs)
		}
	}

	// The cross-OS leg is the point of splitting the job: running the Windows and
	// macOS binaries is the one thing the Ubuntu leg cannot do.
	crossJob, ok := jobs["smoke-cross"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no 'smoke-cross' job")
	}
	matrix := jobMatrixOS(t, crossJob)
	for _, want := range []string{"macos-latest", "windows-latest"} {
		if !slices.Contains(matrix, want) {
			t.Errorf("smoke-cross does not run on %s; its matrix is %v", want, matrix)
		}
	}
}

// Test_releaseSmokeWorkflow_coversEveryOS asserts the pre-release smoke workflow
// exercises the artifacts on all three operating systems, so a packaging
// regression is caught on the pull request that introduces it rather than at
// tag time.
func Test_releaseSmokeWorkflow_coversEveryOS(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, filepath.Join(workflowDir, "release-smoke.yml"))

	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatal("release-smoke workflow has no jobs block")
	}
	build, ok := jobs["build"].(map[string]any)
	if !ok {
		t.Fatal("release-smoke workflow has no 'build' job")
	}
	if build["runs-on"] != "ubuntu-latest" {
		t.Errorf("the build job runs on %v, want ubuntu-latest", build["runs-on"])
	}

	verify, ok := jobs["verify"].(map[string]any)
	if !ok {
		t.Fatal("release-smoke workflow has no 'verify' job")
	}
	matrix := jobMatrixOS(t, verify)
	for _, want := range []string{"macos-latest", "windows-latest"} {
		if !slices.Contains(matrix, want) {
			t.Errorf("the verify job does not run on %s; its matrix is %v", want, matrix)
		}
	}
}

// jobMatrixOS returns the strategy.matrix.os entries of a job.
func jobMatrixOS(t *testing.T, job map[string]any) []string {
	t.Helper()
	strategy, ok := job["strategy"].(map[string]any)
	if !ok {
		t.Fatal("job has no strategy block")
	}
	matrix, ok := strategy["matrix"].(map[string]any)
	if !ok {
		t.Fatal("job strategy has no matrix")
	}
	return stringSlice(matrix["os"])
}
