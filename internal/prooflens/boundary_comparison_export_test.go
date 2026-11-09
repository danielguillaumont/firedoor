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

func comparisonPaths(root string) (string, string, string) {
	return filepath.Join(root, filepath.FromSlash(LocalComparisonJSON)),
		filepath.Join(root, filepath.FromSlash(LocalComparisonHTML)),
		filepath.Join(root, filepath.FromSlash(boundaryComparisonExportLock))
}

func TestApprovedComparisonExportFromSyntheticReviewedPair(t *testing.T) {
	root, baseline, hardened := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	beforeB, _ := os.ReadFile(baseline)
	beforeH, _ := os.ReadFile(hardened)
	jrel, hrel, err := ExportApprovedBoundaryComparisonOnce(root, BoundaryComparisonExportConsent)
	if err != nil || jrel != LocalComparisonJSON || hrel != LocalComparisonHTML {
		t.Fatalf("offline export unexpectedly rejected: %v", err)
	}
	jpath, hpath, lock := comparisonPaths(root)
	jsonData, err := os.ReadFile(jpath)
	if err != nil {
		t.Fatal(err)
	}
	var doc reporting.ComparisonDocument
	if err := json.Unmarshal(jsonData, &doc); err != nil || doc.Validate() != nil {
		t.Fatal("comparison artifact invalid")
	}
	canonical, err := reporting.RenderComparisonJSON(doc)
	if err != nil || !bytes.Equal(jsonData, canonical) {
		t.Fatal("JSON is not deterministic and canonical")
	}
	if doc.Verdict != reporting.VerdictInconclusive || doc.DriftAssessment != reporting.DriftNotAssessed {
		t.Fatal("comparison made unsupported enforcement or drift claim")
	}
	htmlData, err := os.ReadFile(hpath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(htmlData, []byte("INCONCLUSIVE")) || !bytes.Contains(htmlData, []byte("NOT ASSESSED")) || bytes.Contains(htmlData, []byte("<script")) {
		t.Fatal("HTML missing safety warning or contains active script")
	}
	if _, err := os.Lstat(lock); !os.IsNotExist(err) {
		t.Fatal("lock remained after successful export")
	}
	if b, err := os.ReadFile(baseline); err != nil || !bytes.Equal(b, beforeB) {
		t.Fatal("baseline modified")
	}
	if h, err := os.ReadFile(hardened); err != nil || !bytes.Equal(h, beforeH) {
		t.Fatal("hardened source modified")
	}
	if _, _, err := ExportApprovedBoundaryComparisonOnce(root, BoundaryComparisonExportConsent); err == nil {
		t.Fatal("export overwrote existing output")
	}
	if b, err := os.ReadFile(jpath); err != nil || !bytes.Equal(b, jsonData) {
		t.Fatal("second export changed JSON")
	}
	if b, err := os.ReadFile(hpath); err != nil || !bytes.Equal(b, htmlData) {
		t.Fatal("second export changed HTML")
	}
}

func TestApprovedComparisonExportFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		consent string
		mutate  func(t *testing.T, root, baseline, hardened string)
	}{
		{"no consent", "", nil},
		{"wrong consent", "invalid", nil},
		{"missing baseline", BoundaryComparisonExportConsent, func(t *testing.T, _, baseline, _ string) {
			if err := os.Remove(baseline); err != nil {
				t.Fatal(err)
			}
		}},
		{"same provenance id", BoundaryComparisonExportConsent, func(t *testing.T, _, b, h string) {
			editBoundaryPairDoc(t, h, func(d *reporting.Document) { d.ProofID = "BP-V08-BASE-004" })
		}},
		{"forged verdict", BoundaryComparisonExportConsent, func(t *testing.T, _, b, h string) {
			editRawBoundaryPair(t, h, func(data []byte) []byte { return bytes.Replace(data, []byte("inconclusive"), []byte("pass"), 1) })
		}},
		{"existing JSON", BoundaryComparisonExportConsent, func(t *testing.T, root, _, _ string) {
			j, _, _ := comparisonPaths(root)
			if err := os.WriteFile(j, []byte("keep me"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"existing HTML", BoundaryComparisonExportConsent, func(t *testing.T, root, _, _ string) {
			_, h, _ := comparisonPaths(root)
			if err := os.WriteFile(h, []byte("keep me"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"existing lock", BoundaryComparisonExportConsent, func(t *testing.T, root, _, _ string) {
			_, _, l := comparisonPaths(root)
			if err := os.WriteFile(l, []byte("stale lock"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unignored directory", BoundaryComparisonExportConsent, func(t *testing.T, root, _, _ string) {
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(""), 0600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, baseline, hardened := boundaryPairFixtureRepo(t, "feat/v08-proofs")
			if tc.mutate != nil {
				tc.mutate(t, root, baseline, hardened)
			}
			jpath, hpath, lock := comparisonPaths(root)
			pre := map[string][]byte{}
			for _, path := range []string{jpath, hpath, lock} {
				if b, err := os.ReadFile(path); err == nil {
					pre[path] = b
				}
			}
			if _, _, err := ExportApprovedBoundaryComparisonOnce(root, tc.consent); err == nil {
				t.Fatal("unsafe export was accepted")
			}
			for _, path := range []string{jpath, hpath, lock} {
				if old, ok := pre[path]; ok {
					b, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(old, b) {
						t.Fatalf("modified existing private file: %s", path)
					}
				} else if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("created output after rejection: %s (%v)", path, err)
				}
			}
		})
	}
}

func TestApprovedComparisonExportRejectsTrackedAndLinkedOutputs(t *testing.T) {
	root, _, _ := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	jpath, _, _ := comparisonPaths(root)
	if err := os.WriteFile(jpath, []byte("tracked"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "-f", "--", LocalComparisonJSON)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cannot stage test output: %v (%s)", err, output)
	}
	if _, _, err := ExportApprovedBoundaryComparisonOnce(root, BoundaryComparisonExportConsent); err == nil {
		t.Fatal("tracked output accepted")
	}
	if runtime.GOOS != "windows" {
		root2, _, _ := boundaryPairFixtureRepo(t, "feat/v08-proofs")
		jpath, _, _ = comparisonPaths(root2)
		if err := os.Symlink(filepath.Join(root2, ".gitignore"), jpath); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ExportApprovedBoundaryComparisonOnce(root2, BoundaryComparisonExportConsent); err == nil {
			t.Fatal("symlink output accepted")
		}
	}
}

func TestApprovedComparisonExportRejectsWrongRootAndBranch(t *testing.T) {
	for _, branch := range []string{"feat/v07-containment-map", "feat/v08-proofs"} {
		root, _, _ := boundaryPairFixtureRepo(t, branch)
		path := root
		if branch == "feat/v08-proofs" {
			path = filepath.Join(root, "docs")
		}
		if _, _, err := ExportApprovedBoundaryComparisonOnce(path, BoundaryComparisonExportConsent); err == nil {
			t.Fatalf("unsafe root/branch accepted: %s", strings.TrimSpace(branch))
		}
	}
}
