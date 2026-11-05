package prooflens

import (
	"strings"
	"testing"
)

func reportFixture(t *testing.T) (ReceiptObservationCorrelation, HistoricalBehaviorContext) {
	t.Helper()
	c, w := historicalContextFixture(t)
	h, err := BuildHistoricalBehaviorContext(c, w)
	if err != nil {
		t.Fatal(err)
	}
	return c, h
}

func TestProofReportVerifiedReadOnlyMarkdownAndTerminal(t *testing.T) {
	c, h := reportFixture(t)
	r, err := BuildProofReport(c, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProofReport(r); err != nil {
		t.Fatal(err)
	}
	if r.Verdict != Inconclusive || !r.LocalChecksOnly || r.TerraformApplied || r.CurrentTrafficProbesRun ||
		r.WitnessExperimentReplayed || r.ContemporaneousEvidence || r.HistoricalWitnessCount != 2 ||
		r.ReceiptID != c.ReceiptID || r.PlannedChanges != c.PlanChanges {
		t.Fatal("report misstated evidence")
	}
	plain, err := RenderProofReportText(r)
	if err != nil {
		t.Fatal(err)
	}
	md, err := RenderProofReportMarkdown(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"INCONCLUSIVE", "NO APPLY", "NOT A PROOF CHAIN", "unreachable -> reachable -> unreachable", "edge-to-data", "data-to-app"} {
		if !strings.Contains(plain, s) {
			t.Fatalf("terminal missing %q", s)
		}
	}
	for _, s := range []string{"# Firedoor // Proof Lens", "speculative only", "historical context only", "| Route |", "`data-to-app`", "`edge-to-data`", "not network enforcement proof"} {
		if !strings.Contains(strings.ToLower(md), strings.ToLower(s)) {
			t.Fatalf("markdown missing %q", s)
		}
	}
	if strings.Contains(plain, `terraform.tfstate`) || strings.Contains(md, `v04-live-edge-to-data-`) {
		t.Fatal("report leaked internal raw evidence names or paths")
	}
}

func TestProofReportRejectsMismatchedAndUpgradedEvidence(t *testing.T) {
	c, h := reportFixture(t)
	wrong := c
	wrong.CorrelationSHA256 = strings.Repeat("f", 64)
	if _, err := BuildProofReport(wrong, h); err == nil {
		t.Fatal("accepted invalid correlation")
	}
	wrong = c
	wrong.ReceiptID = "TFPLAN-ANOTHER-RECEIPT"
	if _, err := BuildProofReport(wrong, h); err == nil {
		t.Fatal("accepted unrelated receipt")
	}
	changed := h
	changed.ContextSHA256 = strings.Repeat("e", 64)
	if _, err := BuildProofReport(c, changed); err == nil {
		t.Fatal("accepted changed historical context")
	}
	r, err := BuildProofReport(c, h)
	if err != nil {
		t.Fatal(err)
	}
	cases := []func(*ProofReport){
		func(r *ProofReport) { r.Verdict = "supported" },
		func(r *ProofReport) { r.TerraformApplied = true },
		func(r *ProofReport) { r.ContemporaneousEvidence = true },
		func(r *ProofReport) { r.CurrentTrafficProbesRun = true },
		func(r *ProofReport) { r.HistoricalWitnessCount = 1 },
		func(r *ProofReport) { r.PlannedChanges.Change++ },
		func(r *ProofReport) { r.ObservationCapturedUTC = "1999-01-01T00:00:00Z" },
		func(r *ProofReport) { r.ReportSHA256 = strings.Repeat("0", 64) },
		func(r *ProofReport) { r.ReceiptID = "x | injected" },
	}
	for i, fn := range cases {
		corrupted := r
		fn(&corrupted)
		if VerifyProofReport(corrupted) == nil {
			t.Fatalf("accepted unsafe report change %d", i)
		}
		if _, err := RenderProofReportText(corrupted); err == nil {
			t.Fatalf("rendered unsafe terminal %d", i)
		}
		if _, err := RenderProofReportMarkdown(corrupted); err == nil {
			t.Fatalf("rendered unsafe markdown %d", i)
		}
	}
}
