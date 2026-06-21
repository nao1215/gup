//nolint:paralleltest // tests mutate global function variables for stubbing
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

const (
	testBinMainTool   = "maintool"
	testBinMasterTool = "mastertool"
	testBinLatestTool = "latesttool"
	testVerMaster     = "v0.0.0-master"

	// refMain and refMaster are the go toolchain version selectors
	// fetchVerForChannel passes to "go list" for the @main and @master channels.
	refMain   = string(goutil.UpdateChannelMain)
	refMaster = string(goutil.UpdateChannelMaster)
)

// newCheckPkg builds a package fixture for doCheck channel tests. The Go
// toolchain version is pinned equal to itself so only the package version drives
// the "needs update" decision.
func newCheckPkg(name, current string, channel goutil.UpdateChannel) goutil.Package {
	return goutil.Package{
		Name:          name,
		ImportPath:    "example.com/" + name + "/cmd/" + name,
		ModulePath:    "example.com/" + name,
		Version:       &goutil.Version{Current: current},
		GoVersion:     &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		UpdateChannel: channel,
	}
}

// captureCheckOutput runs fn while capturing everything printed to stdout/stderr.
func captureCheckOutput(t *testing.T, fn func() int) string {
	t.Helper()
	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	fn()

	_ = pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()
	return buf.String()
}

func Test_fetchVerForChannel(t *testing.T) {
	origLatest := getLatestVerCtx
	origRef := getVerByRefCtx
	t.Cleanup(func() {
		getLatestVerCtx = origLatest
		getVerByRefCtx = origRef
	})

	getLatestVerCtx = func(_ context.Context, _ string) (string, error) {
		return "v1.0.0-latest", nil
	}
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		switch ref {
		case refMain:
			return "", errors.New("unknown revision main")
		case refMaster:
			return testVerMaster, nil
		default:
			return "", fmt.Errorf("unexpected ref %q", ref)
		}
	}

	tests := []struct {
		name    string
		channel goutil.UpdateChannel
		want    string
	}{
		{"latest channel uses go list @latest", goutil.UpdateChannelLatest, "v1.0.0-latest"},
		{"master channel uses go list @master", goutil.UpdateChannelMaster, testVerMaster},
		{"main channel falls back to @master when @main is missing", goutil.UpdateChannelMain, testVerMaster},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchVerForChannel(context.Background(), "example.com/mod", tt.channel)
			if err != nil {
				t.Fatalf("fetchVerForChannel() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("fetchVerForChannel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_fetchVerForChannel_mainNoFallbackOnContextError(t *testing.T) {
	origRef := getVerByRefCtx
	t.Cleanup(func() { getVerByRefCtx = origRef })

	var refCalls []string
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		refCalls = append(refCalls, ref)
		return "", context.Canceled
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := fetchVerForChannel(ctx, "example.com/mod", goutil.UpdateChannelMain); err == nil {
		t.Fatal("fetchVerForChannel() should return error on context cancellation")
	}
	if len(refCalls) != 1 || refCalls[0] != refMain {
		t.Fatalf("expected only the @main attempt on context cancellation, got %v", refCalls)
	}
}

func Test_doCheck_respectsSavedChannels(t *testing.T) {
	origLatest := getLatestVerCtx
	origRef := getVerByRefCtx
	t.Cleanup(func() {
		getLatestVerCtx = origLatest
		getVerByRefCtx = origRef
	})

	// @latest always reports v1.0.0 so a main/master binary would look
	// up-to-date if check wrongly ignored the saved channel.
	getLatestVerCtx = func(_ context.Context, _ string) (string, error) {
		return testVersionOne, nil
	}
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		switch ref {
		case refMain:
			return "v1.5.0", nil
		case refMaster:
			return testVersionTwo, nil
		default:
			return "", fmt.Errorf("unexpected ref %q", ref)
		}
	}

	pkgs := []goutil.Package{
		// v1.0.0 == @latest v1.0.0 -> up to date
		newCheckPkg(testBinLatestTool, testVersionOne, goutil.UpdateChannelLatest),
		// v1.0.0 < @main v1.5.0 -> needs update
		newCheckPkg(testBinMainTool, testVersionOne, goutil.UpdateChannelMain),
		// v2.0.0 == @master v2.0.0 -> up to date
		newCheckPkg(testBinMasterTool, testVersionTwo, goutil.UpdateChannelMaster),
	}

	out := captureCheckOutput(t, func() int {
		return doCheck(pkgs, 1, 0, true)
	})

	idx := strings.Index(out, "$ gup update ")
	if idx < 0 {
		t.Fatalf("expected an update hint, got:\n%s", out)
	}
	hint := out[idx:]
	if !strings.Contains(hint, testBinMainTool) {
		t.Fatalf("expected %s in update hint (outdated against @main), got:\n%s", testBinMainTool, hint)
	}
	if strings.Contains(hint, testBinLatestTool) {
		t.Fatalf("%s should be up-to-date against @latest, got hint:\n%s", testBinLatestTool, hint)
	}
	if strings.Contains(hint, testBinMasterTool) {
		t.Fatalf("%s should be up-to-date against @master, got hint:\n%s", testBinMasterTool, hint)
	}
}

func Test_applySavedChannels(t *testing.T) {
	confPkgs := []goutil.Package{
		{Name: testBinMainTool, UpdateChannel: goutil.UpdateChannelMain},
		{Name: testBinMasterTool, UpdateChannel: goutil.UpdateChannelMaster},
		{Name: testBinLatestTool, UpdateChannel: goutil.UpdateChannelLatest},
	}
	pkgs := []goutil.Package{
		{Name: testBinMainTool},
		{Name: testBinMasterTool},
		{Name: testBinLatestTool},
		{Name: "unconfigured"}, // not present in config -> defaults to latest
	}

	got := applySavedChannels(pkgs, confPkgs)

	want := map[string]goutil.UpdateChannel{
		testBinMainTool:   goutil.UpdateChannelMain,
		testBinMasterTool: goutil.UpdateChannelMaster,
		testBinLatestTool: goutil.UpdateChannelLatest,
		"unconfigured":    goutil.UpdateChannelLatest,
	}
	for _, p := range got {
		if p.UpdateChannel != want[p.Name] {
			t.Errorf("applySavedChannels() %s channel = %q, want %q", p.Name, p.UpdateChannel, want[p.Name])
		}
	}
}
