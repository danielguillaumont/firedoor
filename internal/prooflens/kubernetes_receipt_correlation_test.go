package prooflens

import (
	"strings"
	"testing"
	"time"
)

func validCorrelationPair(t *testing.T) (PlanReceipt, KubernetesObservation) {
	t.Helper()
	p := makePlanReceipt(t)
	src := fixtureTerraformSourceManifest(t)
	obs, err := BuildKubernetesObservation(fixturePreflight(), src)
	if err != nil {
		t.Fatal(err)
	}
	// Keep a valid independently sealed snapshot linked to this plan fixture.
	// This is a synthetic test only; no actual Terraform apply is implied.
	obs.SourceTreeSHA256 = p.SourceTreeSHA256
	recorded, err := time.Parse(time.RFC3339Nano, p.RecordedUTC)
	if err != nil {
		t.Fatal(err)
	}
	obs.CapturedUTC = recorded.Add(time.Second).UTC().Format(time.RFC3339Nano)
	obs.ObservationSHA256, err = observationDigest(obs)
	if err != nil || VerifyKubernetesObservation(obs) != nil {
		t.Fatalf("fixture observation invalid: %v", err)
	}
	return p, obs
}

func TestReceiptObservationCorrelationHonestLinkAndIntegrity(t *testing.T) {
	p, obs := validCorrelationPair(t)
	c, err := CorrelatePlanReceiptWithObservation(p, obs)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceiptObservationCorrelation(c); err != nil {
		t.Fatal(err)
	}
	if c.ReceiptID != p.ReceiptID || c.ReceiptSHA256 != p.ReceiptSHA256 ||
		c.ObservationSHA256 != obs.ObservationSHA256 || c.ProofVerdict != Inconclusive ||
		c.PlanApplied || c.NetworkProbesRun || c.ReceiptCorrelation != ReceiptCorrelationStatus {
		t.Fatal("correlation fabricated evidence or failed to link specific records")
	}
	for _, mutate := range []func(*ReceiptObservationCorrelation){
		func(c *ReceiptObservationCorrelation) { c.ProofVerdict = "supported" },
		func(c *ReceiptObservationCorrelation) { c.PlanApplied = true },
		func(c *ReceiptObservationCorrelation) { c.NetworkProbesRun = true },
		func(c *ReceiptObservationCorrelation) { c.ReceiptCorrelation = "KUBERNETES_SECURITY_PROVEN" },
		func(c *ReceiptObservationCorrelation) { c.PolicyCount++ },
		func(c *ReceiptObservationCorrelation) { c.SourceTreeSHA256 = strings.Repeat("a", 64) },
		func(c *ReceiptObservationCorrelation) { c.CorrelationSHA256 = strings.Repeat("0", 64) },
	} {
		wrong := c
		mutate(&wrong)
		if VerifyReceiptObservationCorrelation(wrong) == nil {
			t.Fatal("tampered correlation accepted")
		}
	}
}

func TestReceiptObservationCorrelationRejectsMismatches(t *testing.T) {
	p, obs := validCorrelationPair(t)
	wrong := obs
	wrong.SourceTreeSHA256 = strings.Repeat("a", 64)
	wrong.ObservationSHA256, _ = observationDigest(wrong)
	if _, err := CorrelatePlanReceiptWithObservation(p, wrong); err == nil {
		t.Fatal("two valid but different source digests correlated")
	}
	wrong = obs
	wrong.CapturedUTC = p.RecordedUTC
	wrong.ObservationSHA256, _ = observationDigest(wrong)
	if _, err := CorrelatePlanReceiptWithObservation(p, wrong); err != nil {
		t.Fatal("equal timestamps should be permitted", err)
	}
	ptime, _ := time.Parse(time.RFC3339Nano, p.RecordedUTC)
	wrong.CapturedUTC = ptime.Add(-time.Second).UTC().Format(time.RFC3339Nano)
	wrong.ObservationSHA256, _ = observationDigest(wrong)
	if _, err := CorrelatePlanReceiptWithObservation(p, wrong); err == nil {
		t.Fatal("observation captured before receipt was correlated")
	}
	wrong = obs
	wrong.Namespaces = append([]ObservedNamespace(nil), obs.Namespaces...)
	wrong.Namespaces[1].PolicyCount++
	if _, err := CorrelatePlanReceiptWithObservation(p, wrong); err == nil {
		t.Fatal("modified Kubernetes observation accepted")
	}
	other := p
	other.Applied = true
	if _, err := CorrelatePlanReceiptWithObservation(other, obs); err == nil {
		t.Fatal("applied or forged receipt accepted")
	}
}
