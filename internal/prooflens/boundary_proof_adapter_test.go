package prooflens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

func validatedV05BoundaryFixture(t *testing.T) ProofReport {
	t.Helper()
	correlation, history := reportFixture(t)
	source, err := BuildProofReport(correlation, history)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestHistoricalBoundaryProofIsNonCausalAndDeterministic(t *testing.T) {
	source := validatedV05BoundaryFixture(t)
	doc, err := BuildHistoricalBoundaryProof(source, "BP-V08-004", "FD-004", "edge-to-data")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if doc.Verdict != reporting.VerdictInconclusive || doc.ScenarioID != "FD-004" || doc.Route != "edge-to-data" || doc.Profile != "hardened" {
		t.Fatal("unexpected claim or scope")
	}
	if doc.TerraformIntent.SHA256 != source.ReceiptSHA256 || doc.KubernetesObservation.SHA256 != source.ObservationSHA256 {
		t.Fatal("detached evidence digests changed")
	}
	if doc.TerraformIntent.Status != reporting.StatusRecorded || doc.KubernetesObservation.Status != reporting.StatusRecorded || doc.NetworkBehavior.Status != reporting.StatusRecorded {
		t.Fatal("missing verified detached provenance")
	}
	var witnessSHA string
	for _, witness := range source.HistoricalWitnesses {
		if witness.RouteID == "edge-to-data" {
			witnessSHA = witness.ReportSHA256
		}
	}
	if doc.NetworkBehavior.SHA256 != witnessSHA {
		t.Fatal("historical witness SHA was altered or substituted")
	}
	plan, e1 := time.Parse(time.RFC3339Nano, doc.TerraformIntent.CapturedAtUTC)
	observation, e2 := time.Parse(time.RFC3339Nano, doc.KubernetesObservation.CapturedAtUTC)
	historical, e3 := time.Parse(time.RFC3339Nano, doc.NetworkBehavior.CapturedAtUTC)
	if e1 != nil || e2 != nil || e3 != nil || !historical.Before(plan) || observation.Before(plan) {
		t.Fatal("independent capture chronology lost")
	}
	jsonFirst, err := reporting.RenderJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	htmlFirst, err := reporting.RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	docAgain, err := BuildHistoricalBoundaryProof(source, "BP-V08-004", "FD-004", "edge-to-data")
	if err != nil {
		t.Fatal(err)
	}
	jsonAgain, _ := reporting.RenderJSON(docAgain)
	htmlAgain, _ := reporting.RenderHTML(docAgain)
	if !bytes.Equal(jsonFirst, jsonAgain) || !bytes.Equal(htmlFirst, htmlAgain) {
		t.Fatal("projection is nondeterministic")
	}
	if !bytes.Contains(jsonFirst, []byte("inconclusive")) || !bytes.Contains(htmlFirst, []byte("INCONCLUSIVE")) {
		t.Fatal("inconclusive verdict lost")
	}
	for _, forbidden := range []string{source.ReceiptID, "docs/evidence/", "v04-live-", "terraform.tfstate", "CONT AINED"} {
		if strings.Contains(string(jsonFirst), forbidden) || strings.Contains(string(htmlFirst), forbidden) {
			t.Fatalf("raw evidence reference appeared in report: %s", forbidden)
		}
	}
}

func TestHistoricalBoundaryProofRejectsUnverifiedSourcesAndInventedScope(t *testing.T) {
	good := validatedV05BoundaryFixture(t)
	bad := []struct {
		name   string
		mutate func(*ProofReport)
	}{
		{"wrong checksum", func(p *ProofReport) { p.ReportSHA256 = strings.Repeat("0", 64) }},
		{"claimed apply", func(p *ProofReport) { p.TerraformApplied = true }},
		{"claimed probe", func(p *ProofReport) { p.CurrentTrafficProbesRun = true }},
		{"claimed contemporaneous", func(p *ProofReport) { p.ContemporaneousEvidence = true }},
		{"unverified network record", func(p *ProofReport) {
			p.HistoricalWitnesses = append([]HistoricalBehaviorReference(nil), p.HistoricalWitnesses...)
			p.HistoricalWitnesses[1].ReportSHA256 = strings.Repeat("b", 64)
		}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			input := good
			tc.mutate(&input)
			if _, err := BuildHistoricalBoundaryProof(input, "BP-V08-004", "FD-004", "edge-to-data"); err == nil {
				t.Fatal("accepted unsupported detached evidence")
			}
		})
	}
	for _, tc := range []struct{ id, scenario, route string }{
		{"BP-OK", "FD-001", "edge-to-data"},
		{"BP-OK", "FD-004", "data-to-app"},
		{"BP-OK", "FD-004", "app-to-data"},
		{"BP-OK", "FD-999", "edge-to-data"},
		{"../../secrets", "FD-004", "edge-to-data"},
	} {
		if _, err := BuildHistoricalBoundaryProof(good, tc.id, tc.scenario, tc.route); err == nil {
			t.Fatalf("accepted unsafe scope: %+v", tc)
		}
	}
}
