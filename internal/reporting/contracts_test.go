package reporting

import (
	"strings"
	"testing"
)

func fixtureDocument(t *testing.T) Document {
	t.Helper()
	recorded := Layer{
		Status:        StatusRecorded,
		Reference:     "TFPLAN-01234567",
		SHA256:        strings.Repeat("a", 64),
		CapturedAtUTC: "2026-11-05T17:22:35Z",
	}
	d, err := NewDocument("BP-TEST-001", "FD-004", "hardened", "data-to-app", recorded,
		Layer{Status: StatusMissing}, Layer{Status: StatusMissing})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestNewDocumentConservative(t *testing.T) {
	d := fixtureDocument(t)
	if d.Verdict != VerdictInconclusive || len(d.Limitations) != 4 {
		t.Fatal("unsupported conclusion")
	}
	if d.TerraformIntent.Status != StatusRecorded || d.NetworkBehavior.Status != StatusMissing {
		t.Fatal("evidence layers conflated")
	}
}

func TestRejectUnsupportedIdentityOrVerdict(t *testing.T) {
	examples := []struct {
		name   string
		mutate func(*Document)
	}{
		{"version", func(d *Document) { d.SchemaVersion = "firedoor.future" }},
		{"contained", func(d *Document) { d.Verdict = "contained" }},
		{"pass", func(d *Document) { d.Verdict = "pass" }},
		{"scenario", func(d *Document) { d.ScenarioID = "FD-999" }},
		{"route", func(d *Document) { d.Route = "external-to-app" }},
		{"profile", func(d *Document) { d.Profile = "production" }},
		{"path", func(d *Document) { d.ProofID = "../../etc/passwd" }},
		{"missing caveat", func(d *Document) { d.Limitations = nil }},
		{"edited caveat", func(d *Document) { d.Limitations[0] = "This is secure." }},
	}
	for _, tc := range examples {
		t.Run(tc.name, func(t *testing.T) {
			d := fixtureDocument(t)
			tc.mutate(&d)
			if err := d.Validate(); err == nil {
				t.Fatal("accepted unsupported report")
			}
			if _, err := RenderJSON(d); err == nil {
				t.Fatal("JSON renderer accepted invalid report")
			}
			if _, err := RenderHTML(d); err == nil {
				t.Fatal("HTML renderer accepted invalid report")
			}
		})
	}
}

func TestRejectUnsafeOrAmbiguousEvidenceReferences(t *testing.T) {
	examples := []struct {
		name   string
		mutate func(*Document)
	}{
		{"raw path", func(d *Document) { d.TerraformIntent.Reference = "docs/evidence/local/private.json" }},
		{"URL", func(d *Document) { d.TerraformIntent.Reference = "https://example.org/proof" }},
		{"HTML", func(d *Document) { d.TerraformIntent.Reference = "<script>alert(1)</script>" }},
		{"newline", func(d *Document) { d.TerraformIntent.Reference = "receipt\nInjected" }},
		{"uppercase digest", func(d *Document) { d.TerraformIntent.SHA256 = strings.Repeat("A", 64) }},
		{"short digest", func(d *Document) { d.TerraformIntent.SHA256 = "abc" }},
		{"offset timestamp", func(d *Document) { d.TerraformIntent.CapturedAtUTC = "2026-11-05T12:22:35-05:00" }},
		{"noncanonical timestamp", func(d *Document) { d.TerraformIntent.CapturedAtUTC = "2026-11-05T17:22:35.000Z" }},
		{"missing provenance", func(d *Document) { d.TerraformIntent.SHA256 = "" }},
		{"unsupported evidence status", func(d *Document) { d.TerraformIntent.Status = "verified" }},
		{"missing layer with hash", func(d *Document) { d.NetworkBehavior.SHA256 = strings.Repeat("b", 64) }},
	}
	for _, tc := range examples {
		t.Run(tc.name, func(t *testing.T) {
			d := fixtureDocument(t)
			tc.mutate(&d)
			if err := d.Validate(); err == nil {
				t.Fatal("untrusted provenance accepted")
			}
		})
	}
}
