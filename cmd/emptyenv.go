package cmd

import (
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/print"
)

// handleEmptyEnvironment renders the shared "no manageable binaries" outcome
// that update and check both reach when target resolution yields no packages,
// and returns the process exit code.
//
// explicitSelection is true when the user narrowed the selection themselves
// (positional targets, or update's --exclude); in that case the empty result is
// a usage error reported with usageErr. Otherwise the empty environment is a
// normal first-run condition, not an error (#350): an explicitly named --file is
// still validated so honoring explicit user input does not depend on unrelated
// environment state (#368), and the command emits an empty JSON array (--json)
// or an informational note before exiting 0.
func handleEmptyEnvironment(confFile string, jsonOut, explicitSelection bool, usageErr string) int {
	if explicitSelection {
		print.Err(usageErr)
		return 1
	}
	if err := configstate.ValidateExplicitFile(confFile); err != nil {
		print.Err(err)
		return 1
	}
	if jsonOut {
		if err := encodeJSONPackages(nil); err != nil {
			print.Err(err)
			return 1
		}
		return 0
	}
	print.Info(emptyEnvMessage)
	return 0
}
