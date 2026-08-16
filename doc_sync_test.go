package main

import (
	"os"
	"strings"
	"testing"
)

// README.md is the only user-facing documentation file in the repository
// (issue #282). These tests keep it honest: the structural sections must stay
// in place, the sections' content must not silently disappear, and the
// copy-pasteable install commands must stay current.

// readmePath is the English README, the single source of truth for user-facing
// documentation. Naming it once keeps goconst satisfied without a global.
const readmePath = "README.md"

func Test_englishReadme_hasRequiredSections(t *testing.T) {
	t.Parallel()
	// requiredEnglishSections lists structural headings the English README must
	// keep. Add new first-class sections here so a regression is caught early.
	requiredEnglishSections := []string{
		"## Supported OS",
		"## How to install",
		"## Verifying release integrity",
		"## How to use",
		"### Quiet output for large tool sets",
		"### Machine-readable JSON output",
		"### Disable colorized output",
		"## Feature comparison",
		"## Contributing",
		"## LICENSE",
	}
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	content := string(raw)
	for _, section := range requiredEnglishSections {
		if !strings.Contains(content, section) {
			t.Errorf("%s is missing required section %q", readmePath, section)
		}
	}
}

func Test_websiteFooter_hasCanonicalLicense(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("website/layouts/baseof.html")
	if err != nil {
		t.Fatalf("failed to read website footer template: %v", err)
	}
	content := string(raw)
	want := `<a href="https://github.com/nao1215/gup/blob/main/LICENSE">Apache License 2.0</a>`
	if !strings.Contains(content, want) {
		t.Errorf("website footer is missing the canonical license link %q", want)
	}
	if strings.Contains(strings.ToLower(content), "mit licensed") {
		t.Error("website footer incorrectly identifies gup as MIT licensed")
	}
}

// Test_englishReadme_hasRequiredSectionContent asserts that each documented
// section still carries the payload it is built around. The heading test above
// only checks that a heading EXISTS; it cannot catch a section whose body was
// gutted while the heading stayed. Keying off shell commands, URLs, and literal
// synopses makes the check robust against prose rewrites (issue #306).
func Test_englishReadme_hasRequiredSectionContent(t *testing.T) {
	t.Parallel()
	// requiredSectionMarkers maps a human-readable section name to the stable
	// strings that section's content is built around: shell commands, URLs, tool
	// names, and the literal command synopsis.
	requiredSectionMarkers := map[string][]string{
		// Verifying release integrity: the cosign / SLSA verification commands.
		"Verifying release integrity": {
			"cosign verify-blob",                  // signed-checksum verification command
			"gh attestation verify gup_<version>", // SLSA build-provenance command
		},
		// Migrate: the command synopsis and the mise rationale link.
		"Migrate": {
			"gup migrate BEFORE_PATH AFTER_PATH [BINARY...]", // command synopsis
			"https://mise.jdx.dev/",                          // "why this is useful" link
		},
		// Quiet output: the --quiet/-q example commands.
		"Quiet output": {
			"gup update --quiet", // --quiet example
			"gup check -q",       // short-flag example
		},
		// Machine-readable JSON output: the --json command and a stable JSON payload.
		"Machine-readable JSON output": {
			"gup check --json",                 // --json example command
			"\"status\": \"update-available\"", // stable JSON payload from the example
		},
		// Disable colorized output: the --no-color / NO_COLOR examples and convention link.
		"Disable colorized output": {
			"NO_COLOR=1 gup update", // NO_COLOR env-var example
			"https://no-color.org/", // NO_COLOR convention link
		},
		// Pin a tool to a specific version: the pin/unpin commands and the
		// pinned-channel JSON.
		"Pin a tool to a specific version": {
			"gup pin golangci-lint v1.62.0", // pin example command
			"gup unpin golangci-lint",       // unpin example command
			"\"channel\": \"pinned\"",       // pinned gup.json entry
		},
		// Feature comparison: the migrate --force row is unique to this table, and
		// the benchmark result (folded into this section) carries the competitor
		// link and the measurement-environment note.
		"Feature comparison": {
			"migrate --force", // command-scoped row unique to the comparison table
			"https://github.com/Gelio/go-global-update", // benchmarked competitor (table column)
			"AMD Ryzen AI Max+ 395",                     // benchmark measurement-environment note
		},
		// Generate man-pages: the MANPATH note added when man learned to honor
		// MANPATH.
		"Generate man-pages": {
			"MANPATH", // man writes under each MANPATH entry's man1 dir
		},
		// Shell completion --install: the HOME requirement note. "`HOME`" is
		// backtick-wrapped and does not collide with "$XDG_CONFIG_HOME" (no
		// backtick precedes HOME there), so it only matches the --install paragraph.
		"Shell completion --install": {
			"`HOME`", // --install fails fast when HOME is empty
		},
	}
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	content := string(raw)
	for section, markers := range requiredSectionMarkers {
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Errorf("%s is missing the %q section: expected to find %q", readmePath, section, marker)
			}
		}
	}
}

// Test_englishReadme_hasCanonicalInstallCommands asserts that the
// copy-pasteable install commands are byte-for-byte what gup actually supports.
// A stale command (e.g. `brew install nao1215/gup` after the tap moved to
// `brew install nao1215/tap/gup`) is a copy-paste hazard for users, and it is
// exactly the "content exists but is wrong" drift the section checks miss.
func Test_englishReadme_hasCanonicalInstallCommands(t *testing.T) {
	t.Parallel()
	// canonicalCommands are install one-liners that must appear verbatim. Keep
	// this list in sync with the command blocks in README.md's "How to install".
	canonicalCommands := []string{
		"go install github.com/nao1215/gup@latest",
		"brew install nao1215/tap/gup",
		"winget install --id nao1215.gup",
		"mise use -g gup@latest",
		"nix profile install nixpkgs#gogup",
	}
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	content := string(raw)
	for _, command := range canonicalCommands {
		if !strings.Contains(content, command) {
			t.Errorf("%s is missing or has a stale install command: expected verbatim %q", readmePath, command)
		}
	}
}
