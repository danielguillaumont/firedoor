package liveverify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fd004ShiftProof(p *FD004LiveProof, shift time.Duration) {
	p.Observation.StartedAtUTC = p.Observation.StartedAtUTC.Add(shift)
	p.Observation.FinishedAtUTC = p.Observation.FinishedAtUTC.Add(shift)
	p.Observation.Inventory.CapturedAtUTC = p.Observation.Inventory.CapturedAtUTC.Add(shift)
	for i := range p.Observation.Routes {
		for j := range p.Observation.Routes[i].Attempts {
			p.Observation.Routes[i].Attempts[j].AtUTC = p.Observation.Routes[i].Attempts[j].AtUTC.Add(shift)
		}
	}
	for _, collection := range []*FD004LiveCollection{&p.Before, &p.After} {
		collection.CapturedAtUTC = collection.CapturedAtUTC.Add(shift)
		collection.KubernetesSnapshot.CapturedAtUTC = collection.KubernetesSnapshot.CapturedAtUTC.Add(shift)
	}
}

func fd004RecoveryEvidenceFixture(t *testing.T) (FD004LiveProof, FD004LiveProof, FD004LiveProof) {
	t.Helper()
	baseline, restored := fd004TransitionFixture(t)
	raw, err := json.Marshal(restored)
	if err != nil {
		t.Fatal(err)
	}
	var initial FD004LiveProof
	if err := json.Unmarshal(raw, &initial); err != nil {
		t.Fatal(err)
	}
	fd004ShiftProof(&initial, -2*time.Hour)
	return initial, baseline, restored
}

func TestFD004RecoveryEvidenceAcceptsReversibleObservationsWithoutCausalClaims(t *testing.T) {
	initial, baseline, restored := fd004RecoveryEvidenceFixture(t)
	result, err := ReviewFD004RecoveryEvidence(initial, baseline, restored)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "observed_recovery_sequence_cause_unproven" || result.ComparedRoutes != 6 ||
		result.ReversedAndRestoredProtectedRoutes != 2 || result.PreservedAllowedRoutes != 4 ||
		result.TerraformAuthenticated || result.PolicyCausalityProven || result.ApplyReceiptVerified ||
		!strings.Contains(result.Limitations, "not authenticated") {
		t.Fatalf("unsupported recovery claim: %+v", result)
	}
	if result.Routes[2].Classification != "reversed_and_restored" || result.Routes[3].Classification != "reversed_and_restored" {
		t.Fatal("protected route reversal missing")
	}
}

func TestFD004RecoveryEvidenceFailsClosed(t *testing.T) {
	cases := map[string]func(*FD004LiveProof, *FD004LiveProof, *FD004LiveProof){
		"wrong first profile":      func(a, b, c *FD004LiveProof) { *a = *b },
		"duplicate baseline":       func(a, b, c *FD004LiveProof) { *c = *b },
		"wrong phase order":        func(a, b, c *FD004LiveProof) { *a, *c = *c, *a },
		"stale phase":              func(a, b, c *FD004LiveProof) { fd004ShiftProof(a, -26*time.Hour) },
		"replaced pod":             func(a, b, c *FD004LiveProof) { a.Observation.Inventory.Pods[0].UID = "other-uid" },
		"missing protected denial": func(a, b, c *FD004LiveProof) { a.Observation.Routes[2].Observed = ObservedReachable },
		"untrusted attestation":    func(a, b, c *FD004LiveProof) { c.PolicyCausalityProven = true },
		"changed restored output":  func(a, b, c *FD004LiveProof) { c.After.TerraformOutputs = []byte(`{}`) },
		"changed initial config":   func(a, b, c *FD004LiveProof) { a.Before.KubernetesSnapshot.Zones[1].NetworkPolicies = nil },
		"changed interim workload": func(a, b, c *FD004LiveProof) {
			b.Before.KubernetesSnapshot.Zones[0].Deployment.Name = "foreign-workload"
			b.After.KubernetesSnapshot.Zones[0].Deployment.Name = "foreign-workload"
		},
		"no route":              func(a, b, c *FD004LiveProof) { c.Observation.Routes = c.Observation.Routes[:5] },
		"forged restored claim": func(a, b, c *FD004LiveProof) { c.Status = "policy_causality_proven" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a, b, c := fd004RecoveryEvidenceFixture(t)
			mutate(&a, &b, &c)
			if report, err := ReviewFD004RecoveryEvidence(a, b, c); err == nil {
				t.Fatalf("accepted unsupported evidence: %+v", report)
			}
		})
	}
}
