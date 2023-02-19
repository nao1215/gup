package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/nao1215/gup/internal/completion"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var updateGolangCmd = &cobra.Command{
	Use:   "update-go",
	Short: "Install or Update golang itself under /usr/local/go.",
	Long: `Install or Update golang itself under /usr/local/go.

update-go subcommand update golang if golang installed in /usr/local/go
is not up-to-date. If golang is not on the system, gup will not install
the latest version of golang in /usr/local/go.

update-go subcommand is an experimental feature. In the future, update-go
may be removed or become a another command.
`,
	Example: "  sudo gup update-go",
	Run: func(cmd *cobra.Command, args []string) {
		OsExit(updateGolang(cmd, args))
	},
}

var latestGoVersion = "1.20.1"

func init() {
	// Not support windows.
	if !completion.IsWindows() {
		rootCmd.AddCommand(updateGolangCmd)
	}
}

var errNoNeedToUpdateGo = errors.New("no need to update golang")

// updateGolang update /usr/local/go.
func updateGolang(cmd *cobra.Command, args []string) int {
	if err := compareCurrentVerAndLatestVer(); err != nil {
		if errors.Is(err, errNoNeedToUpdateGo) {
			print.Info(fmt.Sprintf("current go version is equal to or newer than version %s", latestGoVersion))
			return 0
		}
		print.Err(fmt.Errorf("%s: %w", "go version check error", err))
		return 1
	}

	root, err := hasRootPrivirage()
	if err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not get user information", err))
		return 1
	}
	if !root {
		print.Err("you must have root privileges to run update-go")
		return 1
	}

	print.Info(fmt.Sprintf("download %s at current directory", tarballName()))
	if err := fetchGolangTarball(tarballName()); err != nil {
		print.Err(fmt.Errorf("%s %s: %w", "can not download", tarballName(), err))
		return 1
	}

	if err := compareChecksum(tarballName()); err != nil {
		print.Err(fmt.Errorf("%s: %w", "failed to compare checksum", err))
		return 1
	}

	print.Info("backup original /usr/local/go as /usr/local/go.backup")
	if err := renameIfDirExists("/usr/local/go", "/usr/local/go.backup"); err != nil {
		print.Err(fmt.Errorf("%s: %w", "failed to backup old /usr/local/go", err))
		return 1
	}

	print.Info(fmt.Sprintf("start extract %s at %s", tarballName(), "/usr/local/go"))
	if err := extractTarball(tarballName(), "/usr/local"); err != nil {
		print.Warn(fmt.Sprintf("failed to extract %s", tarballName()))
		print.Info("start restore /usr/local/go from backup")
		if err := recovery("/usr/local/go", "/usr/local/go.backup"); err != nil {
			print.Err(fmt.Errorf("%s: %w", "!!! failed to restore !!! golang may not be available", err))
			return 1
		}
		print.Info("success to restore /usr/local/go from backup")
		return 1
	}

	print.Info(fmt.Sprintf("delete backup (%s)", "/usr/local/go.backup"))
	if err := os.RemoveAll("/usr/local/go.backup"); err != nil {
		print.Err(fmt.Errorf("%s %s: %w", "failed to delete", "/usr/local/go.backup", err))
		return 1
	}

	print.Info(fmt.Sprintf("delete %s", tarballName()))
	if err := os.RemoveAll(tarballName()); err != nil {
		print.Err(fmt.Errorf("%s %s: %w", "failed to delete", tarballName(), err))
		return 1
	}

	print.Info(fmt.Sprintf("success to update golang (version %s)", latestGoVersion))
	return 0
}

func compareCurrentVerAndLatestVer() error {
	if _, err := exec.LookPath("/usr/local/go/bin/go"); err != nil {
		return nil // this system does not install golang. So, install it.
	}

	currentVer, err := getCurrentGoSemanticVer()
	if err != nil {
		return err
	}

	current, err := semver.NewVersion(currentVer)
	if err != nil {
		return err
	}

	latest, err := semver.NewVersion(latestGoVersion)
	if err != nil {
		return err
	}

	print.Info(fmt.Sprintf("current=%s, latest=%s", currentVer, latestGoVersion))
	if current.Equal(latest) || current.GreaterThan(latest) {
		return errNoNeedToUpdateGo
	}
	return nil
}

