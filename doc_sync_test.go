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
