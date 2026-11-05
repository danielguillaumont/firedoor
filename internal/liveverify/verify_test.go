package liveverify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func verificationFakeExecutor(f *labFake) Executor {
	e := testExecutor(f)
	e.Inspector.Now = e.Now
	return e
}

func verifiedFixture(t *testing.T) (Plan, SuiteObservation) {
	t.Helper()
	plan, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := verificationFakeExecutor(&labFake{}).Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Status != "completed" {
		t.Fatalf("unexpected status: %s", obs.Status)
	}
	return plan, obs
}

func TestVerifySixRealisticHardenedObservations(t *testing.T) {
	plan, obs := verifiedFixture(t)
	report, err := Verify(plan, obs)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != VerificationSchema || report.Status != "matched" || report.Summary != (VerificationSummary{Total: 6, Match: 6}) {
		t.Fatalf("unexpected verification summary: %+v", report)
	}
	if !strings.Contains(report.Limitations, "does NOT prove") {
		t.Fatal("verification does not communicate limitations")
	}
	for _, route := range report.Routes {
		if route.Verdict != VerdictMatch || route.AttemptCount == 0 {
			t.Fatalf("missing route match or attempts: %+v", route)
		}
	}
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded VerificationReport
	if err := json.Unmarshal(b, &decoded); err != nil || decoded.Summary.Match != 6 {
		t.Fatal("verification report JSON round trip failed")
	}
}

func TestVerifyMismatchDoesNotBecomePass(t *testing.T) {
	plan, obs := verifiedFixture(t)
	// A successful HTTP response on a route expected to be denied is a
	// meaningful mismatch. The attempt marker must support that observation.
	r := &obs.Routes[2]
	r.Observed = ObservedReachable
	r.Reason = "HTTP reachable with exact destination marker"
	r.Attempts = r.Attempts[:1]
	r.Attempts[0].Observation = ObservedReachable
	r.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	report, err := Verify(plan, obs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "mismatched" || report.Summary.Match != 5 || report.Summary.Mismatch != 1 || report.Routes[2].Verdict != VerdictMismatch {
		t.Fatalf("mismatch hidden: %+v", report)
	}
}

func TestVerifyInconclusiveNeverBecomesDeniedOrMatch(t *testing.T) {
	plan, _ := verifiedFixture(t)
	obs, err := verificationFakeExecutor(&labFake{execFails: true}).Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Verify(plan, obs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "inconclusive" || report.Summary.Inconclusive != 6 || report.Summary.Match != 0 {
		t.Fatalf("error masqueraded as success: %+v", report)
	}
}

func TestVerifyInvalidEvidenceRejectedWithoutVerdict(t *testing.T) {
	tests := map[string]func(*SuiteObservation){
		"altered context":     func(o *SuiteObservation) { o.Context = "production" },
		"false provenance":    func(o *SuiteObservation) { o.ProfileProvenance = "terraform-verified-by-cli" },
		"missing attempt":     func(o *SuiteObservation) { o.Routes[2].Attempts = o.Routes[2].Attempts[:1] },
		"forged reachability": func(o *SuiteObservation) { o.Routes[0].Attempts[0].Reason = "some HTTP response" },
		"spoofed target IP":   func(o *SuiteObservation) { o.Routes[0].TargetPodIP = "10.10.10.10" },
		"wrong status": func(o *SuiteObservation) {
			o.Status = "incomplete"
		},
		"unknown observation": func(o *SuiteObservation) { o.Routes[0].Observed = "open" },
		"duplicate attempt":   func(o *SuiteObservation) { o.Routes[2].Attempts[1].Number = 1 },
		"reversed time":       func(o *SuiteObservation) { o.FinishedAtUTC = o.StartedAtUTC.Add(-time.Second) },
		"future attempt": func(o *SuiteObservation) {
			o.Routes[0].Attempts[0].AtUTC = o.FinishedAtUTC.Add(time.Second)
		},
		"missing reason": func(o *SuiteObservation) { o.Routes[0].Reason = "" },
		"foreign pod":    func(o *SuiteObservation) { o.Inventory.Pods[0].IP = "8.8.8.8" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			plan, obs := verifiedFixture(t)
			mutate(&obs)
			if _, err := Verify(plan, obs); err == nil {
				t.Fatalf("accepted inconsistent evidence: %s", name)
			}
		})
	}
}

func TestMismatchHasPriorityOverInconclusive(t *testing.T) {
	plan, obs := verifiedFixture(t)
	r := &obs.Routes[0]
	r.Observed = ObservedInconclusive
	r.Reason = "kubectl exec failed"
	r.Attempts[0].Observation = ObservedInconclusive
	r.Attempts[0].Reason = "kubectl exec failed"
	obs.Status = "incomplete"
	bad := &obs.Routes[2]
	bad.Observed = ObservedReachable
	bad.Reason = "HTTP reachable with exact destination marker"
	bad.Attempts = bad.Attempts[:1]
	bad.Attempts[0].Observation = ObservedReachable
	bad.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	report, err := Verify(plan, obs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "mismatched" || report.Summary.Mismatch != 1 || report.Summary.Inconclusive != 1 {
		t.Fatalf("mismatch not prioritized: %+v", report)
	}
}
