package completion

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestHasSameBashCompletionContent_ExactMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := testCompletionCmd()

	content := generateBashCompletion(t, cmd)
	writeBashCompletionFile(t, content)

	if !isSameBashCompletionFile(cmd) {
		t.Fatal("isSameBashCompletionFile() = false, want true")
	}
}

func TestHasSameBashCompletionContent_DifferentButContains(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := testCompletionCmd()

	content := generateBashCompletion(t, cmd)
	writeBashCompletionFile(t, append([]byte("# stale header\n"), content...))

	if isSameBashCompletionFile(cmd) {
		t.Fatal("isSameBashCompletionFile() = true, want false")
	}
}

func testCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "gup"}
	cmd.PersistentFlags().Bool("verbose", false, "verbose output")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "update tools",
		Annotations: map[string]string{
			"group": "maintenance",
		},
	}
	updateCmd.Flags().String("channel", "latest", "update channel")

	cmd.AddCommand(updateCmd)
	return cmd
}

func generateBashCompletion(t *testing.T, cmd *cobra.Command) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	if err := cmd.GenBashCompletionV2(buf, false); err != nil {
		t.Fatalf("GenBashCompletionV2() error = %v", err)
	}
	return buf.Bytes()
}

func writeBashCompletionFile(t *testing.T, content []byte) {
	t.Helper()
	path := bashCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
