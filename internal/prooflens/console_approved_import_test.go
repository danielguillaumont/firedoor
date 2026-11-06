package prooflens

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func approvedConsoleTempRepo(t *testing.T, branch string) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", branch, root)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("Git init unavailable: %v (%s)", err, string(b))
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/docs/evidence/local/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "evidence", "local"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestApprovedConsoleImportValidDetachedReportAndFailClosed(t *testing.T) {
	correlation, history := reportFixture(t)
	report, err := BuildProofReport(correlation, history)
	if err != nil {
		t.Fatal(err)
	}
	root := approvedConsoleTempRepo(t, "feat/v06-console")
	rel := filepath.Join(root, filepath.FromSlash(ConsoleReportRelativePath))
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rel, b, 0600); err != nil {
		t.Fatal(err)
	}
	summary, err := LoadApprovedConsoleProof(root, ConsoleApprovalConsent)
	if err != nil {
		t.Fatalf("valid detached report rejected: %v", err)
	}
	if summary.Verdict != Inconclusive || summary.Terraform.Applied || summary.Kubernetes.CurrentRead || summary.ProofChainEstablished || summary.IndependentlyAttested {
		t.Fatal("import overclaimed evidence")
	}
	if _, err := LoadApprovedConsoleProof(root, "wrong-consent"); err == nil {
		t.Fatal("invalid consent accepted")
	}
	if err := os.WriteFile(rel, append(b, []byte(` {"another":"document"}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApprovedConsoleProof(root, ConsoleApprovalConsent); err == nil {
		t.Fatal("multiple documents accepted")
	}
	if err := os.WriteFile(rel, append(b, []byte(strings.Repeat(" ", 129*1024))...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApprovedConsoleProof(root, ConsoleApprovalConsent); err == nil {
		t.Fatal("oversized report accepted")
	}
	if err := os.Remove(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApprovedConsoleProof(root, ConsoleApprovalConsent); err == nil {
		t.Fatal("missing report accepted")
	}
}

func TestApprovedConsoleImportRejectsOutsideRepoOrUnapprovedBranch(t *testing.T) {
	root := approvedConsoleTempRepo(t, "feature/arbitrary")
	if _, err := LoadApprovedConsoleProof(root, ConsoleApprovalConsent); err == nil {
		t.Fatal("unapproved branch accepted")
	}
	if _, err := LoadApprovedConsoleProof(t.TempDir(), ConsoleApprovalConsent); err == nil {
		t.Fatal("non-repository accepted")
	}
}
