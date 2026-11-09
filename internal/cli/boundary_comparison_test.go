package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/danielguillaumont/firedoor/internal/reporting"
)

func TestComparisonCLIRequiresConsentNoArbitraryPaths(t *testing.T) {
	cmd := newBoundaryComparisonExportCommand()
	for _, flag := range []string{"input", "output", "format", "baseline", "hardened"} {
		if cmd.Flags().Lookup(flag) != nil {
			t.Fatalf("unexpected arbitrary path/format flag: %s", flag)
		}
	}
	for _, consent := range []string{"", "invalid"} {
		root := NewRootCommand()
		root.SetArgs([]string{"proof", "export-comparison", "--root", t.TempDir(), "--consent", consent})
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stdout)
		if err := root.Execute(); err == nil {
			t.Fatal("CLI allowed invalid consent")
		}
		if strings.Contains(stdout.String(), "sha256") || strings.Contains(stdout.String(), "TFPLAN-") {
			t.Fatal("CLI leaked private metadata")
		}
	}
}

func TestComparisonCLIOfflineExportOnce(t *testing.T) {
	rootDir := t.TempDir()
	init := exec.Command("git", "init", "-q", "-b", "feat/v08-proofs", rootDir)
	if out, err := init.CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".gitignore"), []byte("/docs/evidence/local/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(rootDir, "docs", "evidence", "local")
	if err := os.MkdirAll(local, 0700); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, profile, rel string }{
		{"BP-V08-BASE-004", "baseline", prooflens.LocalBaselineProofJSON},
		{"BP-V08-HARD-004", "hardened", prooflens.LocalHardenedProofJSON},
	} {
		doc, err := reporting.NewDocument(item.id, "FD-004", item.profile, "edge-to-data",
			reporting.Layer{Status: reporting.StatusMissing},
			reporting.Layer{Status: reporting.StatusMissing},
			reporting.Layer{Status: reporting.StatusMissing})
		if err != nil {
			t.Fatal(err)
		}
		b, err := reporting.RenderJSON(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDir, filepath.FromSlash(item.rel)), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"proof", "export-comparison", "--root", rootDir, "--consent", prooflens.BoundaryComparisonExportConsent})
	if err := root.Execute(); err != nil {
		t.Fatalf("CLI export failed: %v", err)
	}
	if !strings.Contains(output.String(), "INCONCLUSIVE") || !strings.Contains(output.String(), "NOT ASSESSED") {
		t.Fatal("CLI output omitted conservative verdict")
	}
	if !strings.Contains(output.String(), prooflens.LocalComparisonJSON) {
		t.Fatal("CLI did not report fixed output")
	}
	if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(prooflens.LocalComparisonHTML))); err != nil {
		t.Fatal(err)
	}
	root = NewRootCommand()
	root.SetArgs([]string{"proof", "export-comparison", "--root", rootDir, "--consent", prooflens.BoundaryComparisonExportConsent})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("CLI overwrote existing comparison")
	}
}
