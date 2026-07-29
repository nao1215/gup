package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The website is intentionally assembled from small, copyable examples and a
// cookbook. Keep those examples tied to the Cobra command tree: a renamed flag
// or command must fail this test while the documentation change is still in
// review.
type documentedInvocation struct {
	source string
	line   int
	args   []string
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func readRepositoryFile(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec // test paths are fixed below
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func shellWords(line string) []string {
	var words []string
	var word strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if word.Len() == 0 {
			return
		}
		words = append(words, word.String())
		word.Reset()
	}

	for _, r := range line {
		if escaped {
			word.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t':
			flush()
		case '#', '|', ';', '&', '>':
			flush()
			words = append(words, string(r))
		default:
			word.WriteRune(r)
		}
	}
	if escaped {
		word.WriteRune('\\')
	}
	flush()
	return words
}

func documentedCommands(source, markdown string) []documentedInvocation {
	var invocations []documentedInvocation
	lines := strings.Split(markdown, "\n")
	inFence := false
	for lineNumber, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "$ ") {
			trimmed = strings.TrimPrefix(trimmed, "$ ")
		}
		words := shellWords(trimmed)
		for i, word := range words {
			if word == "#" || word == "|" || word == ";" || word == "&" || word == ">" {
				break
			}
			if strings.HasPrefix(word, "gup:") {
				continue
			}
			if word == "gup" || word == "\\gup" {
				args := words[i+1:]
				for j, arg := range args {
					if arg == "#" || arg == "|" || arg == ";" || arg == "&" || arg == ">" {
						args = args[:j]
						break
					}
				}
				invocations = append(invocations, documentedInvocation{
					source: source,
					line:   lineNumber + 1,
					args:   append([]string(nil), args...),
				})
				break
			}
			if word == "sudo" || strings.HasPrefix(word, "NO_COLOR=") {
				continue
			}
			if word == "go" && i+2 < len(words) && words[i+1] == "run" &&
				strings.HasPrefix(words[i+2], "github.com/nao1215/gup@") {
				args := words[i+3:]
				for j, arg := range args {
					if arg == "#" || arg == "|" || arg == ";" || arg == "&" || arg == ">" {
						args = args[:j]
						break
					}
				}
				invocations = append(invocations, documentedInvocation{
					source: source,
					line:   lineNumber + 1,
					args:   append([]string(nil), args...),
				})
				break
			}
		}
	}
	return invocations
}

func websiteMarkdownSources(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "website", "content", "*.md"))
	if err != nil {
		t.Fatalf("glob website Markdown: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("website/content contains no Markdown pages")
	}
	sources := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("make website path relative: %v", err)
		}
		sources = append(sources, filepath.ToSlash(rel))
	}
	sort.Strings(sources)
	return sources
}

func validateDocumentedInvocation(invocation documentedInvocation) error {
	// man is intentionally not registered on Windows, while the cookbook
	// documents it for Linux and macOS.
	if runtime.GOOS == "windows" && len(invocation.args) > 0 && invocation.args[0] == "man" {
		return nil
	}
	root := newRootCmd()
	command, args, err := root.Find(invocation.args)
	if err != nil {
		return err
	}
	if err := command.ParseFlags(args); err != nil {
		return err
	}
	return nil
}

func TestDocs_EveryDocumentedInvocationParses(t *testing.T) {
	root := repositoryRoot(t)
	sources := append([]string{"README.md", "doc/cookbook.md"}, websiteMarkdownSources(t, root)...)
	var invocations []documentedInvocation
	for _, source := range sources {
		invocations = append(invocations, documentedCommands(source, readRepositoryFile(t, root, source))...)
	}
	if len(invocations) < 50 {
		t.Fatalf("found only %d documented gup invocations; the extractor or the docs probably regressed", len(invocations))
	}
	for _, invocation := range invocations {
		invocation := invocation
		t.Run(fmt.Sprintf("%s:%d %s", invocation.source, invocation.line, strings.Join(invocation.args, " ")), func(t *testing.T) {
			if err := validateDocumentedInvocation(invocation); err != nil {
				t.Fatalf("%s:%d documents a command the parser rejects: gup %s: %v", invocation.source, invocation.line, strings.Join(invocation.args, " "), err)
			}
		})
	}
}

