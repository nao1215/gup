package print

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/cmdinfo"
)

// Info print information message at STDOUT.
func Info(msg string) {
	fmt.Fprintf(os.Stdout, "%s:%s: %s\n",
		cmdinfo.Name(), color.GreenString("INFO"), msg)
}

// Warn print warning message at STDERR.
func Warn(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.YellowString("WARN"), err)
}

// Err print error message at STDERR.
func Err(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.HiYellowString("ERROR"), err)
}

// Fatal print dying message at STDERR.
func Fatal(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s:%s: %v\n",
		cmdinfo.Name(), color.RedString("FATAL"), err)
	os.Exit(1)
}
