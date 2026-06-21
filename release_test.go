package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

	for _, want := range []string{"features", "bug fixes", "performance", "documentation"} {
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
	raw, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("failed to read release workflow: %v", err)
	}
	content := string(raw)

	doc := map[string]any{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("release.yml is not valid YAML: %v", err)
	}

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

	if !strings.Contains(content, "sigstore/cosign-installer") {
		t.Error("release workflow does not install cosign")
	}
	if !strings.Contains(content, "attest-build-provenance") {
		t.Error("release workflow does not attest build provenance")
	}
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
