// Package cmd define subcommands provided by the gup command
package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/cmdinfo"
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
	gobinDir := ""
	if runtime.GOOS == "windows" {
		gobinDir = filepath.Join("testdata", "check_success_for_windows")
	} else {
		gobinDir = filepath.Join("testdata", "check_success")
	}
	t.Setenv("GOBIN", gobinDir)

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

	os.Args = []string{"gup", "check"}
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

	if !strings.Contains(got[len(got)-2], "posixer") {
		t.Errorf("posixer package is not included in the update target: %s", got[len(got)-2])
	}
	if !strings.Contains(got[len(got)-2], "gal") {
		t.Errorf("gal package is not included in the update target: %s", got[len(got)-2])
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
	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
		want   int
	}
	tests := []test{}

	if runtime.GOOS == "windows" {
		tests = append(tests, test{
			name:  "check success in windows",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "list"},
			stdout: []string{
				"github.com/nao1215/gal/cmd/gal",
				"github.com/nao1215/posixer",
			},
			want: 2,
		})
	} else {
		tests = append(tests, test{
			name:  "check success in nix family",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "list"},
			stdout: []string{
				"    gal: github.com/nao1215/gal/cmd/gal@v1.1.1",
				"posixer: github.com/nao1215/posixer@v0.1.0",
				" subaru: github.com/nao1215/subaru@v1.0.0",
			},
			want: 3,
		})
	}
	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

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
				if strings.Contains(g, w) {
					count++
				}
			}
		}
		if count != tt.want {
			t.Errorf("value is mismatch. want=%d got=%d", tt.want, count)
		}
	}
}

func TestExecute_Remove_Force(t *testing.T) {
	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}
	tests := []test{}

	src := ""
	dest := ""
	if runtime.GOOS == "windows" {
		tests = append(tests, test{
			name:   "success",
			gobin:  filepath.Join("testdata", "delete_force"),
			args:   []string{"gup", "remove", "-f", "posixer.exe"},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete_force"), 0755); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete_force"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success_for_windows", "posixer.exe")
		dest = filepath.Join("testdata", "delete_force", "posixer.exe")
	} else {
		tests = append(tests, test{
			name:   "success",
			gobin:  filepath.Join("testdata", "delete"),
			args:   []string{"gup", "remove", "-f", "posixer"},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete"), 0755); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success", "posixer")
		dest = filepath.Join("testdata", "delete", "posixer")
	}

	newFile, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}

	oldFile, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.Copy(newFile, oldFile)
	if err != nil {
		t.Fatal(err)
	}
	oldFile.Close()
	newFile.Close()

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			Execute()
		})

		if file.IsFile(filepath.Join(dest)) {
			t.Errorf("failed to remove posixer command")
		}
	}
}

func TestExecute_Export(t *testing.T) {
	tests := []struct {
		name  string
		gobin string
		args  []string
		want  string
	}{}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "export"},
			want: `gal.exe = github.com/nao1215/gal/cmd/gal
posixer.exe = github.com/nao1215/posixer
`,
		})
	} else {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "export"},
			want: `gal = github.com/nao1215/gal/cmd/gal
posixer = github.com/nao1215/posixer
subaru = github.com/nao1215/subaru
`,
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

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

		if string(got) != tt.want {
			t.Errorf("mismatch: want=%s, got=%s", tt.want, string(got))
		}
	}
}

