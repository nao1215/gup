package main

import (
	"os"

	"github.com/nao1215/gup/cmd"
	"github.com/nao1215/gup/internal/print"
)

func main() {
	if err := cmd.Execute(); err != nil {
		print.NewColorable().Err(err)
		os.Exit(1)
	}
}
