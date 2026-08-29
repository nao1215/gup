package completion

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/spf13/cobra"
)

// PowerShell completion install.
//
// bash, fish and zsh all read completion out of a directory their shell already
// scans, so installing them is writing one file. PowerShell has no such
// directory: a completer is a script that must be dot-sourced from the user's
// profile, which is why `gup completion --install` used to refuse on Windows and
// tell the user to redirect `gup completion powershell` into a file and wire it
// up by hand. That is three manual steps on the one platform where a user is
// least likely to have a shell-config habit.
//
// So the install does the wiring: it writes the generated completer beside the
// profile and adds one dot-source line to the profile itself, inside a marked
// block gup owns. The profile is a file users keep their own configuration in,
// so nothing outside the markers is ever touched, and the block is matched by
// marker rather than by content, which makes a re-run replace it in place
// instead of appending a second copy. This mirrors exactly what the zsh install
// already does with the fpath block in .zshrc.

const (
	// psProfileMarker opens gup's managed block in the PowerShell profile, and
	// psProfileEndMarker closes it. Two markers rather than zsh's one because the
	// body here is a single line that a user might reasonably reformat: an
	// explicit terminator keeps the block's extent unambiguous.
	psProfileMarker    = "# setting for gup command (auto generate)"
	psProfileEndMarker = "# end of setting for gup command"

	// psCompletionFileName is the generated completer, written next to the
	// profile so the two travel together when a user copies their PowerShell
	// directory to another machine.
	psCompletionFileName = "gup.completion.ps1"
)

// psProfileBlockRE matches gup's managed block: the opening marker, anything it
// contains, and the closing marker. It is non-greedy so two blocks (which should
// not happen, but a hand-edited profile can produce) are matched separately
// rather than swallowing everything between the first and last marker. A block
// whose terminator was deleted is repaired by the marker-only fallback below.
var psProfileBlockRE = regexp.MustCompile(
	`(?s)[ \t]*` + regexp.QuoteMeta(psProfileMarker) + `[ \t]*\n.*?` +
		regexp.QuoteMeta(psProfileEndMarker) + `[ \t]*\n?`)

// psOrphanMarkerRE matches an opening marker whose closing marker is gone, plus
// the line under it. Without this a profile broken by hand would collect a fresh
// block on every --install, which is the duplication the markers exist to avoid.
var psOrphanMarkerRE = regexp.MustCompile(
	`[ \t]*` + regexp.QuoteMeta(psProfileMarker) + `[ \t]*\n(?:[^\n]*\n?)?`)

// deployPowerShellCompletion installs PowerShell completion for the current
// user: it writes the completer script and makes the profile source it. Both
// writes are atomic, and an already-correct install rewrites nothing, so
// re-running --install is a no-op rather than a duplicate.
func deployPowerShellCompletion(cmd *cobra.Command) error {
	profilePath, err := powerShellProfilePath()
	if err != nil {
		return err
	}

	completionPath := filepath.Join(filepath.Dir(profilePath), psCompletionFileName)
	// sync compares before writing, so an unchanged completer is left alone -
	// the same "re-running --install rewrites nothing" behavior the POSIX shells
	// get. The profile is reconciled afterwards either way, so a block a user
	// deleted is repaired even when the completer itself is already current.
	if err := powerShellCompletionSpec(completionPath).sync(cmd); err != nil {
		return err
	}
	return syncPowerShellProfile(profilePath, completionPath)
}

// powerShellCompletionSpec describes the generated completer file, reusing the
// same "generate, compare, write if changed" type the POSIX shells use.
func powerShellCompletionSpec(path string) completionFile {
	return completionFile{
		shell: "powershell",
		path:  func() string { return path },
		generate: func(cmd *cobra.Command, w io.Writer) error {
			return cmd.GenPowerShellCompletionWithDesc(w)
		},
	}
}

// powerShellProfileBlock is the block gup writes into the profile. The Test-Path
// guard matters: a user who deletes the completer file, or copies their profile
// to a machine without gup, gets a working shell rather than an error on every
// prompt.
func powerShellProfileBlock(completionPath string) string {
	// PowerShell single quotes take no escapes except a doubled quote, which is
	// the right quoting for a Windows path (backslashes stay literal).
	quoted := "'" + strings.ReplaceAll(completionPath, "'", "''") + "'"
	return fmt.Sprintf("%s\nif (Test-Path %s) { . %s }\n%s\n",
		psProfileMarker, quoted, quoted, psProfileEndMarker)
}

