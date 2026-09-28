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

// permRead is the read-only GITHUB_TOKEN scope the workflows are held to.
const permRead = "read"

// The GitHub-hosted runner labels the cross-platform jobs are asserted against.
const (
	runnerMacOS   = "macos-latest"
	runnerWindows = "windows-latest"
	runnerUbuntu  = "ubuntu-latest"
)

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

	// The write scopes belong to the publishing job only; the smoke jobs run
	// with the workflow's read-only default.
	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no jobs")
	}
	release, ok := jobs["release"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no release job")
	}
	const write = "write"
	perms, ok := release["permissions"].(map[string]any)
	if !ok {
		t.Fatal("release job is missing a permissions block")
	}
	if perms["id-token"] != write {
		t.Errorf("release job needs 'id-token: write' for keyless signing/provenance, got %v", perms["id-token"])
	}
	if perms["attestations"] != write {
		t.Errorf("release job needs 'attestations: write' for provenance, got %v", perms["attestations"])
	}
	if perms["contents"] != write {
		t.Errorf("release job needs 'contents: write' to publish the release, got %v", perms["contents"])
	}
	if top, ok := doc["permissions"].(map[string]any); !ok || top["contents"] != permRead || len(top) != 1 {
		t.Errorf("release workflow's top-level permissions must be exactly 'contents: read', got %v", doc["permissions"])
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
// release job, which holds the tokens that publish to Homebrew and winget.
// Most workflows were already pinned; website.yml was not, and
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
		// GitHub Actions accepts both extensions, so a .yaml workflow must not be
		// able to slip past the pinning guardrail by spelling its suffix
		// differently.
		if entry.IsDir() || !isWorkflowFile(entry.Name()) {
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
			// The SLSA generator is the one reusable workflow that must be
			// referenced by a release tag: it reads its own ref to decide which
			// builder binary to fetch, and slsa-verifier rejects provenance whose
			// builder ref is not a vX.Y.Z tag. Scorecard's Pinned-Dependencies
			// exempts it for the same reason. The tag itself is asserted by
			// Test_releaseWorkflow_slsaProvenance.
			if strings.HasPrefix(ref, slsaGeneratorWorkflow+"@") {
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
	if perms["contents"] != permRead {
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
	for _, want := range []string{runnerMacOS, runnerWindows} {
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
	if build["runs-on"] != runnerUbuntu {
		t.Errorf("the build job runs on %v, want %s", build["runs-on"], runnerUbuntu)
	}

	verify, ok := jobs["verify"].(map[string]any)
	if !ok {
		t.Fatal("release-smoke workflow has no 'verify' job")
	}
	matrix := jobMatrixOS(t, verify)
	for _, want := range []string{runnerMacOS, runnerWindows} {
		if !slices.Contains(matrix, want) {
			t.Errorf("the verify job does not run on %s; its matrix is %v", want, matrix)
		}
	}
}

// jobMatrixOS returns the strategy.matrix.os entries of a job, after checking
// that the job's runs-on actually resolves from that matrix.
//
// Declaring macOS and Windows in a matrix proves nothing on its own: a job whose
// runs-on is hard-coded to ubuntu-latest runs every leg on Ubuntu while the
// matrix still lists three names, and the leg that was supposed to execute a
// Windows binary quietly does not. The binding is the part that matters.
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

	runsOn, _ := job["runs-on"].(string)
	if !strings.Contains(runsOn, "matrix.os") {
		t.Errorf("runs-on is %q, want it to resolve from matrix.os; otherwise every matrix leg runs on the same runner", runsOn)
	}
	return stringSlice(matrix["os"])
}

// isWorkflowFile reports whether name is a GitHub Actions workflow. Both
// extensions are accepted by GitHub, so both are inspected here.
func isWorkflowFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yml" || ext == ".yaml"
}

// Test_e2eWorkflow_runsOnEveryOS asserts the end-to-end suite is not quietly
// Linux-only again. gup ships on three operating systems and behaves differently
// on each -- the .exe suffix, the config directory, PowerShell completion -- so a
// suite that drives the real binary on one of them is testing a third of what it
// claims to.
func Test_e2eWorkflow_runsOnEveryOS(t *testing.T) {
	t.Parallel()
	doc := readYAMLFile(t, filepath.Join(workflowDir, "e2e.yml"))

	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatal("e2e workflow has no jobs block")
	}
	job, ok := jobs["e2e"].(map[string]any)
	if !ok {
		t.Fatal("e2e workflow has no 'e2e' job")
	}

	// The matrix is a fromJSON expression so pull requests get a shorter list
	// than pushes; asserting on the raw expression is what keeps that check
	// honest without evaluating GitHub's expression language here.
	strategy, ok := job["strategy"].(map[string]any)
	if !ok {
		t.Fatal("the e2e job has no strategy block")
	}
	matrix, ok := strategy["matrix"].(map[string]any)
	if !ok {
		t.Fatal("the e2e job strategy has no matrix")
	}
	if runsOn, _ := job["runs-on"].(string); !strings.Contains(runsOn, "matrix.os") {
		t.Errorf("the e2e job's runs-on is %q, want it to resolve from matrix.os", runsOn)
	}
	// A plain list, not an expression: the matrix must not be able to shrink for
	// pull requests, because a leg that runs only after merge reports a break
	// when it is already on main.
	osList := stringSlice(matrix["os"])
	for _, want := range []string{runnerUbuntu, runnerWindows, runnerMacOS} {
		if !slices.Contains(osList, want) {
			t.Errorf("the e2e matrix never runs on %s; os = %v", want, osList)
		}
	}

	// A hung scenario must fail the job rather than occupy a runner for six
	// hours, which is the default when no timeout is set.
	if _, ok := job["timeout-minutes"]; !ok {
		t.Error("the e2e job has no timeout-minutes; a hung scenario would run until GitHub's six-hour cap")
	}

	// The bootstrap has to be the Go runner: a bash one would make the Windows
	// leg depend on Git for Windows rather than on gup.
	steps, ok := job["steps"].([]any)
	if !ok {
		t.Fatal("the e2e job has no steps")
	}
	ranRunner := false
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if run, ok := step["run"].(string); ok && strings.Contains(run, "go run ./e2e/runner") {
			ranRunner = true
		}
	}
	if !ranRunner {
		t.Error("the e2e job does not run 'go run ./e2e/runner'")
	}
}