func TestCookbook_EveryRecipeSectionIsExercised(t *testing.T) {
	root := repositoryRoot(t)
	cookbook := readRepositoryFile(t, root, "doc/cookbook.md")
	spec := readRepositoryFile(t, root, "e2e/atago/cookbook.atago.yaml")

	sectionPattern := regexp.MustCompile(`^## (.+)$`)
	scenarioPattern := regexp.MustCompile(`^  - name: "([^"]+)"$`)
	sections := make(map[string]bool)
	for _, line := range strings.Split(cookbook, "\n") {
		match := sectionPattern.FindStringSubmatch(line)
		if len(match) == 2 && match[1] != "Find a recipe by task" {
			sections[match[1]] = true
		}
	}

	covered := make(map[string]bool)
	for _, line := range strings.Split(spec, "\n") {
		match := scenarioPattern.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		name := match[1]
		section, _, ok := strings.Cut(name, ": ")
		if !ok {
			t.Errorf("scenario %q does not identify its cookbook section", name)
			continue
		}
		if !sections[section] {
			t.Errorf("scenario %q names a cookbook section that does not exist", name)
		}
		covered[section] = true
	}

	var missing []string
	for section := range sections {
		if !covered[section] {
			missing = append(missing, section)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("cookbook sections have no executable atago scenario: %s", strings.Join(missing, ", "))
	}
}

func markdownHeadingIDs(markdown string) map[string]bool {
	ids := make(map[string]bool)
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		text = strings.ToLower(text)
		text = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(text, "-")
		ids[strings.Trim(text, "-")] = true
	}
	return ids
}

func TestSite_InternalLinksAndImagesResolve(t *testing.T) {
	root := repositoryRoot(t)
	documents := []struct {
		path string
	}{
		{path: "doc/cookbook.md"},
	}
	routes := map[string]bool{"/": true, "/cookbook/": true}
	for _, source := range websiteMarkdownSources(t, root) {
		name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
		route := "/"
		if name != "_index" {
			route = "/" + name + "/"
		}
		routes[route] = true
		documents = append(documents, struct {
			path string
		}{path: source})
	}
	linkPattern := regexp.MustCompile(`\]\(([^)\s]+)`) // Markdown destinations cannot contain an unescaped space here.

	for _, document := range documents {
		markdown := readRepositoryFile(t, root, document.path)
		headingIDs := markdownHeadingIDs(markdown)
		for _, match := range linkPattern.FindAllStringSubmatch(markdown, -1) {
			destination := match[1]
			if strings.HasPrefix(destination, "http://") || strings.HasPrefix(destination, "https://") ||
				strings.HasPrefix(destination, "mailto:") || strings.HasPrefix(destination, "//") {
				continue
			}
			path, fragment, _ := strings.Cut(destination, "#")
			if path == "" {
				if !headingIDs[fragment] {
					t.Errorf("%s links to missing heading #%s", document.path, fragment)
				}
				continue
			}
			if strings.HasPrefix(path, "/img/") {
				if _, err := os.Stat(filepath.Join(root, "doc", strings.TrimPrefix(path, "/"))); err != nil {
					t.Errorf("%s references missing image %s: %v", document.path, path, err)
				}
				continue
			}
			if !strings.HasSuffix(path, "/") {
				path += "/"
			}
			if !routes[path] {
				t.Errorf("%s links to %s, which is not a page of the site", document.path, destination)
			}
		}
	}
}

func TestSite_CookbookUsesCommittedSource(t *testing.T) {
	root := repositoryRoot(t)
	template := readRepositoryFile(t, root, "website/content/_content.gotmpl")
	config := readRepositoryFile(t, root, "website/hugo.toml")
	if !strings.Contains(template, `resources.Get "doc/cookbook.md"`) {
		t.Error("cookbook page is not generated from doc/cookbook.md")
	}
	if !strings.Contains(config, `source = "../doc/cookbook.md"`) {
		t.Error("doc/cookbook.md is not mounted into the Hugo site")
	}
}