// syncPowerShellProfile reconciles gup's block in the profile without disturbing
// anything else in it. A missing profile is created (with its parent directory),
// an existing one keeps every line the user wrote, an existing gup block is
// replaced in place, and an already-correct profile is not rewritten at all.
func syncPowerShellProfile(profilePath, completionPath string) error {
	block := powerShellProfileBlock(completionPath)

	if !fileutil.IsFile(profilePath) {
		return atomicWriteFile(profilePath, []byte(block), "powershell profile")
	}

	raw, err := os.ReadFile(filepath.Clean(profilePath))
	if err != nil {
		return fmt.Errorf("can not read the PowerShell profile %s: %w", profilePath, err)
	}
	content := string(raw)

	var updated string
	switch {
	case psProfileBlockRE.MatchString(content):
		updated = psProfileBlockRE.ReplaceAllLiteralString(content, block)
	case psOrphanMarkerRE.MatchString(content):
		updated = psOrphanMarkerRE.ReplaceAllLiteralString(content, block)
	default:
		updated = appendBlock(content, block)
	}

	if updated == content {
		return nil
	}
	return atomicWriteFile(profilePath, []byte(updated), "powershell profile")
}

// appendBlock adds the block to the end of an existing profile, separated by a
// blank line and without clobbering a final line that has no newline of its own.
func appendBlock(content, block string) string {
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + block
}

// powerShellProfilePath decides which profile to wire up.
//
// $PROFILE is PowerShell's own answer to this question, and it wins when it is
// exported, because a user with a relocated profile has already told the system
// where it is. PowerShell does not export it by default, though, so the fallback
// reconstructs the standard locations from the user's home directory and prefers
// one that already exists: a user running PowerShell 5.1 has
// Documents\WindowsPowerShell, a user on PowerShell 7 has Documents\PowerShell,
// and writing to the wrong one would produce an install that silently does
// nothing. With neither present, the PowerShell 7 path is created, since that is
// the shell a new install gets.
func powerShellProfilePath() (string, error) {
	if profile := strings.TrimSpace(os.Getenv("PROFILE")); profile != "" {
		if !filepath.IsAbs(profile) {
			return "", fmt.Errorf(
				"PROFILE must be an absolute path to install PowerShell completion, but is a relative path: %q",
				profile)
		}
		return profile, nil
	}

	home, err := powerShellHome()
	if err != nil {
		return "", err
	}

	candidates := powerShellProfileCandidates(home)
	for _, candidate := range candidates {
		if fileutil.IsFile(candidate) {
			return candidate, nil
		}
	}
	return candidates[0], nil
}

// powerShellProfileCandidates lists the profile locations to consider, most
// preferred first. OneDrive is included because Windows redirects the Documents
// folder there on a large share of consumer machines, and a profile living under
// the redirected path is the one PowerShell actually reads.
func powerShellProfileCandidates(home string) []string {
	const profileName = "Microsoft.PowerShell_profile.ps1"
	return []string{
		filepath.Join(home, "Documents", "PowerShell", profileName),
		filepath.Join(home, "Documents", "WindowsPowerShell", profileName),
		filepath.Join(home, "OneDrive", "Documents", "PowerShell", profileName),
		filepath.Join(home, "OneDrive", "Documents", "WindowsPowerShell", profileName),
	}
}

// powerShellHome returns the user's home directory, from USERPROFILE (which is
// what Windows sets) or HOME (which is what a POSIX host running PowerShell
// sets). It fails fast on an unset or relative value for the same reason the
// POSIX install fails fast on a relative HOME: gup never absolutizes such a
// value, because doing so would write configuration into whatever directory the
// user happened to be in.
func powerShellHome() (string, error) {
	for _, name := range []string{"USERPROFILE", "HOME"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			continue
		}
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf(
				"%s must be an absolute path to install PowerShell completion, but is a relative path: %q",
				name, value)
		}
		return value, nil
	}
	return "", fmt.Errorf(
		"neither USERPROFILE nor HOME is set; cannot determine where to install %s completion for PowerShell",
		cmdinfo.Name)
}
