package cmd

import (
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/print"
)

// handleEmptyEnvironment renders the shared "no manageable binaries" outcome
// that update and check both reach when target resolution yields no packages,
// and returns the process exit code.
//
// explicitSelection is true when the user named the binaries to act on
// (positional targets); in that case the empty result means none of those names
// exist, a usage error reported with usageErr. update's --exclude is a filter,
// not a selection, and never makes an empty environment an error (#422). Otherwise the empty environment is a
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
	return handleNothingSelected(p, confFile, jsonOut, emptyEnvMessage)
}

// handleNothingSelected renders a successful "nothing to do" outcome: an empty
// JSON array (--json) or note as an informational message, exit 0. The config
// gup would read is still validated first, so an empty selection fails fast on
// the same config problems a non-empty one would (#368) instead of silently
// succeeding just because there is no work.
func handleNothingSelected(p *print.Printer, confFile string, jsonOut bool, note string) int {
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
	p.Info(note)
	return 0
}
