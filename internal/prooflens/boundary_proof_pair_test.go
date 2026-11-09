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

func boundaryPairFixtureRepo(t *testing.T, branch string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", branch, root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/docs/evidence/local/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "evidence", "local"), 0700); err != nil {
		t.Fatal(err)
	}
	baseline := boundaryPairDocument(t, "BP-V08-BASE-004", "baseline", "a")
	hardened := boundaryPairDocument(t, "BP-V08-HARD-004", "hardened", "b")
	bp := filepath.Join(root, filepath.FromSlash(LocalBaselineProofJSON))
	hp := filepath.Join(root, filepath.FromSlash(LocalHardenedProofJSON))
	for _, entry := range []struct {
		path string
		doc  reporting.Document
	}{{bp, baseline}, {hp, hardened}} {
		data, err := reporting.RenderJSON(entry.doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(entry.path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root, bp, hp
}

func boundaryPairDocument(t *testing.T, id, profile, digest string) reporting.Document {
	t.Helper()
	doc, err := reporting.NewDocument(id, "FD-004", profile, "edge-to-data",
		reporting.Layer{Status: reporting.StatusRecorded, Reference: "plan-" + profile, SHA256: strings.Repeat(digest, 64), CapturedAtUTC: "2026-10-01T11:00:00Z"},
		reporting.Layer{Status: reporting.StatusMissing},
		reporting.Layer{Status: reporting.StatusRecorded, Reference: "witness-" + profile, SHA256: strings.Repeat("c", 64), CapturedAtUTC: "2026-10-01T10:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func assertBoundaryPairUntouched(t *testing.T, root, baselinePath, hardenedPath string, baselineBefore, hardenedBefore []byte) {
	t.Helper()
	for _, entry := range []struct {
		path string
		want []byte
	}{{baselinePath, baselineBefore}, {hardenedPath, hardenedBefore}} {
		if entry.want != nil {
			got, err := os.ReadFile(entry.path)
			if err != nil || !bytes.Equal(got, entry.want) {
				t.Fatalf("loader altered source: %s", entry.path)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs", "evidence", "local"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 2 {
		t.Fatalf("read-only loader created unexpected files: %d entries", len(entries))
	}
}

func TestLoadApprovedBoundaryPairReadOnlyAndConservative(t *testing.T) {
	root, bp, hp := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	beforeB, _ := os.ReadFile(bp)
	beforeH, _ := os.ReadFile(hp)
	pair, err := LoadApprovedBoundaryProofPair(root, BoundaryPairReadConsent)
	if err != nil {
		t.Fatalf("pair unexpectedly rejected: %v", err)
	}
	if pair.BaselineProofID != "BP-V08-BASE-004" || pair.HardenedProofID != "BP-V08-HARD-004" ||
		pair.Verdict != reporting.VerdictInconclusive || pair.DriftAssessment != reporting.DriftNotAssessed ||
		pair.TerraformIntent.EvidenceRelation != reporting.RelationDifferentDigest ||
		pair.KubernetesObservation.EvidenceRelation != reporting.RelationNotComparable ||
		pair.NetworkBehavior.EvidenceRelation != reporting.RelationSameDigest {
		t.Fatal("comparison semantics or identity incorrect")
	}
	if _, err := reporting.RenderComparisonHTML(pair); err != nil {
		t.Fatal(err)
	}
	assertBoundaryPairUntouched(t, root, bp, hp, beforeB, beforeH)
}

func TestLoadApprovedBoundaryPairFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		branch  string
		consent string
		mutate  func(t *testing.T, root, baseline, hardened string)
	}{
		{"no consent", "feat/v08-proofs", "", nil},
		{"wrong consent", "feat/v08-proofs", "other", nil},
		{"wrong branch", "feat/v07-containment-map", BoundaryPairReadConsent, nil},
		{"missing baseline", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, b, _ string) {
			if err := os.Remove(b); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing hardened", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			if err := os.Remove(h); err != nil {
				t.Fatal(err)
			}
		}},
		{"same profile", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editBoundaryPairDoc(t, h, func(d *reporting.Document) { d.Profile = "baseline" })
		}},
		{"same proof id", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editBoundaryPairDoc(t, h, func(d *reporting.Document) { d.ProofID = "BP-V08-BASE-004" })
		}},
		{"mismatched route", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editBoundaryPairDoc(t, h, func(d *reporting.Document) { d.Route = "data-to-edge" })
		}},
		{"mismatched scenario", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editBoundaryPairDoc(t, h, func(d *reporting.Document) { d.ScenarioID = "FD-003" })
		}},
		{"forged verdict", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editRawBoundaryPair(t, h, func(b []byte) []byte { return bytes.Replace(b, []byte(`"inconclusive"`), []byte(`"pass"`), 1) })
		}},
		{"altered limitations", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, _, h string) {
			editRawBoundaryPair(t, h, func(b []byte) []byte {
				return bytes.Replace(b, []byte("No policy causation"), []byte("Verified enforcement"), 1)
			})
		}},
		{"unknown field", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, b, _ string) {
			editRawBoundaryPair(t, b, func(data []byte) []byte {
				return bytes.Replace(data, []byte("{\n"), []byte("{\n  \"forged\": true,\n"), 1)
			})
		}},
		{"trailing document", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, b, _ string) {
			editRawBoundaryPair(t, b, func(data []byte) []byte { return append(data, []byte("{}")...) })
		}},
		{"oversize input", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, b, _ string) {
			editRawBoundaryPair(t, b, func(data []byte) []byte { return append(data, bytes.Repeat([]byte(" "), reporting.MaxJSONBytes)...) })
		}},
		{"noncanonical json", "feat/v08-proofs", BoundaryPairReadConsent, func(t *testing.T, _, b, _ string) {
			editRawBoundaryPair(t, b, func(data []byte) []byte { return append(data, '\n') })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, bp, hp := boundaryPairFixtureRepo(t, tc.branch)
			if tc.mutate != nil {
				tc.mutate(t, root, bp, hp)
			}
			if _, err := LoadApprovedBoundaryProofPair(root, tc.consent); err == nil {
				t.Fatal("unsafe pair was accepted")
			}
			// The read-only path cannot create derived comparison files or locks.
			for _, rel := range []string{"docs/evidence/local/v08-boundary-comparison.json", "docs/evidence/local/v08-boundary-comparison.html"} {
				if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Fatalf("unexpected output %s", rel)
				}
			}
		})
	}
}

