package print

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/gup/internal/cmdinfo"
)

var (
	stdout = colorable.NewColorableStdout()
	stderr = colorable.NewColorableStderr()
)

// Info print information message at STDOUT.
func Info(msg string) {
	fmt.Fprintf(stdout, "%s:%s: %s\n",
		cmdinfo.Name(), color.GreenString("INFO"), msg)
}

// Warn print warning message at STDERR.
func Warn(err interface{}) {
	fmt.Fprintf(stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.YellowString("WARN"), err)
}

// Err print error message at STDERR.
func Err(err interface{}) {
	fmt.Fprintf(stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.HiYellowString("ERROR"), err)
}

// Fatal print dying message at STDERR.
func Fatal(err interface{}) {
	fmt.Fprintf(stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.RedString("FATAL"), err)
	os.Exit(1)
}
