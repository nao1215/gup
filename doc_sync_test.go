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

// readmeSection returns the body of the section introduced by heading, ending
// at the next heading of the same or shallower level (so a "## " section keeps
// its "### " subsections, while a "### " section stops at its sibling). It
// reports false when the heading is absent. Scoping marker checks to a single
// section is what makes them meaningful: a whole-file search passes even after
// a marker has drifted into an unrelated part of the README.
func readmeSection(content, heading string) (string, bool) {
	level := strings.IndexByte(heading+" ", ' ') // number of leading '#' characters
	// Git checks the README out with CRLF endings on Windows, which would leave a
	// trailing \r on every line and never match a heading.
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if line == heading {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", false
	}
	inFence := false
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], " \t")
		// Shell comments inside fenced code blocks ("# Install files ...") look
		// exactly like an h1, so fences have to be tracked or a section ends early.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		if hashes > 0 && hashes <= level && strings.HasPrefix(trimmed[hashes:], " ") {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	return strings.Join(lines[start:], "\n"), true
}

// Test_englishReadme_hasRequiredSectionContent asserts that each documented
// section still carries the payload it is built around. The heading test above
// only checks that a heading EXISTS; it cannot catch a section whose body was
// gutted while the heading stayed. Keying off shell commands, URLs, and literal
// synopses makes the check robust against prose rewrites (issue #306), and each
// marker is looked up inside its own section so a marker that survives only
// somewhere else in the file still fails.
func Test_englishReadme_hasRequiredSectionContent(t *testing.T) {
	t.Parallel()
	// requiredSectionMarkers maps a README heading, verbatim, to the stable
	// strings that section's content is built around: shell commands, URLs, tool
	// names, and the literal command synopsis.
	requiredSectionMarkers := map[string][]string{
		// Verifying release integrity: the cosign / SLSA verification commands.
		"## Verifying release integrity": {
			"cosign verify-blob",                  // signed-checksum verification command
			"gh attestation verify gup_<version>", // SLSA build-provenance command
		},
		// Migrate: the command synopsis and the mise rationale link.
		"### Migrate binaries to a new $GOBIN": {
			"gup migrate BEFORE_PATH AFTER_PATH [BINARY...]", // command synopsis
			"https://mise.jdx.dev/",                          // "why this is useful" link
		},
		// Quiet output: the --quiet/-q example commands.
		"### Quiet output for large tool sets": {
			"gup update --quiet", // --quiet example
			"gup check -q",       // short-flag example
		},
		// Machine-readable JSON output: the --json command and a stable JSON payload.
		"### Machine-readable JSON output (for scripting / CI)": {
			"gup check --json",                 // --json example command
			"\"status\": \"update-available\"", // stable JSON payload from the example
		},
		// Disable colorized output: the --no-color / NO_COLOR examples and convention link.
		"### Disable colorized output": {
			"NO_COLOR=1 gup update", // NO_COLOR env-var example
			"https://no-color.org/", // NO_COLOR convention link
		},
		// Pin a tool to a specific version: the pin/unpin commands and the
		// pinned-channel JSON.
		"### Pin a tool to a specific version": {
			"gup pin golangci-lint v1.62.0", // pin example command
			"gup unpin golangci-lint",       // unpin example command
			"\"channel\": \"pinned\"",       // pinned gup.json entry
		},
		// Feature comparison: the migrate --force row is unique to this table, and
		// the benchmark result (folded into this section) carries the competitor
		// link and the measurement-environment note.
		"## Feature comparison": {
			"migrate --force", // command-scoped row unique to the comparison table
			"https://github.com/Gelio/go-global-update", // benchmarked competitor (table column)
			"AMD Ryzen AI Max+ 395",                     // benchmark measurement-environment note
		},
		// Generate man-pages: the MANPATH note added when man learned to honor
		// MANPATH.
		"### Generate man-pages (for linux, mac)": {
			"MANPATH", // man writes under each MANPATH entry's man1 dir
		},
		// Shell completion --install: the HOME requirement note. "`HOME`" is
		// backtick-wrapped and does not collide with "$XDG_CONFIG_HOME" (no
		// backtick precedes HOME there), so it only matches the --install paragraph.
		"### Generate shell completion file (for bash, zsh, fish, PowerShell)": {
			"`HOME`", // --install fails fast when HOME is empty
		},
	}
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	content := string(raw)
	for heading, markers := range requiredSectionMarkers {
		body, ok := readmeSection(content, heading)
		if !ok {
			t.Errorf("%s is missing the section %q", readmePath, heading)
			continue
		}
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Errorf("the %q section of %s is missing %q", heading, readmePath, marker)
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
	// canonicalCommands are install one-liners that must appear verbatim in the
	// "How to install" section. Keep this list in sync with its command blocks.
	canonicalCommands := []string{
		"go install github.com/nao1215/gup@latest",
		"brew install gup",
		"brew install nao1215/tap/gup",
		"winget install --id nao1215.gup",
		"mise use -g gup@latest",
		"nix profile install nixpkgs#gogup",
		"aqua g -i nao1215/gup",
		"paru -S gup-bin",
	}
	// obsoleteCommands are install lines gup has moved away from. Checking that
	// the canonical form is PRESENT does not catch a superseded line left sitting
	// next to it, so retired commands are asserted absent from the whole file.
	obsoleteCommands := []string{
		"brew install nao1215/gup", // the tap moved to nao1215/tap/gup
	}
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	content := string(raw)
	installSection, ok := readmeSection(content, "## How to install")
	if !ok {
		t.Fatalf(`%s is missing the "## How to install" section`, readmePath)
	}
	for _, command := range canonicalCommands {
		if !strings.Contains(installSection, command) {
			t.Errorf(`the "How to install" section of %s is missing or has a stale install command: expected verbatim %q`, readmePath, command)
		}
	}
	for _, command := range obsoleteCommands {
		if strings.Contains(content, command) {
			t.Errorf("%s still documents the retired install command %q", readmePath, command)
		}
	}
}
