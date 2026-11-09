package reporting

import (
	"bytes"
	"strings"
	"testing"
)

func TestV08ComparisonExecutiveSummaryAndReadableRelations(t *testing.T) {
	baseline, hardened := comparisonFixture(t)
	comparison, err := CompareBoundaryProofs(baseline, hardened)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderComparisonHTML(comparison)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RenderComparisonHTML(comparison)
	if err != nil || !bytes.Equal(rendered, again) {
		t.Fatal("presentation not deterministic")
	}
	for _, wanted := range []string{
		"Evidence at a glance", "Different artifact digests", "Same artifact digest", "Not comparable",
		"Matching historical network artifacts", "INCONCLUSIVE", "NOT ASSESSED",
		"same_artifact_digest", "different_artifact_digests", "not_comparable",
		"<details", "<summary", "class=\"digest-full\"", "@media print", "Content-Security-Policy",
	} {
		if !strings.Contains(string(rendered), wanted) {
			t.Fatalf("comparison missing %q", wanted)
		}
	}
	if len(rendered) > MaxComparisonHTMLBytes {
		t.Fatalf("comparison exceeds maximum: %d", len(rendered))
	}
	for _, digest := range []string{baseline.TerraformIntent.SHA256, hardened.TerraformIntent.SHA256, baseline.NetworkBehavior.SHA256} {
		if digest != "" && !bytes.Contains(rendered, []byte(digest)) {
			t.Fatal("full digest was lost")
		}
	}
	for _, forbidden := range []string{"<script", "https://", "http://", "file://", "Policy enforcement: VERIFIED"} {
		if bytes.Contains(rendered, []byte(forbidden)) {
			t.Fatalf("unexpected unsafe or unsupported text: %q", forbidden)
		}
	}
}

func TestV08ProofExecutiveSummaryIsNonCausal(t *testing.T) {
	doc := fixtureDocument(t)
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"Evidence at a glance", "Recorded reference", "Not available", "INCONCLUSIVE", "independent", "<details", "Historical", "What this report cannot prove"} {
		if !bytes.Contains(html, []byte(wanted)) {
			t.Fatalf("proof missing %q", wanted)
		}
	}
	if len(html) > MaxHTMLBytes {
		t.Fatal("proof exceeds output limit")
	}
	if doc.TerraformIntent.SHA256 != "" && !bytes.Contains(html, []byte(doc.TerraformIntent.SHA256)) {
		t.Fatal("full proof digest missing")
	}
}

func TestDigestAbbreviationRetainsNoControlOverEvidence(t *testing.T) {
	if shortDigest("") != "Not available" {
		t.Fatal("missing digest not handled")
	}
	digest := strings.Repeat("a", 64)
	if shortDigest(digest) != strings.Repeat("a", 12)+"…"+strings.Repeat("a", 8) {
		t.Fatal("unexpected abbreviation")
	}
	if label, _ := presentationRelation(RelationNotComparable); label != "Not comparable" {
		t.Fatal("missing label")
	}
}
