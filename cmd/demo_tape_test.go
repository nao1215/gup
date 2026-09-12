package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// tapeDir holds the VHS scripts that render the GIFs README.md embeds.
const tapeDir = "../doc/img"

// TestDemoTapeCommandsStillResolve requires every gup command line the tapes
// type to still name a real subcommand whose flags and argument count are
// accepted.
//
// A GIF cannot be diffed to prove it is current -- VHS output varies with
// timing, fonts and the encoder -- so what CI can check is the thing that
// actually rots: the command lines inside the tape. Rename a subcommand, drop a
// flag, or tighten an Args rule and this fails here, instead of the next
// `vhs doc/img/update.tape` quietly recording a terminal full of usage errors.
//
// The commands are resolved and validated rather than run. `gup update` and
// `gup check` reach the module proxy, and a test that needs the network to
// prove a tape is current is a test that fails for reasons the tape knows
// nothing about.
func TestDemoTapeCommandsStillResolve(t *testing.T) {
	t.Parallel()

	commands := gupCommandsInTapes(t)
	if len(commands) == 0 {
		t.Fatalf("%s types no gup commands; the tapes or this parser are wrong", tapeDir)
	}

	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			args := strings.Fields(command)[1:]
			root := newRootCmd()

			target, rest, err := root.Find(args)
			if err != nil {
				t.Fatalf("%q: %v", command, err)
			}
			if target == root {
				t.Fatalf("%q does not name a subcommand", command)
			}
			if err := target.ParseFlags(rest); err != nil {
				t.Fatalf("%q: %v", command, err)
			}
			if err := target.ValidateArgs(target.Flags().Args()); err != nil {
				t.Fatalf("%q: %v", command, err)
			}
		})
	}
}

// gupCommandsInTapes returns every gup command line the tapes type, once each.
// VHS keeps typed text in `Type "..."` directives; everything else in a tape
// (Set, Sleep, Hide, Screenshot, shell one-liners) is not a gup invocation.
func gupCommandsInTapes(t *testing.T) []string {
	t.Helper()

	tapes, err := filepath.Glob(filepath.Join(tapeDir, "*.tape"))
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, tape := range tapes {
		data, err := os.ReadFile(tape) //nolint:gosec // a path this test globbed inside the repository
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			typed, ok := typedText(strings.TrimSpace(line))
			if !ok || !strings.HasPrefix(typed, "gup ") {
				continue
			}
			seen[typed] = true
		}
	}

	commands := make([]string, 0, len(seen))
	for command := range seen {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}

// typedText extracts the quoted argument of a VHS `Type "..."` directive. A tape
// line may carry more directives after it (`Type "gup list" Sleep 400ms Enter`),
// so the text ends at the closing quote rather than at the end of the line.
func typedText(line string) (string, bool) {
	const prefix = `Type "`
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	rest := line[len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// TestDemoTapesRenderTheGIFsTheReadmeEmbeds pins each tape's Output path. The
// README embeds doc/img/*.gif by name; a tape that renders somewhere else leaves
// the README showing a stale image with nothing failing.
func TestDemoTapesRenderTheGIFsTheReadmeEmbeds(t *testing.T) {
	t.Parallel()

	for tape, want := range map[string]string{
		"sample.tape": "Output doc/img/sample.gif",
		"update.tape": "Output doc/img/update.gif",
		"list.tape":   "Output doc/img/list.gif",
	} {
		t.Run(tape, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(tapeDir, tape)) //nolint:gosec // a fixed path inside the repository
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), want) {
				t.Errorf("%s does not contain %q, so it no longer renders the GIF README.md embeds", tape, want)
			}
		})
	}
}
