// Package print defines functions to accept colored standard output and user input
package print

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/gup/internal/cmdinfo"
)

var (
	// Stdout is new instance of Writer which handles escape sequence for stdout.
	Stdout = colorable.NewColorableStdout()
	// Stderr is new instance of Writer which handles escape sequence for stderr.
	Stderr = colorable.NewColorableStderr()
)

// Info print information message at STDOUT in green.
// This function is used to print some information (that is not error) to the user.
//
// NOTE: When we executed gup update, the standard output became quite wide.
// To make the information more readable for the user, I removed the 'gup:INFO:' part.
func Info(msg string) {
	fmt.Fprintf(Stdout, "%s\n", msg)
}

// Warn print warning message at STDERR in yellow.
// This function is used to print warning message to the user.
func Warn(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name, color.YellowString("WARN "), err)
}

// Err print error message at STDERR in yellow.
// This function is used to print error message to the user.
func Err(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name, color.HiYellowString("ERROR"), err)
}

// OsExit is wrapper for  os.Exit(). It's for unit test.
var OsExit = os.Exit

// Fatal print dying message at STDERR in red.
// After print message, process will exit
func Fatal(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name, color.RedString("FATAL"), err)
	OsExit(1)
}

// FmtScanln is wrapper for fmt.Scanln(). It's for unit test.
var FmtScanln = fmt.Scanln

// Question displays the question in the terminal and receives an answer from the user.
func Question(ask string) bool {
	var response string

	fmt.Fprintf(Stdout, "%s:%s: %s",
		cmdinfo.Name, color.GreenString("CHECK"), ask+" [Y/n] ")
	_, err := FmtScanln(&response)
	if err != nil {
		// If user input only enter, [Y/n] syntax is commonly used to denote that
		// "yes" is the default.
		// https://github.com/nao1215/gup/issues/146
		if strings.Contains(err.Error(), "expected newline") {
			return true
		}
		fmt.Fprint(os.Stderr, err.Error())
		return false
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return Question(ask)
	}
}
