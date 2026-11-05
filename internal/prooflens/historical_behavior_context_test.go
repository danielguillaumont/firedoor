package prooflens

import (
	"strings"
	"testing"
	"time"
)

func historicalContextFixture(t *testing.T) (ReceiptObservationCorrelation, []HistoricalWitnessRecord) {
	t.Helper()
	aDir, aName, _ := historicalFixture(t, 0)
	bDir, bName, _ := historicalFixture(t, 1)
	a, err := InspectHistoricalWitnessPair(aDir, aName)
	if err != nil {
		t.Fatal(err)
	}
	b, err := InspectHistoricalWitnessPair(bDir, bName)
	if err != nil {
		t.Fatal(err)
	}
	// Create the v0.5 plan/observation fixture *after* the old A/B/A fixtures.
	p, o := validCorrelationPair(t)
	c, err := CorrelatePlanReceiptWithObservation(p, o)
	if err != nil {
		t.Fatal(err)
	}
	return c, []HistoricalWitnessRecord{b, a} // deliberately unordered
}

func TestHistoricalBehaviorContextHonestReadOnlyTimeline(t *testing.T) {
	c, w := historicalContextFixture(t)
	got, err := BuildHistoricalBehaviorContext(c, w)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyHistoricalBehaviorContext(got); err != nil {
		t.Fatal(err)
	}
	if got.ReceiptID != c.ReceiptID || got.CorrelationSHA256 != c.CorrelationSHA256 ||
		got.HistoricalWitnessCount != 2 || got.HistoricalWitnesses[0].RouteID != "data-to-app" ||
		got.HistoricalWitnesses[1].RouteID != "edge-to-data" || got.ContextStatus != HistoricalBehaviorContextStatus ||
		got.ProofVerdict != Inconclusive || got.ContemporaneousEvidence || got.TerraformApplied ||
		got.CurrentTrafficProbesRun || got.WitnessExperimentReplayed || got.TerraformExecutedInThisStep {
		t.Fatal("historical context fabricated an apply, probe, proof or causal link")
	}
	if !strings.Contains(strings.Join(got.Limitations, " "), "historical") {
		t.Fatal("missing historical limitations")
	}
}

func TestHistoricalBehaviorContextRejectsMissingMismatchedOrReorderedEvidence(t *testing.T) {
	c, w := historicalContextFixture(t)
	wrong := c
	wrong.ProofVerdict = "supported"
	if _, err := BuildHistoricalBehaviorContext(wrong, w); err == nil {
		t.Fatal("accepted forged live correlation")
	}
	if _, err := BuildHistoricalBehaviorContext(c, w[:1]); err == nil {
		t.Fatal("accepted missing witness")
	}
	if _, err := BuildHistoricalBehaviorContext(c, []HistoricalWitnessRecord{w[0], w[0]}); err == nil {
		t.Fatal("accepted duplicate witness")
	}
	altered := append([]HistoricalWitnessRecord(nil), w...)
	altered[0].Context = "wrong-cluster"
	if _, err := BuildHistoricalBehaviorContext(c, altered); err == nil {
		t.Fatal("accepted wrong cluster context")
	}
	altered = append([]HistoricalWitnessRecord(nil), w...)
	altered[0].ReportSHA256 = strings.Repeat("0", 64)
	if _, err := BuildHistoricalBehaviorContext(c, altered); err == nil {
		t.Fatal("accepted modified historic report hash")
	}
	altered = append([]HistoricalWitnessRecord(nil), w...)
	future, _ := time.Parse(time.RFC3339Nano, c.PlanCapturedUTC)
	altered[0].CapturedUTC = future.Add(time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := BuildHistoricalBehaviorContext(c, altered); err == nil {
		t.Fatal("accepted post-plan historical report")
	}
	got, err := BuildHistoricalBehaviorContext(c, w)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []func(*HistoricalBehaviorContext){
		func(x *HistoricalBehaviorContext) { x.ProofVerdict = "supported" },
		func(x *HistoricalBehaviorContext) { x.TerraformApplied = true },
		func(x *HistoricalBehaviorContext) { x.CurrentTrafficProbesRun = true },
		func(x *HistoricalBehaviorContext) { x.ContemporaneousEvidence = true },
		func(x *HistoricalBehaviorContext) { x.ContextStatus = "SECURITY_PROVEN" },
		func(x *HistoricalBehaviorContext) { x.CorrelationSHA256 = strings.Repeat("a", 64) },
		func(x *HistoricalBehaviorContext) { x.ContextSHA256 = strings.Repeat("b", 64) },
	}
	for i, mutate := range mutations {
		changed := got
		mutate(&changed)
		if VerifyHistoricalBehaviorContext(changed) == nil {
			t.Fatalf("accepted overstated summary mutation %d", i)
		}
	}
	changed := got
	changed.HistoricalWitnesses = append([]HistoricalBehaviorReference(nil), got.HistoricalWitnesses...)
	changed.HistoricalWitnesses[0].Phases = append([]HistoricalWitnessPhase(nil), changed.HistoricalWitnesses[0].Phases...)
	changed.HistoricalWitnesses[0].Phases[1].Observed = "unreachable"
	if VerifyHistoricalBehaviorContext(changed) == nil {
		t.Fatal("accepted altered A/B/A phases")
	}
}
