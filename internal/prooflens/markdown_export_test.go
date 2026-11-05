package prooflens

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func exportTestRoot(t *testing.T, ignored bool) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "--quiet")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "evidence", "local"), 0755); err != nil {
		t.Fatal(err)
	}
	if ignored {
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("docs/evidence/local/\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestExportVerifiedMarkdownOnceNoOverwriteAndNoSecrets(t *testing.T) {
	correlation, history := reportFixture(t)
	report, err := BuildProofReport(correlation, history)
	if err != nil {
		t.Fatal(err)
	}
	root := exportTestRoot(t, true)
	relative, err := ExportVerifiedMarkdownOnce(root, report)
	if err != nil || relative != LocalMarkdownReport {
		t.Fatalf("first export rejected: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, token := range []string{"INCONCLUSIVE", "not a causal proof chain", "data-to-app", "edge-to-data"} {
		if !strings.Contains(strings.ToLower(content), strings.ToLower(token)) {
			t.Fatalf("missing %s", token)
		}
	}
	if strings.Contains(content, "terraform.tfstate") || strings.Contains(content, "v04-live-edge-to-data") {
		t.Fatal("sensitive source filename leaked")
	}
	if _, err := ExportVerifiedMarkdownOnce(root, report); err == nil {
		t.Fatal("overwrote existing report")
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil || string(after) != content {
		t.Fatal("existing evidence changed")
	}
}

func TestExportVerifiedMarkdownOnceFailsClosed(t *testing.T) {
	c, h := reportFixture(t)
	report, err := BuildProofReport(c, h)
	if err != nil {
		t.Fatal(err)
	}
	unsafe := report
	unsafe.Verdict = "supported"
	if _, err := ExportVerifiedMarkdownOnce(exportTestRoot(t, true), unsafe); err == nil {
		t.Fatal("exported forged verdict")
	}
	unignored := exportTestRoot(t, false)
	if _, err := ExportVerifiedMarkdownOnce(unignored, report); err == nil {
		t.Fatal("exported unignored report")
	}
	if _, err := os.Stat(filepath.Join(unignored, filepath.FromSlash(LocalMarkdownReport))); !os.IsNotExist(err) {
		t.Fatal("unignored output was created")
	}
	root := exportTestRoot(t, true)
	external := t.TempDir()
	local := filepath.Join(root, "docs", "evidence", "local")
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, local); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ExportVerifiedMarkdownOnce(root, report); err == nil {
		t.Fatal("followed evidence symlink")
	}
	if _, err := os.Stat(filepath.Join(external, "v05-proof-lens-report.md")); !os.IsNotExist(err) {
		t.Fatal("wrote outside repository")
	}
}
