package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestComparisonRenderingBoundedOfflineAndDeterministic(t *testing.T) {
	base, hard := comparisonFixture(t)
	doc, err := CompareBoundaryProofs(base, hard)
	if err != nil {
		t.Fatal(err)
	}
	j1, err := RenderComparisonJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	j2, _ := RenderComparisonJSON(doc)
	if !bytes.Equal(j1, j2) || len(j1) > MaxComparisonJSONBytes {
		t.Fatal("JSON not deterministic or bounded")
	}
	var decoded ComparisonDocument
	if err := json.Unmarshal(j1, &decoded); err != nil || decoded.Validate() != nil {
		t.Fatal("output did not round trip as a safe comparison")
	}
	if !bytes.Contains(j1, []byte(`"drift_assessment": "not_assessed"`)) {
		t.Fatal("explicit unknown drift assessment missing")
	}
	h1, err := RenderComparisonHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := RenderComparisonHTML(doc)
	if !bytes.Equal(h1, h2) || len(h1) > MaxComparisonHTMLBytes {
		t.Fatal("HTML not deterministic or bounded")
	}
	for _, want := range []string{"INCONCLUSIVE", "NOT ASSESSED", "Baseline", "Hardened", "Terraform intent", "Kubernetes observation", "Network behavior", "BP-BASELINE-004", "BP-HARDENED-004", "Not available", "Content-Security-Policy", "same_artifact_digest"} {
		if !strings.Contains(string(h1), want) {
			t.Fatalf("missing key presentation text: %s", want)
		}
	}
	for _, forbidden := range []string{"<script", "https://", "http://", "file://", "<iframe", "terraform.tfstate", "Policy enforcement: VERIFIED"} {
		if strings.Contains(string(h1), forbidden) {
			t.Fatalf("unsafe HTML content: %s", forbidden)
		}
	}
}

func TestComparisonDoesNotInventBaselineFromOneHistoricalDocument(t *testing.T) {
	_, hardened := comparisonFixture(t)
	if _, err := CompareBoundaryProofs(Document{}, hardened); err == nil {
		t.Fatal("invented baseline comparison")
	}
}

func TestComparisonRejectsUnreviewedHTMLOrPathValues(t *testing.T) {
	base, hard := comparisonFixture(t)
	base.TerraformIntent.Reference = "<script>alert('x')</script>"
	if _, err := CompareBoundaryProofs(base, hard); err == nil {
		t.Fatal("accepted HTML injection from evidence")
	}
	base, hard = comparisonFixture(t)
	hard.NetworkBehavior.Reference = "docs/evidence/local/private.json"
	if _, err := CompareBoundaryProofs(base, hard); err == nil {
		t.Fatal("accepted local file path from evidence")
	}
}