// slsaGeneratorWorkflow is the reusable workflow that writes the SLSA build
// provenance attached to every release.
const slsaGeneratorWorkflow = "slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml"

// Test_releaseWorkflow_slsaProvenance asserts that every release ships SLSA
// build provenance as a release asset (multiple.intoto.jsonl) and that the
// release run verifies it. The GitHub attestation in the release job is not a
// release asset, so without this job a user holding only the downloaded files
// has no provenance to check, and OpenSSF Scorecard's Signed-Releases check
// sees none either. The chain has three links, and each one is checked here:
// the release job exports the base64 of checksums.txt, the provenance job
// hands it to the generator with upload-assets, and the verification job runs
// slsa-verifier against what was published.
func Test_releaseWorkflow_slsaProvenance(t *testing.T) {
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
	outputs, ok := release["outputs"].(map[string]any)
	if !ok {
		t.Fatal("the release job has no outputs; the provenance job has no subjects to attest")
	}
	if hashes, _ := outputs["hashes"].(string); !strings.Contains(hashes, "steps.hash.outputs.hashes") {
		t.Errorf("the release job's hashes output is %q, want it to come from the 'hash' step", hashes)
	}
	if !releaseHasStep(release, "run-goreleaser") {
		t.Error("the GoReleaser step has no 'id: run-goreleaser'; the subject step cannot read its artifacts output")
	}
	if !releaseHasStep(release, "hash") {
		t.Error("the release job has no step with 'id: hash' that writes the provenance subjects")
	}

	provenance, ok := jobs["provenance"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no 'provenance' job")
	}
	uses, _ := provenance["uses"].(string)
	generatorTag := regexp.MustCompile(`^` + regexp.QuoteMeta(slsaGeneratorWorkflow) + `@v\d+\.\d+\.\d+$`)
	if !generatorTag.MatchString(uses) {
		t.Errorf("the provenance job uses %q, want %s@vX.Y.Z (a release tag, not a SHA or branch)", uses, slsaGeneratorWorkflow)
	}
	if needs := stringSlice(provenance["needs"]); !slices.Contains(needs, "release") {
		t.Errorf("the provenance job does not depend on 'release' (needs = %v)", needs)
	}
	with, ok := provenance["with"].(map[string]any)
	if !ok {
		t.Fatal("the provenance job passes no inputs to the generator")
	}
	if subjects, _ := with["base64-subjects"].(string); !strings.Contains(subjects, "needs.release.outputs.hashes") {
		t.Errorf("the provenance job's base64-subjects is %q, want needs.release.outputs.hashes", subjects)
	}
	if with["upload-assets"] != true {
		t.Errorf("the provenance job must set upload-assets: true so multiple.intoto.jsonl is a release asset, got %v", with["upload-assets"])
	}
	const write = "write"
	perms, ok := provenance["permissions"].(map[string]any)
	if !ok {
		t.Fatal("the provenance job has no permissions block")
	}
	for scope, want := range map[string]string{"actions": permRead, "id-token": write, "contents": write} {
		if perms[scope] != want {
			t.Errorf("the provenance job needs '%s: %s', got %v", scope, want, perms[scope])
		}
	}
	if len(perms) != 3 {
		t.Errorf("the provenance job grants more than actions/id-token/contents: %v", perms)
	}

	verification, ok := jobs["verification"].(map[string]any)
	if !ok {
		t.Fatal("release workflow has no 'verification' job; published provenance would never be checked")
	}
	needs := stringSlice(verification["needs"])
	for _, want := range []string{"release", "provenance"} {
		if !slices.Contains(needs, want) {
			t.Errorf("the verification job does not depend on %q (needs = %v)", want, needs)
		}
	}
	if vperms, ok := verification["permissions"].(map[string]any); !ok || vperms["contents"] != permRead || len(vperms) != 1 {
		t.Errorf("the verification job must run with exactly 'contents: read', got %v", verification["permissions"])
	}
	installsVerifier, runsVerifier := false, false
	steps, _ := verification["steps"].([]any)
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if u, ok := step["uses"].(string); ok && strings.HasPrefix(u, "slsa-framework/slsa-verifier/actions/installer@") {
			installsVerifier = true
		}
		if run, ok := step["run"].(string); ok && strings.Contains(run, "slsa-verifier verify-artifact") {
			runsVerifier = true
		}
	}
	if !installsVerifier {
		t.Error("the verification job does not install slsa-verifier")
	}
	if !runsVerifier {
		t.Error("the verification job never runs 'slsa-verifier verify-artifact'")
	}
}

// releaseHasStep reports whether the job has a step with the given id.
func releaseHasStep(job map[string]any, id string) bool {
	steps, _ := job["steps"].([]any)
	for _, s := range steps {
		if step, ok := s.(map[string]any); ok && step["id"] == id {
			return true
		}
	}
	return false
}
