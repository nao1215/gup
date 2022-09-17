// Package cmd define subcommands provided by the gup command
package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "success",
			args: []string{""},
		},
		{
			name: "fail",
			args: []string{"no-exist-subcommand", "--no-exist-option"},
		},
	}
	for _, tt := range tests {
		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})
	}
}

func TestExecute_Check(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout []string
	}{
		{
			name:   "success",
			args:   []string{"gup", "check"},
			stdout: []string{"", ""},
		},
	}
	for _, tt := range tests {
		oldGoBin := os.Getenv("GOBIN")
		if err := os.Setenv("GOBIN", "./testdata/check_success"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Setenv("GOBIN", oldGoBin); err != nil {
				t.Fatal(err)
			}
		}()

		orgStdout := print.Stdout
		orgStderr := print.Stderr
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		print.Stdout = pw
		print.Stderr = pw

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		os.Args = tt.args

		Execute()
		pw.Close()
		print.Stdout = orgStdout
		print.Stderr = orgStderr

		buf := bytes.Buffer{}
		_, err = io.Copy(&buf, pr)
		if err != nil {
			t.Error(err)
		}
		defer pr.Close()
		got := strings.Split(buf.String(), "\n")

		if !strings.Contains(got[len(got)-2], "subaru") {
			t.Errorf("subaru package is not included in the update target: %s", got[len(got)-2])
		}
		if !strings.Contains(got[len(got)-2], "posixer") {
			t.Errorf("posixer package is not included in the update target: %s", got[len(got)-2])
		}
		if !strings.Contains(got[len(got)-2], "gal") {
			t.Errorf("gal package is not included in the update target: %s", got[len(got)-2])
		}
	}
}

func TestExecute_Version(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout []string
	}{
		{
			name:   "success",
			args:   []string{"gup", "version"},
			stdout: []string{"gup version  (under Apache License version 2.0)", ""},
		},
	}
	for _, tt := range tests {
		orgStdout := os.Stdout
		orgStderr := os.Stderr
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = pw
		os.Stderr = pw

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})
		pw.Close()
		os.Stdout = orgStdout
		os.Stderr = orgStderr

		buf := bytes.Buffer{}
		_, err = io.Copy(&buf, pr)
		if err != nil {
			t.Error(err)
		}
		defer pr.Close()
		got := strings.Split(buf.String(), "\n")

		if diff := cmp.Diff(tt.stdout, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestExecute_List(t *testing.T) {
	tests := []struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}{
		{
			name:  "success",
			gobin: "./testdata/check_success",
			args:  []string{"gup", "list"},
			stdout: []string{
				"    gal: github.com/nao1215/gal/cmd/gal@v1.1.1",
				"posixer: github.com/nao1215/posixer@v0.1.0",
				" subaru: github.com/nao1215/subaru@v1.0.0",
			},
		},
	}
	for _, tt := range tests {
		oldGoBin := os.Getenv("GOBIN")
		if err := os.Setenv("GOBIN", tt.gobin); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Setenv("GOBIN", oldGoBin); err != nil {
				t.Fatal(err)
			}
		}()

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		orgStdout := print.Stdout
		orgStderr := print.Stderr
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		print.Stdout = pw
		print.Stderr = pw

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})
		pw.Close()
		print.Stdout = orgStdout
		print.Stderr = orgStderr

		buf := bytes.Buffer{}
		_, err = io.Copy(&buf, pr)
		if err != nil {
			t.Error(err)
		}
		defer pr.Close()
		got := strings.Split(buf.String(), "\n")

		count := 0
		for _, g := range got {
			for _, w := range tt.stdout {
				if g == w {
					count++
				}
			}
		}
		if count != 3 {
			t.Errorf("value is mismatch. want=3 got=%d", count)
		}
	}
}

