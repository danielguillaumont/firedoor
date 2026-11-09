package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

func TestSyntheticReviewUsesRealOfflineComparisonCLI(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Firedoor-v08-Synthetic-Review")
	if err := generate(target); err != nil {
		t.Fatalf("synthetic demo was rejected: %v", err)
	}
	fileNames := []string{
		"SYNTHETIC-baseline.json", "SYNTHETIC-baseline.html",
		"SYNTHETIC-hardened.json", "SYNTHETIC-hardened.html",
		"SYNTHETIC-comparison.json", "SYNTHETIC-comparison.html",
		"SHA256SUMS-SYNTHETIC.txt", "README-SYNTHETIC.txt",
	}
	files, err := os.ReadDir(target)
	if err != nil || len(files) != len(fileNames) {
		t.Fatalf("wrong number of demo output files: %v", err)
	}
	for _, name := range fileNames {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	jsonData, err := os.ReadFile(filepath.Join(target, "SYNTHETIC-comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc reporting.ComparisonDocument
	if err := json.Unmarshal(jsonData, &doc); err != nil || doc.Validate() != nil {
		t.Fatal("real CLI comparison output is not valid")
	}
	if doc.Verdict != reporting.VerdictInconclusive || doc.DriftAssessment != reporting.DriftNotAssessed ||
		doc.TerraformIntent.EvidenceRelation != reporting.RelationDifferentDigest ||
		doc.KubernetesObservation.EvidenceRelation != reporting.RelationNotComparable ||
		doc.NetworkBehavior.EvidenceRelation != reporting.RelationSameDigest {
		t.Fatal("synthetic comparison makes an unsupported claim")
	}
	for _, name := range []string{"SYNTHETIC-baseline.html", "SYNTHETIC-hardened.html", "SYNTHETIC-comparison.html"} {
		html, err := os.ReadFile(filepath.Join(target, name))
		if err != nil || !bytes.Contains(html, []byte(syntheticWarning)) || !bytes.Contains(html, []byte("INCONCLUSIVE")) {
			t.Fatalf("synthetic labels or verdict missing in %s", name)
		}
	}
	readme, err := os.ReadFile(filepath.Join(target, "README-SYNTHETIC.txt"))
	if err != nil || !strings.Contains(string(readme), "NOT REAL SECURITY EVIDENCE") {
		t.Fatal("missing prominent README warning")
	}
	before := append([]byte(nil), jsonData...)
	if err := generate(target); err == nil {
		t.Fatal("second generation was allowed to overwrite the existing demo")
	}
	after, err := os.ReadFile(filepath.Join(target, "SYNTHETIC-comparison.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retry changed an existing generated report")
	}
}

func TestSyntheticReviewCannotWriteIntoCheckout(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cwd, "forbidden-demo-output")
	if err := generate(target); err == nil {
		t.Fatal("demo allowed output under the source checkout")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("demo created a source-tree output")
	}
}
