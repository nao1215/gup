package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// emptyEnvMessage is the informational note shown when no Go binaries are
// installed yet. An empty global environment is treated as a normal first-run
// condition rather than an error (#350).
const emptyEnvMessage = "no binaries are installed under $GOPATH/bin or $GOBIN"

// argsGuidance builds a concise, actionable error for a missing-argument
// situation: a one-line summary followed by one or two example invocations
// (issue #324). It deliberately stays short and never dumps full help.
func argsGuidance(summary string, examples ...string) error {
	if len(examples) == 0 {
		return fmt.Errorf("%s", summary)
	}
	return fmt.Errorf("%s\n\nExamples:\n  %s", summary, strings.Join(examples, "\n  "))
}

// requireMinArgs returns a cobra.PositionalArgs that requires at least minArgs
// positional arguments and, when fewer are given, reports argsGuidance instead
// of cobra's terse "requires at least N arg(s)" message.
func requireMinArgs(minArgs int, summary string, examples ...string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < minArgs {
			return argsGuidance(summary, examples...)
		}
		return nil
	}
}