func getCurrentGoSemanticVer() (string, error) {
	cmd := exec.Command("/usr/local/go/bin/go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// extract version (e.g. go1.2.1)
	verStr := strings.TrimSpace(string(bytes.Split(output, []byte(" "))[2]))
	return strings.Replace(verStr, "go", "", 1), nil
}

func hasRootPrivirage() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}
	if u.Uid == "0" {
		return true, nil
	}
	return false, nil
}

func tarballName() string {
	return fmt.Sprintf("go%s.%s-%s.tar.gz", latestGoVersion, runtime.GOOS, runtime.GOARCH)
}

// golangTarballChecksums return key=taraball name , value=sha256 checksum
func golangTarballChecksums() map[string]string {
	return map[string]string{
		"go1.20.1.darwin-amd64.tar.gz":  "a300a45e801ab459f3008aae5bb9efbe9a6de9bcd12388f5ca9bbd14f70236de",
		"go1.20.1.darwin-arm64.tar.gz":  "f1a8e06c7f1ba1c008313577f3f58132eb166a41ceb95ce6e9af30bc5a3efca4",
		"go1.20.1.linux-386.tar.gz":     "3a7345036ebd92455b653e4b4f6aaf4f7e1f91f4ced33b23d7059159cec5f4d7",
		"go1.20.1.linux-amd64.tar.gz":   "000a5b1fca4f75895f78befeb2eecf10bfff3c428597f3f1e69133b63b911b02",
		"go1.20.1.linux-arm64.tar.gz":   "5e5e2926733595e6f3c5b5ad1089afac11c1490351855e87849d0e7702b1ec2e",
		"go1.20.1.linux-armv6l.tar.gz":  "e4edc05558ab3657ba3dddb909209463cee38df9c1996893dd08cde274915003",
		"go1.20.1.freebsd-386.tar.gz":   "57d80349dc4fbf692f8cd85a5971f97841aedafcf211e367e59d3ae812292660",
		"go1.20.1.freebsd-amd64.tar.gz": "6e124d54d5850a15fdb15754f782986f06af23c5ddb6690849417b9c74f05f98",
		"go1.20.1.linux-ppc64le.tar.gz": "85cfd4b89b48c94030783b6e9e619e35557862358b846064636361421d0b0c52",
		"go1.20.1.linux-s390x.tar.gz":   "ba3a14381ed4538216dec3ea72b35731750597edd851cece1eb120edf7d60149",
	}
}

// fetchGolangTarball download latest golang
func fetchGolangTarball(tarballName string) error {
	url := fmt.Sprintf("https://go.dev/dl/%s", tarballName)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tarball, err := os.Create(tarballName)
	if err != nil {
		return err
	}
	defer tarball.Close()

	_, err = io.Copy(tarball, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// compareChecksum compare the "sha256 checksum of the downloaded tarball" with the "expected value"
func compareChecksum(tarballName string) error {
	checksumMap := golangTarballChecksums()
	expectSha256, ok := checksumMap[tarballName]
	if !ok {
		return errors.New("checksum (expected value) of downloaded go file not found")
	}

	data, err := os.ReadFile(tarballName)
	if err != nil {
		return err
	}
	sha256checksum := sha256.Sum256(data)
	gotSha256 := fmt.Sprintf("%x", sha256checksum)

	print.Info("[compare sha256 checksum]")
	print.Info(fmt.Sprintf(" expect: %s", expectSha256))
	print.Info(fmt.Sprintf(" got   : %s", gotSha256))

	if expectSha256 != gotSha256 {
		return errors.New("sha256 checksum does not match")
	}
	return nil
}

// renameOldGoDir rename /usr/local/go to /usr/local/go.backup
func renameIfDirExists(oldDir, newDir string) error {
	if file.IsDir(oldDir) {
		if err := os.Rename(oldDir, newDir); err != nil {
			return err
		}
	}
	return nil
}

// extractTarball extract tarball
func extractTarball(tarballPath, targetPath string) error {
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // end of extract
		}
		if err != nil {
			return err
		}

		target := filepath.Join(targetPath, header.Name)
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		createFile := func() error {
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(file, tarReader); err != nil {
				return err
			}
			return nil
		}
		if err := createFile(); err != nil {
			return err
		}
	}
	return nil
}

// recovery restore /usr/local/go from backup if update fails
func recovery(targetPath, backupPath string) error {
	if file.IsDir(targetPath) {
		if err := os.RemoveAll(targetPath); err != nil {
			return err
		}
	}

	if err := renameIfDirExists(backupPath, targetPath); err != nil {
		return err
	}
	return nil
}
