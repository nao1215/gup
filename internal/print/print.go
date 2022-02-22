package print

import (
	"fmt"
	"os"

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

// Info print information message at STDOUT.
func Info(msg string) {
	fmt.Fprintf(Stdout, "%s:%s: %s\n",
		cmdinfo.Name(), color.GreenString("INFO"), msg)
}

// Warn print warning message at STDERR.
func Warn(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.YellowString("WARN"), err)
}

// Err print error message at STDERR.
func Err(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.HiYellowString("ERROR"), err)
}

// Fatal print dying message at STDERR.
func Fatal(err interface{}) {
	fmt.Fprintf(Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.RedString("FATAL"), err)
	os.Exit(1)
}

// InstallResult print the result of "go install"
func InstallResult(result map[string]string) {
	for k, v := range result {
		if v == "Failure" {
			Err(fmt.Errorf("update failure: %s ", k))
		} else {
			Info("update success: " + k)
		}
	}
}
