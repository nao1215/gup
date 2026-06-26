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
// normal first-run condition, not an error (#350): the config gup would read
// (explicit --file or auto-detected) is still validated so an empty environment
// fails fast on the same config problems a non-empty one would — ambiguous
// resolution, malformed file, invalid schema/channel/pin data (#368) — instead
// of silently succeeding just because zero binaries are installed. With no
// config problem, the command emits an empty JSON array (--json) or an
// informational note before exiting 0.
func handleEmptyEnvironment(p *print.Printer, confFile string, jsonOut, explicitSelection bool, usageErr string) int {
	if explicitSelection {
		p.Err(usageErr)
		return 1
	}
	if err := configstate.ValidateResolvedConfig(confFile); err != nil {
		p.Err(err)
		return 1
	}
	if jsonOut {
		if err := encodeJSONPackages(p, nil); err != nil {
			p.Err(err)
			return 1
		}
		return 0
	}
	p.Info(emptyEnvMessage)
	return 0
}
