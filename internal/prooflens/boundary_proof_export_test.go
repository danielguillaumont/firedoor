package prooflens

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

func boundaryExportFixtureRepo(t *testing.T, branch string) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", branch, root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/docs/evidence/local/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "evidence", "local"), 0700); err != nil {
		t.Fatal(err)
	}
	source := validatedV05BoundaryFixture(t)
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(ConsoleReportRelativePath))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return root, path, data
}

func assertBoundaryNoOutputs(t *testing.T, root string) {
	t.Helper()
	for _, rel := range []string{LocalBoundaryJSON, LocalBoundaryHTML, boundaryExportLock} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("unexpected export/lock %s: %v", rel, err)
		}
	}
}

func TestOfflineBoundaryExportCreatesTwoRestrictedFilesOnce(t *testing.T) {
	root, _, _ := boundaryExportFixtureRepo(t, "feat/v08-proofs")
	jsonName, htmlName, err := ExportApprovedBoundaryProofOnce(root, BoundaryExportConsent)
	if err != nil {
		t.Fatalf("approved offline export rejected: %v", err)
	}
	if jsonName != LocalBoundaryJSON || htmlName != LocalBoundaryHTML {
		t.Fatal("non-fixed export location")
	}
	jsonPath := filepath.Join(root, filepath.FromSlash(jsonName))
	htmlPath := filepath.Join(root, filepath.FromSlash(htmlName))
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc reporting.Document
	if err := json.Unmarshal(jsonData, &doc); err != nil || doc.Validate() != nil || doc.Verdict != reporting.VerdictInconclusive {
		t.Fatal("exported contract invalid or not inconclusive")
	}
	if len(jsonData) > reporting.MaxJSONBytes || len(htmlData) > reporting.MaxHTMLBytes || !bytes.Contains(htmlData, []byte("INCONCLUSIVE")) {
		t.Fatal("unexpected or oversized HTML/JSON output")
	}
	for _, raw := range [][]byte{jsonData, htmlData} {
		for _, secret := range []string{"docs/evidence/local", "kind-firedoor-local", "terraform.tfstate", "TFPLAN-"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Fatalf("source metadata leaked: %q", secret)
			}
		}
	}
	if runtime.GOOS != "windows" {
		for _, p := range []string{jsonPath, htmlPath} {
			info, _ := os.Stat(p)
			if info.Mode().Perm()&0077 != 0 {
				t.Fatal("export must not be group/world-readable")
			}
		}
	}
	if _, _, err := ExportApprovedBoundaryProofOnce(root, BoundaryExportConsent); err == nil {
		t.Fatal("second export overwrote files")
	}
	after, _ := os.ReadFile(jsonPath)
	if !bytes.Equal(after, jsonData) {
		t.Fatal("repeated export altered existing output")
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(boundaryExportLock))); !os.IsNotExist(err) {
		t.Fatal("successful export left a lock")
	}
}

func TestOfflineBoundaryExportFailsClosedBeforeCreatingFiles(t *testing.T) {
	cases := []struct {
		name    string
		branch  string
		consent string
		mutate  func(t *testing.T, root, source string, data []byte)
	}{
		{"missing consent", "feat/v08-proofs", "", nil},
		{"incorrect consent", "feat/v08-proofs", "I-APPROVE-OTHER", nil},
		{"wrong branch", "feat/v07-containment-map", BoundaryExportConsent, nil},
		{"missing input", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, _, p string, _ []byte) {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}},
		{"trailing document", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, _, p string, b []byte) {
			if err := os.WriteFile(p, append(b, []byte("\n{}")...), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unknown field", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, _, p string, b []byte) {
			changed := strings.Replace(string(b), "{", `{"forged":true,`, 1)
			if err := os.WriteFile(p, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"oversized report", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, _, p string, b []byte) {
			if err := os.WriteFile(p, append(b, bytes.Repeat([]byte(" "), 130*1024)...), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"inconsistent report", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, _, p string, b []byte) {
			changed := strings.Replace(string(b), `"inconclusive"`, `"contained"`, 1)
			if changed == string(b) {
				t.Fatal("fixture missing verdict")
			}
			if err := os.WriteFile(p, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"existing output", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, root, _ string, _ []byte) {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(LocalBoundaryHTML)), []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"active lock", "feat/v08-proofs", BoundaryExportConsent, func(t *testing.T, root, _ string, _ []byte) {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(boundaryExportLock)), []byte("locked"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, source, data := boundaryExportFixtureRepo(t, tc.branch)
			if tc.mutate != nil {
				tc.mutate(t, root, source, data)
			}
			if _, _, err := ExportApprovedBoundaryProofOnce(root, tc.consent); err == nil {
				t.Fatal("unsafe export accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(LocalBoundaryJSON))); !os.IsNotExist(err) {
				t.Fatal("invalid export wrote JSON")
			}
			// A deliberately pre-existing HTML file or lock must never be deleted.
			for _, rel := range []string{LocalBoundaryHTML, boundaryExportLock} {
				if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && tc.name != "existing output" && tc.name != "active lock" {
					t.Fatalf("invalid export left artifact %s", rel)
				}
			}
		})
	}
}

func TestOfflineBoundaryExportRejectsSymlinkAndTrackedEvidence(t *testing.T) {
	if runtime.GOOS != "windows" { // Windows creation requires developer-mode privileges.
		root, src, _ := boundaryExportFixtureRepo(t, "feat/v08-proofs")
		if err := os.Remove(src); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, ".gitignore"), src); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ExportApprovedBoundaryProofOnce(root, BoundaryExportConsent); err == nil {
			t.Fatal("source symlink accepted")
		}
		assertBoundaryNoOutputs(t, root)
		root2, _, _ := boundaryExportFixtureRepo(t, "feat/v08-proofs")
		if err := os.RemoveAll(filepath.Join(root2, "docs", "evidence", "local")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), filepath.Join(root2, "docs", "evidence", "local")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ExportApprovedBoundaryProofOnce(root2, BoundaryExportConsent); err == nil {
			t.Fatal("directory symlink accepted")
		}
	}
	root3, _, _ := boundaryExportFixtureRepo(t, "feat/v08-proofs")
	cmd := exec.Command("git", "add", "-f", "--", ConsoleReportRelativePath)
	cmd.Dir = root3
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("track fixture: %v %s", err, out)
	}
	if _, _, err := ExportApprovedBoundaryProofOnce(root3, BoundaryExportConsent); err == nil {
		t.Fatal("tracked private evidence accepted")
	}
	assertBoundaryNoOutputs(t, root3)
}

func TestOfflineBoundaryExportCannotEscapeRepo(t *testing.T) {
	root, _, _ := boundaryExportFixtureRepo(t, "feat/v08-proofs")
	if _, _, err := ExportApprovedBoundaryProofOnce(filepath.Join(root, "docs"), BoundaryExportConsent); err == nil {
		t.Fatal("subfolder treated as repo root")
	}
	if _, _, err := ExportApprovedBoundaryProofOnce(t.TempDir(), BoundaryExportConsent); err == nil {
		t.Fatal("non-repo accepted")
	}
}
