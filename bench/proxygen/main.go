// Command proxygen writes a Go module proxy of synthetic command modules to a
// directory, for the himorime suites in bench/ to install and update from with
// GOPROXY=file://DIR and no network.
//
//	proxygen DIR N
//
// Module example.test/mNNNN is a main package published at v1.0.0 and v1.0.1,
// so a binary installed at v1.0.0 has an update. The output depends only on N.
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	wantArgs  = 3
	exitUsage = 2
	dirMode   = 0o750
	fileMode  = 0o600
)

func main() {
	if len(os.Args) != wantArgs {
		fmt.Fprintln(os.Stderr, "usage: proxygen DIR N")
		os.Exit(exitUsage)
	}
	n, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "proxygen:", err)
		os.Exit(exitUsage)
	}
	if err := write(os.Args[1], n); err != nil {
		fmt.Fprintln(os.Stderr, "proxygen:", err)
		os.Exit(1)
	}
}

// write publishes the modules under proxy. Every file is written through an
// os.Root of that directory, so nothing lands outside it.
func write(proxy string, n int) error {
	if err := os.MkdirAll(proxy, dirMode); err != nil { //nolint:gosec // G703: the directory the caller asked for on the command line
		return err
	}
	root, err := os.OpenRoot(proxy)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for i := range n {
		mod := fmt.Sprintf("example.test/m%04d", i)
		dir := filepath.Join(mod, "@v")
		if err := root.MkdirAll(dir, dirMode); err != nil {
			return err
		}
		if err := root.WriteFile(filepath.Join(dir, "list"), []byte("v1.0.0\nv1.0.1\n"), fileMode); err != nil {
			return err
		}
		gomod := "module " + mod + "\n\ngo 1.21\n"
		for _, v := range []string{"v1.0.0", "v1.0.1"} {
			info := `{"Version":"` + v + `","Time":"2020-01-01T00:00:00Z"}`
			if err := root.WriteFile(filepath.Join(dir, v+".info"), []byte(info), fileMode); err != nil {
				return err
			}
			if err := root.WriteFile(filepath.Join(dir, v+".mod"), []byte(gomod), fileMode); err != nil {
				return err
			}
			if err := writeZip(root, filepath.Join(dir, v+".zip"), mod, v, gomod); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeZip(root *os.Root, path, mod, version, gomod string) error {
	f, err := root.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	prefix := mod + "@" + version + "/"
	for name, body := range map[string]string{
		"go.mod":  gomod,
		"main.go": "package main\n\nfunc main() { _ = \"" + version + "\" }\n",
	} {
		w, err := zw.Create(prefix + name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}
