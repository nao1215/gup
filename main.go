package main

import (
	"github.com/nao1215/gup/cmd"
	"github.com/nao1215/gup/internal/print"
)

func main() {
	if err := cmd.Execute(); err != nil {
		print.Err(err)
	}
}
