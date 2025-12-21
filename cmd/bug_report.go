package cmd

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newBugReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "bug-report",
		Short:             "Submit a bug report at GitHub",
		Long:              "bug-report opens the default browser to start a bug report which will include useful system information.",
		Example:           "   gup bug-report",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(bugReport(cmd, args, openBrowser))
		},
	}
}

// openBrowserFunc is a function that opens a browser to the specified URL.
type openBrowserFunc func(string) bool

// bugReport opens the default browser to start a bug report which will include useful system information.
func bugReport(cmd *cobra.Command, _ []string, openBrowser openBrowserFunc) int {
	var buf bytes.Buffer

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
		additionalDetails = `## Additional details**
Any other useful data to share.
`
	)
	buf.WriteString(fmt.Sprintf("## gup version\n%s\n\n", cmd.Version))
	buf.WriteString(description)
	buf.WriteString(toReproduce)
	buf.WriteString(expectedBehavior)
	buf.WriteString(additionalDetails)

	body := buf.String()
	q := url.Values{
		"title": {"[Bug Report] Title"},
		"body":  {body},
	}
	url := "https://github.com/nao1215/gup/issues/new?" + q.Encode()

	if !openBrowser(url) {
		fmt.Print("Please file a new issue at https://github.com/nao1215/gup/issues/new using this template:\n\n")
		fmt.Print(body)
	}

	return 0
}