func TestExecute_Export_WithOutputOption(t *testing.T) {
	type test struct {
		name  string
		gobin string
		args  []string
		want  []string
	}

	tests := []test{}
	if runtime.GOOS == "windows" {
		tests = append(tests, test{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "export", "--output"},
			want: []string{
				"gal.exe = github.com/nao1215/gal/cmd/gal",
				"posixer.exe = github.com/nao1215/posixer",
				""},
		})
	} else {
		tests = append(tests, test{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "export", "--output"},
			want: []string{
				"gal = github.com/nao1215/gal/cmd/gal",
				"posixer = github.com/nao1215/posixer",
				"subaru = github.com/nao1215/subaru",
				""},
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

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

		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestExecute_Import_WithInputOption(t *testing.T) {
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}
	if file.IsDir(gobin) {
		if err := os.Rename(gobin, gobin+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if file.IsDir(gobin + ".backup") {
				os.RemoveAll(gobin)
				if err := os.Rename(gobin+".backup", gobin); err != nil {
					t.Fatal(err)
				}
			}
		}()

		if err := os.MkdirAll(gobin, 0755); err != nil {
			t.Fatal(err)
		}
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	confFile := "testdata/gup_config/nix.conf"
	if runtime.GOOS == "windows" {
		confFile = "testdata/gup_config/windows.conf"
	}
	os.Args = []string{"gup", "import", "-i", confFile}
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

	contain := false
	for _, v := range got {
		if strings.Contains(v, "gup:INFO : [1/1] github.com/nao1215/gup") {
			contain = true
		}
	}

	if !contain {
		t.Error("failed import")
	}
}

func TestExecute_Import_WithBadInputFile(t *testing.T) {
	tests := []struct {
		name      string
		inputFile string
		want      []string
	}{
		{
			name:      "specify not exist file",
			inputFile: "not_exist_file_path",
			want: []string{
				"gup:ERROR: not_exist_file_path is not found",
				"",
			},
		},
		{
			name:      "specify empty file",
			inputFile: "testdata/gup_config/empty.conf",
			want: []string{
				"gup:ERROR: unable to import package: no package information",
				"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			os.Args = []string{"gup", "import", "-i", tt.inputFile}
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

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExecute_Update(t *testing.T) {
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}
	if file.IsDir(gobin) {
		if err := os.Rename(gobin, gobin+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if file.IsDir(gobin + ".backup") {
				os.RemoveAll(gobin)
				if err := os.Rename(gobin+".backup", gobin); err != nil {
					t.Fatal(err)
				}
			}
		}()

		if err := os.MkdirAll(gobin, 0755); err != nil {
			t.Fatal(err)
		}

		targetPath := ""
		binName := ""
		if runtime.GOOS == "windows" {
			binName = "gal.exe"
			targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
		} else {
			binName = "gal"
			targetPath = filepath.Join("testdata", "check_success", binName)
		}
		in, err := os.Open(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()

		out, err := os.Create(filepath.Join(gobin, binName))
		if err != nil {
			t.Fatal(err)
		}

		_, err = io.Copy(out, in)
		if err != nil {
			t.Fatal(err)
		}
		in.Close()
		out.Close()

		if err = os.Chmod(filepath.Join(gobin, binName), 0777); err != nil {
			t.Fatal(err)
		}
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	os.Args = []string{"gup", "update"}
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

	contain := false
	for _, v := range got {
		if strings.Contains(v, "gup:INFO : [1/1] github.com/nao1215/gal/cmd/gal") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update gal command")
	}
}

func TestExecute_Update_DryRunAndNotify(t *testing.T) {
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}
	if file.IsDir(gobin) {
		if err := os.Rename(gobin, gobin+".backup"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if file.IsDir(gobin + ".backup") {
				os.RemoveAll(gobin)
				if err := os.Rename(gobin+".backup", gobin); err != nil {
					t.Fatal(err)
				}
			}
		}()

		if err := os.MkdirAll(gobin, 0755); err != nil {
			t.Fatal(err)
		}

		targetPath := ""
		binName := ""
		if runtime.GOOS == "windows" {
			binName = "posixer.exe"
			targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
		} else {
			binName = "posixer"
			targetPath = filepath.Join("testdata", "check_success", binName)
		}
		in, err := os.Open(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()

		out, err := os.Create(filepath.Join(gobin, binName))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			t.Fatal(err)
		}
		in.Close()
		out.Close()

		if err = os.Chmod(filepath.Join(gobin, binName), 0777); err != nil {
			t.Fatal(err)
		}
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	os.Args = []string{"gup", "update", "--dry-run", "--notify"}
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

	contain := false
	for _, v := range got {
		if strings.Contains(v, "gup:INFO : [1/1] github.com/nao1215/posixer") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update posixer command")
	}
}

func TestExecute_Completion(t *testing.T) {
	t.Run("generate completion file", func(t *testing.T) {
		os.Args = []string{"gup", "completion"}
		Execute()

		bash := filepath.Join(os.Getenv("HOME"), ".bash_completion")
		if runtime.GOOS == "windows" {
			if file.IsFile(bash) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", bash)
			}
		} else {
			if !file.IsFile(bash) {
				t.Errorf("failed to generate %s", bash)
			}
		}

		fish := filepath.Join(os.Getenv("HOME"), ".config", "fish", "completions", cmdinfo.Name+".fish")
		if runtime.GOOS == "windows" {
			if file.IsFile(fish) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", fish)
			}
		} else {
			if !file.IsFile(fish) {
				t.Errorf("failed to generate %s", fish)
			}
		}

		zsh := filepath.Join(os.Getenv("HOME"), ".zsh", "completion", "_"+cmdinfo.Name)
		if runtime.GOOS == "windows" {
			if file.IsFile(zsh) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", zsh)
			}
		} else {
			if !file.IsFile(zsh) {
				t.Errorf("failed to generate  %s", zsh)
			}
		}
	})
}