func editBoundaryPairDoc(t *testing.T, path string, mutate func(*reporting.Document)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc reporting.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	mutate(&doc)
	data, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func editRawBoundaryPair(t *testing.T, path string, mutate func([]byte) []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mutate(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadApprovedBoundaryPairRejectsSymlinksHardlinksTracked(t *testing.T) {
	if runtime.GOOS != "windows" {
		root, bp, hp := boundaryPairFixtureRepo(t, "feat/v08-proofs")
		if err := os.Remove(hp); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(bp, hp); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadApprovedBoundaryProofPair(root, BoundaryPairReadConsent); err == nil {
			t.Fatal("source symlink accepted")
		}
		root2, _, _ := boundaryPairFixtureRepo(t, "feat/v08-proofs")
		local := filepath.Join(root2, "docs", "evidence", "local")
		if err := os.RemoveAll(local); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), local); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadApprovedBoundaryProofPair(root2, BoundaryPairReadConsent); err == nil {
			t.Fatal("directory symlink accepted")
		}
	}
	root, bp, hp := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	if err := os.Remove(hp); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(bp, hp); err != nil {
		t.Skipf("hardlink not supported: %v", err)
	}
	if _, err := LoadApprovedBoundaryProofPair(root, BoundaryPairReadConsent); err == nil {
		t.Fatal("hardlink to same input accepted")
	}
	root2, _, _ := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	cmd := exec.Command("git", "add", "-f", "--", LocalBaselineProofJSON)
	cmd.Dir = root2
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cannot stage test input: %v (%s)", err, out)
	}
	if _, err := LoadApprovedBoundaryProofPair(root2, BoundaryPairReadConsent); err == nil {
		t.Fatal("tracked private source accepted")
	}
}

func TestLoadApprovedBoundaryPairRejectsWrongRoot(t *testing.T) {
	root, _, _ := boundaryPairFixtureRepo(t, "feat/v08-proofs")
	if _, err := LoadApprovedBoundaryProofPair(filepath.Join(root, "docs"), BoundaryPairReadConsent); err == nil {
		t.Fatal("accepted subdirectory")
	}
	if _, err := LoadApprovedBoundaryProofPair(t.TempDir(), BoundaryPairReadConsent); err == nil {
		t.Fatal("accepted non-repository")
	}
}
