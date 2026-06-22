package cmd

import (
	"bytes"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/spf13/cobra"
)

func newBugReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "bug-report",
		Short:             "Submit a bug report at GitHub",
		Long:              "bug-report opens the default browser to start a bug report pre-filled with your gup version and OS.",
		Example:           "  gup bug-report",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(bugReport(cmd, args, openBrowser))
		},
	}
}

// openBrowserFunc is a function that opens a browser to the specified URL.
type openBrowserFunc func(string) bool

// bugReport opens the default browser to start a bug report pre-filled with the
// gup version and OS. The issue title is intentionally left empty so the user is
// prompted to write a descriptive one instead of submitting a placeholder.
func bugReport(cmd *cobra.Command, _ []string, openBrowser openBrowserFunc) int {
	var buf bytes.Buffer
	version := strings.TrimSpace(cmd.Version)
	if version == "" {
		version = cmdinfo.GetVersion()
	}

	const (
		description = `## Description (About the problem)
A clear description of the bug encountered.

`
		toReproduce = `## Steps to reproduce
Steps to reproduce the bug.

`
		expectedBehavior = `## Expected behavior
Expected behavior.

`
		additionalDetails = `## Additional details
Any other useful data to share.
`
	)
	_, _ = fmt.Fprintf(&buf, "## gup version\n%s\n\n", version)
	_, _ = fmt.Fprintf(&buf, "## OS\n%s/%s\n\n", runtime.GOOS, runtime.GOARCH)
	buf.WriteString(description)
	buf.WriteString(toReproduce)
	buf.WriteString(expectedBehavior)
	buf.WriteString(additionalDetails)

	body := buf.String()
	// Leave the title empty so GitHub prompts the user for a descriptive title
	// rather than offering a placeholder that often gets submitted unchanged.
	q := url.Values{
		"body": {body},
	}
	url := "https://github.com/nao1215/gup/issues/new?" + q.Encode()

	if !openBrowser(url) {
		fmt.Print("Please file a new issue at https://github.com/nao1215/gup/issues/new using this template:\n\n")
		fmt.Print(body)
	}

	return 0
}
