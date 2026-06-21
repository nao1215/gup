package main

import (
	"os"
	"strings"
	"testing"
)

// README.md is the source of truth for user-facing documentation (issue #282).
// These tests keep the translated READMEs honest: the English file must keep its
// structural sections, and every translation must carry the sync banner that
// links back to English so readers know where the latest information lives.

// translationSyncMarker is a language-agnostic marker that must sit next to the
// "this translation may lag behind English" banner in every translated README.
const translationSyncMarker = "<!-- gup:translation-sync -->"

// canonicalLanguageBar is the language switcher every translated README must
// carry verbatim, so each doc links to English and to every sibling language
// (including a self-link). This guards against drift like a missing self-link
// or a different ordering between translations.
const canonicalLanguageBar = "[English](../../README.md) | " +
	"[日本語](../ja/README.md) | " +
	"[Русский](../ru/README.md) | " +
	"[中文](../zh-cn/README.md) | " +
	"[한국어](../ko/README.md) | " +
	"[Español](../es/README.md) | " +
	"[Français](../fr/README.md)"

func Test_englishReadme_hasRequiredSections(t *testing.T) {
	t.Parallel()
	// requiredEnglishSections lists structural headings the English README must
	// keep. Add new first-class sections here so a regression is caught early.
	requiredEnglishSections := []string{
		"## Benchmark",
		"## Supported OS",
		"## How to install",
		"## Verifying release integrity",
		"## How to use",
		"## Contributing",
		"## LICENSE",
	}
	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	content := string(raw)
	for _, section := range requiredEnglishSections {
		if !strings.Contains(content, section) {
			t.Errorf("README.md is missing required section %q", section)
		}
	}
}

// Test_translatedReadmes_haveRequiredSections asserts that every section the
// English README carries (issue #306: Benchmark, release integrity, Migrate) is
// also present in each translation, keyed off language-independent content so a
// missing section is caught even though headings are translated.
func Test_translatedReadmes_haveRequiredSections(t *testing.T) {
	t.Parallel()
	// requiredSectionMarkers maps a human-readable section name to a set of
	// language-independent strings that the section's content always carries
	// verbatim, regardless of the translation language. Section HEADINGS are fully
	// translated (e.g. "## Benchmark" becomes "## 基准测试" / "## Бенчмарк"), so we
	// cannot key the presence check off heading text. Instead we key off the
	// stable, untranslated payload each section is built around: shell commands,
	// URLs, tool names, and the literal command synopsis. These are identical in
	// every README, so their presence is a robust proxy for "this section exists,
	// translated, in this file". The test fails if any translation drops one of the
	// sections the English README carries (issue #306).
	requiredSectionMarkers := map[string][]string{
		// Benchmark: the comparison table and measurement note.
		"Benchmark": {
			"https://github.com/Gelio/go-global-update", // benchmarked competitor (table row)
			"AMD Ryzen AI Max+ 395",                     // measurement environment note
		},
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
	}
	translatedREADMEs := []string{
		"doc/ja/README.md",
		"doc/es/README.md",
		"doc/fr/README.md",
		"doc/ko/README.md",
		"doc/ru/README.md",
		"doc/zh-cn/README.md",
	}
	for _, path := range translatedREADMEs {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(path) //nolint:gosec // fixed in-repo doc path
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}
			content := string(raw)
			for section, markers := range requiredSectionMarkers {
				for _, marker := range markers {
					if !strings.Contains(content, marker) {
						t.Errorf("%s is missing the %q section: expected to find %q", path, section, marker)
					}
				}
			}
		})
	}
}

func Test_translatedReadmes_haveSyncBanner(t *testing.T) {
	t.Parallel()
	// translatedREADMEs are the localized READMEs that must stay in sync with, or
	// be explicitly marked as lagging behind, the English README.
	translatedREADMEs := []string{
		"doc/ja/README.md",
		"doc/es/README.md",
		"doc/fr/README.md",
		"doc/ko/README.md",
		"doc/ru/README.md",
		"doc/zh-cn/README.md",
	}
	for _, path := range translatedREADMEs {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(path) //nolint:gosec // fixed in-repo doc path
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}
			content := string(raw)
			if !strings.Contains(content, translationSyncMarker) {
				t.Errorf("%s is missing the translation sync marker %q", path, translationSyncMarker)
			}
			// Every translation must link back to the English source of truth.
			if !strings.Contains(content, "../../README.md") {
				t.Errorf("%s does not link back to the English README (../../README.md)", path)
			}
			// Every translation must carry the same language switcher (English +
			// all siblings + a self-link), so navigation stays consistent.
			if !strings.Contains(content, canonicalLanguageBar) {
				t.Errorf("%s is missing the canonical language bar:\n%s", path, canonicalLanguageBar)
			}
		})
	}
}
