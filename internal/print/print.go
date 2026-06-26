// Package print defines functions to accept colored standard output and user input
package print

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/gup/internal/cmdinfo"
)

// Printer writes gup's colored, prefixed messages to a pair of writers. Threading
// a *Printer through the command and helper layers (instead of reading
// package-level Stdout/Stderr globals) lets each caller decide where output goes
// and lets tests capture output by constructing a buffer-backed Printer and
// passing it in, so they no longer mutate shared globals and can run in parallel.
type Printer struct {
	out    io.Writer
	err    io.Writer
	scanln func(a ...any) (n int, err error)
	exit   func(code int)
}

// New returns a Printer that writes normal output to out and warnings/errors to
// err. User input (Question) is read with fmt.Scanln and Fatal exits via os.Exit;
// tests in this package override those fields directly.
func New(out, err io.Writer) *Printer {
	return &Printer{out: out, err: err, scanln: fmt.Scanln, exit: os.Exit}
}

// NewColorable returns the production Printer, writing to the colorable wrappers
// of the process stdout/stderr so escape sequences render on every platform.
func NewColorable() *Printer {
	return New(ColorableStdout(), ColorableStderr())
}

// ColorableStdout returns the process stdout wrapped so escape sequences render
// on every platform. It lets the command layer wire the production output sink
// onto the root command (via cobra SetOut) without importing go-colorable.
func ColorableStdout() io.Writer { return colorable.NewColorableStdout() }

// ColorableStderr returns the process stderr wrapped so escape sequences render
// on every platform.
func ColorableStderr() io.Writer { return colorable.NewColorableStderr() }

// Out returns the writer used for normal output, for callers that must write
// structured output (JSON, config files, formatted tables) directly.
func (p *Printer) Out() io.Writer { return p.out }

// ErrOut returns the writer used for warnings and errors.
func (p *Printer) ErrOut() io.Writer { return p.err }

// Info prints an information message to the normal output in green.
//
// NOTE: When we executed gup update, the standard output became quite wide.
// To make the information more readable for the user, I removed the 'gup:INFO:' part.
func (p *Printer) Info(msg string) {
	_, _ = fmt.Fprintf(p.out, "%s\n", msg)
}

// Warn prints a warning message to the error output in yellow.
func (p *Printer) Warn(err interface{}) {
	_, _ = fmt.Fprintf(p.err, "%s:%s: %v\n",
		cmdinfo.Name, color.YellowString("WARN "), err)
}

// Err prints an error message to the error output in high-intensity yellow.
func (p *Printer) Err(err interface{}) {
	_, _ = fmt.Fprintf(p.err, "%s:%s: %v\n",
		cmdinfo.Name, color.HiYellowString("ERROR"), err)
}

// Hint prints a next-step suggestion to the error output in cyan. It is used to
// follow up an error with actionable guidance (e.g. which command to run next)
// so a failure is not just reported but explained.
func (p *Printer) Hint(msg string) {
	_, _ = fmt.Fprintf(p.err, "%s:%s : %v\n",
		cmdinfo.Name, color.CyanString("HINT"), msg)
}

// Fatal prints a dying message to the error output in red and then exits.
func (p *Printer) Fatal(err interface{}) {
	_, _ = fmt.Fprintf(p.err, "%s:%s: %v\n",
		cmdinfo.Name, color.RedString("FATAL"), err)
	p.exit(1)
}

// Question displays the question on the normal output and receives an answer
// from the user.
func (p *Printer) Question(ask string) bool {
	for {
		var response string

		_, _ = fmt.Fprintf(p.out, "%s:%s: %s",
			cmdinfo.Name, color.GreenString("CHECK"), ask+" [Y/n] ")
		_, err := p.scanln(&response)
		if err != nil {
			// If user input only enter, [Y/n] syntax is commonly used to denote that
			// "yes" is the default.
			// https://github.com/nao1215/gup/issues/146
			if strings.Contains(err.Error(), "expected newline") {
				return true
			}
			p.Err(err)
			return false
		}

		switch strings.ToLower(response) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
		}
	}
}
