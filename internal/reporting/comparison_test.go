package reporting

import (
	"bytes"
	"strings"
	"testing"
)

func comparisonFixture(t *testing.T) (Document, Document) {
	t.Helper()
	b, err := NewDocument("BP-BASELINE-004", "FD-004", "baseline", "edge-to-data",
		Layer{Status: StatusRecorded, Reference: "plan-baseline", SHA256: strings.Repeat("a", 64), CapturedAtUTC: "2026-10-01T14:00:00Z"},
		Layer{Status: StatusMissing},
		Layer{Status: StatusRecorded, Reference: "witness-baseline", SHA256: strings.Repeat("c", 64), CapturedAtUTC: "2026-10-01T14:10:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewDocument("BP-HARDENED-004", "FD-004", "hardened", "edge-to-data",
		Layer{Status: StatusRecorded, Reference: "plan-hardened", SHA256: strings.Repeat("b", 64), CapturedAtUTC: "2026-10-02T16:00:00Z"},
		Layer{Status: StatusMissing},
		Layer{Status: StatusRecorded, Reference: "witness-hardened", SHA256: strings.Repeat("c", 64), CapturedAtUTC: "2026-09-30T16:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return b, h
}

func TestCompareBoundaryProofsOnlyComparesDigestsNotSecurity(t *testing.T) {
	base, hard := comparisonFixture(t)
	got, err := CompareBoundaryProofs(base, hard)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != VerdictInconclusive || got.DriftAssessment != DriftNotAssessed {
		t.Fatal("fabricated verified verdict or drift conclusion")
	}
	if got.TerraformIntent.EvidenceRelation != RelationDifferentDigest ||
		got.KubernetesObservation.EvidenceRelation != RelationNotComparable ||
		got.NetworkBehavior.EvidenceRelation != RelationSameDigest {
		t.Fatal("incorrect evidence relations")
	}
	if got.NetworkBehavior.Baseline.CapturedAtUTC != base.NetworkBehavior.CapturedAtUTC ||
		got.NetworkBehavior.Hardened.CapturedAtUTC != hard.NetworkBehavior.CapturedAtUTC {
		t.Fatal("separate capture timestamps were replaced")
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	got2, err := CompareBoundaryProofs(base, hard)
	if err != nil {
		t.Fatal(err)
	}
	j1, _ := RenderComparisonJSON(got)
	j2, _ := RenderComparisonJSON(got2)
	if !bytes.Equal(j1, j2) {
		t.Fatal("comparison JSON not deterministic")
	}
}

func TestCompareBoundaryProofsMissingLayersNeverImplySame(t *testing.T) {
	base, hard := comparisonFixture(t)
	hard.NetworkBehavior = Layer{Status: StatusMissing}
	got, err := CompareBoundaryProofs(base, hard)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkBehavior.EvidenceRelation != RelationNotComparable || got.Verdict != VerdictInconclusive {
		t.Fatal("missing evidence was promoted to a finding")
	}
	base.NetworkBehavior = Layer{Status: StatusMissing}
	got, err = CompareBoundaryProofs(base, hard)
	if err != nil || got.NetworkBehavior.EvidenceRelation != RelationNotComparable {
		t.Fatal("two missing layers treated as equivalent evidence")
	}
}

func TestCompareBoundaryProofsRejectsInvalidPairing(t *testing.T) {
	base, hard := comparisonFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(*Document, *Document)
	}{
		{"reversed profiles", func(b, h *Document) { b.Profile, h.Profile = "hardened", "baseline" }},
		{"same profile", func(b, h *Document) { h.Profile = "baseline" }},
		{"scenario mismatch", func(b, h *Document) { h.ScenarioID = "FD-003" }},
		{"route mismatch", func(b, h *Document) { h.Route = "data-to-app" }},
		{"same proof ID", func(b, h *Document) { h.ProofID = b.ProofID }},
		{"forged pass", func(b, h *Document) { h.Verdict = "pass" }},
		{"forged limitations", func(b, h *Document) { h.Limitations[0] = "Verified containment." }},
		{"unsafe path", func(b, h *Document) { b.TerraformIntent.Reference = "../private.json" }},
		{"missing digest", func(b, h *Document) { h.NetworkBehavior.SHA256 = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, h := base, hard
			b.Limitations = append([]string(nil), base.Limitations...)
			h.Limitations = append([]string(nil), hard.Limitations...)
			tc.mutate(&b, &h)
			if _, err := CompareBoundaryProofs(b, h); err == nil {
				t.Fatal("unsupported comparison pair accepted")
			}
		})
	}
}

func TestComparisonRejectsTamperedProjections(t *testing.T) {
	base, hard := comparisonFixture(t)
	orig, err := CompareBoundaryProofs(base, hard)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*ComparisonDocument)
	}{
		{"claim pass", func(d *ComparisonDocument) { d.Verdict = "pass" }},
		{"claim zero drift", func(d *ComparisonDocument) { d.DriftAssessment = "no_drift" }},
		{"wrong schema", func(d *ComparisonDocument) { d.SchemaVersion = "other" }},
		{"contradict digest", func(d *ComparisonDocument) { d.TerraformIntent.EvidenceRelation = RelationSameDigest }},
		{"contradict missing", func(d *ComparisonDocument) { d.KubernetesObservation.EvidenceRelation = RelationSameDigest }},
		{"swapped field", func(d *ComparisonDocument) { d.NetworkBehavior.Layer = "terraform_intent" }},
		{"unsafe source", func(d *ComparisonDocument) { d.NetworkBehavior.Hardened.Reference = "C:\\private\\x" }},
		{"fake caveat", func(d *ComparisonDocument) { d.Limitations[0] = "secure" }},
		{"missing caveat", func(d *ComparisonDocument) { d.Limitations = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := orig
			d.Limitations = append([]string(nil), orig.Limitations...)
			tc.edit(&d)
			if err := d.Validate(); err == nil {
				t.Fatal("tampered report accepted")
			}
			if _, err := RenderComparisonJSON(d); err == nil {
				t.Fatal("JSON accepted tampered report")
			}
			if _, err := RenderComparisonHTML(d); err == nil {
				t.Fatal("HTML accepted tampered report")
			}
		})
	}
}