func TestExecute_Remove_Force(t *testing.T) {
	tests := []struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}{
		{
			name:   "success",
			gobin:  "./testdata/delete",
			args:   []string{"gup", "remove", "-f", "posixer"},
			stdout: []string{},
		},
	}

	if err := os.MkdirAll("./testdata/delete", 0755); err != nil {
		t.Fatal(err)
	}

	newFile, err := os.Create("./testdata/delete/posixer")
	if err != nil {
		t.Fatal(err)
	}

	oldFile, err := os.Open("./testdata/check_success/posixer")
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.Copy(newFile, oldFile)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		oldGoBin := os.Getenv("GOBIN")
		if err := os.Setenv("GOBIN", tt.gobin); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Setenv("GOBIN", oldGoBin); err != nil {
				t.Fatal(err)
			}
		}()

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})

		if file.IsFile("./testdata/delete/posixer") {
			t.Errorf("failed to remove posixer command")
		}
	}

	err = os.Remove("./testdata/delete")
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecute_Export(t *testing.T) {
	tests := []struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}{
		{
			name:   "success",
			gobin:  "./testdata/check_success",
			args:   []string{"gup", "export"},
			stdout: []string{},
		},
	}

	for _, tt := range tests {
		oldGoBin := os.Getenv("GOBIN")
		if err := os.Setenv("GOBIN", tt.gobin); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Setenv("GOBIN", oldGoBin); err != nil {
				t.Fatal(err)
			}
		}()

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		doBackup := false
		if file.IsFile(config.FilePath()) {
			if err := os.Rename(config.FilePath(), config.FilePath()+".backup"); err != nil {
				t.Fatal(err)
			}
			doBackup = true
		}
		defer func() {
			if doBackup {
				if err := os.Rename(config.FilePath()+".backup", config.FilePath()); err != nil {
					t.Fatal(err)
				}
			}
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})

		if !file.IsFile(config.FilePath()) {
			t.Errorf(config.FilePath() + " does not exist. failed to generate")
			continue
		}

		got, err := os.ReadFile(config.FilePath())
		if err != nil {
			t.Fatal(err)
		}

		want := `gal = github.com/nao1215/gal/cmd/gal
posixer = github.com/nao1215/posixer
subaru = github.com/nao1215/subaru
`
		if string(got) != want {
			t.Errorf("mismatch: want=%s, got=%s", want, string(got))
		}
	}
}

func TestExecute_Import(t *testing.T) {
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", "./testdata"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatal(err)
		}
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}
	if file.IsFile(filepath.Join(gobin, "posixer")) {
		if err := os.Rename(filepath.Join(gobin, "posixer"), filepath.Join(gobin, "posixer")+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Rename(filepath.Join(gobin, "posixer")+".backup", filepath.Join(gobin, "posixer")); err != nil {
				t.Fatal(err)
			}
		}()
	}

	if file.IsFile(filepath.Join(gobin, "subaru")) {
		if err := os.Rename(filepath.Join(gobin, "subaru"), filepath.Join(gobin, "subaru")+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Rename(filepath.Join(gobin, "subaru")+".backup", filepath.Join(gobin, "subaru")); err != nil {
				t.Fatal(err)
			}
		}()
	}

	if file.IsFile(filepath.Join(gobin, "gal")) {
		if err := os.Rename(filepath.Join(gobin, "gal"), filepath.Join(gobin, "gal")+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Rename(filepath.Join(gobin, "gal")+".backup", filepath.Join(gobin, "gal")); err != nil {
				t.Fatal(err)
			}
		}()
	}

	defer func() {
		os.RemoveAll("./testdata/.config/fish")
		os.RemoveAll("./testdata/.zsh")
		os.RemoveAll("./testdata/.zshrc")
		os.RemoveAll("./testdata/.bash_completion")
		os.RemoveAll("./testdata/.config/gup/assets")
		os.RemoveAll("./testdata/go")
		os.RemoveAll("./testdata/.cache")
	}()

	os.Args = []string{"gup", "import"}
	Execute()

	if !file.IsFile("./testdata/.zsh/completion/_gup") {
		t.Errorf("not install .zsh/completion/_gup")
	}

	if !file.IsFile("./testdata/.config/fish/completions/gup.fish") {
		t.Errorf("not install .config/fish/completions/gup.fish")
	}

	if !file.IsFile("./testdata/.bash_completion") {
		t.Errorf("not install .bash_completion")
	}

	if !file.IsFile("./testdata/.zshrc") {
		t.Errorf("not install .bash_completion")
	}

	if !file.IsFile("./testdata/.config/gup/assets/information.png") {
		t.Errorf("not install .config/gup/assets/information.png")
	}

	if !file.IsFile("./testdata/.config/gup/assets/warning.png") {
		t.Errorf("not install .config/gup/assets/warning.png")
	}

	/*
		if !file.IsFile(filepath.Join(gobin, "gal")) {
			t.Errorf("not import gal command: %s", filepath.Join(gobin, "gal"))
		}

		if !file.IsFile(filepath.Join(gobin, "posixer")) {
			t.Errorf("not import posixer command: %s", filepath.Join(gobin, "posixer"))
		}

		if !file.IsFile(filepath.Join(gobin, "subaru")) {
			t.Errorf("not import subaru command: %s", filepath.Join(gobin, "subaru"))
		}
	*/
}
