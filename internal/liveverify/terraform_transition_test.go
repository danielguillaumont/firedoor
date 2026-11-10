package liveverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fd004TransitionFixture(t *testing.T) (FD004LiveProof, FD004LiveProof) {
	t.Helper()
	create := func(profile Profile, shift time.Duration) FD004LiveProof {
		c, traffic := fd004ProofFixture(t)
		p, err := NewPlan(profile)
		if err != nil {
			t.Fatal(err)
		}
		s := &traffic.suite
		s.Profile = profile
		s.StartedAtUTC = s.StartedAtUTC.Add(shift)
		s.FinishedAtUTC = s.FinishedAtUTC.Add(shift)
		s.Inventory.CapturedAtUTC = s.Inventory.CapturedAtUTC.Add(shift)
		for i := range s.Routes {
			r := &s.Routes[i]
			r.Expected = p.Probes[i].Expected
			for j := range r.Attempts {
				r.Attempts[j].AtUTC = r.Attempts[j].AtUTC.Add(shift)
			}
			if profile == ProfileBaseline && r.Observed == ObservedUnreachable {
				r.Observed = ObservedReachable
				r.Reason = "HTTP reachable with exact destination marker"
				r.Attempts = r.Attempts[:1]
				r.Attempts[0].Observation = ObservedReachable
				r.Attempts[0].Reason = "destination returned the exact expected HTTP body"
			}
		}
		for i := range c.values {
			v := &c.values[i]
			v.Profile = profile
			v.CapturedAtUTC = v.CapturedAtUTC.Add(shift)
			v.KubernetesSnapshot.CapturedAtUTC = v.KubernetesSnapshot.CapturedAtUTC.Add(shift)
			outputs := fixtureFD004Terraform(t, profile)
			sum := sha256.Sum256(outputs)
			v.TerraformOutputs = json.RawMessage(outputs)
			v.TerraformOutputsSHA256 = hex.EncodeToString(sum[:])
			replacement := fixtureFD004Snapshot(profile, v.KubernetesSnapshot.CapturedAtUTC)
			for z := range v.KubernetesSnapshot.Zones {
				v.KubernetesSnapshot.Zones[z].NetworkPolicies = replacement.Zones[z].NetworkPolicies
			}
		}
		proof, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), profile)
		if err != nil {
			t.Fatalf("could not generate %s test proof: %v", profile, err)
		}
		return proof
	}
	return create(ProfileBaseline, 0), create(ProfileHardened, time.Hour)
}

func TestFD004TransitionAcceptsBoundedBehaviorChangeWithoutCausalityClaim(t *testing.T) {
	b, h := fd004TransitionFixture(t)
	r, err := CompareFD004Transition(b, h)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "observed_transition_cause_unproven" || r.ComparedRoutes != 6 ||
		r.ChangedProtectedRoutes != 2 || r.PreservedAllowedRoutes != 4 ||
		r.TerraformAuthenticated || r.PolicyCausalityProven || r.ApplyReceiptVerified ||
		!strings.Contains(r.Limitations, "unauthenticated") {
		t.Fatalf("unsupported transition claim: %+v", r)
	}
	if r.Routes[2].ID != "edge-to-data" || r.Routes[2].Classification != "newly_unreachable" ||
		r.Routes[3].ID != "data-to-app" || r.Routes[3].Classification != "newly_unreachable" {
		t.Fatalf("incorrect security transition: %+v", r.Routes)
	}
}

func TestFD004TransitionRejectsForgedOrIncompleteProofs(t *testing.T) {
	tests := map[string]func(*FD004LiveProof, *FD004LiveProof){
		"swapped profiles":            func(b, h *FD004LiveProof) { *b, *h = *h, *b },
		"baseline status forged":      func(b, h *FD004LiveProof) { b.Status = "observed_behavior_matched_cause_proven" },
		"hardened verdict missing":    func(b, h *FD004LiveProof) { h.Bridge.RoutesMatched = 5 },
		"false terraform attestation": func(b, h *FD004LiveProof) { b.TerraformAuthenticated = true },
		"false policy attestation":    func(b, h *FD004LiveProof) { h.PolicyCausalityProven = true },
		"missing route":               func(b, h *FD004LiveProof) { b.Observation.Routes = b.Observation.Routes[:5] },
		"forged denial":               func(b, h *FD004LiveProof) { h.Observation.Routes[2].Attempts[0].Reason = "blocked by Calico" },
		"inconclusive renamed match":  func(b, h *FD004LiveProof) { h.Observation.Routes[0].Observed = ObservedInconclusive },
		"changed state inventory":     func(b, h *FD004LiveProof) { b.Before.TerraformStateAddresses = b.Before.TerraformStateAddresses[:13] },
		"altered output bytes":        func(b, h *FD004LiveProof) { h.After.TerraformOutputs = json.RawMessage(`{}`) },
		"changed policy":              func(b, h *FD004LiveProof) { h.Before.KubernetesSnapshot.Zones[2].NetworkPolicies = nil },
		"replaced workload":           func(b, h *FD004LiveProof) { h.Observation.Inventory.Pods[0].UID = "other-uid" },
		"changed deployment": func(b, h *FD004LiveProof) {
			h.Before.KubernetesSnapshot.Zones[0].Deployment.Name = "different"
			h.After.KubernetesSnapshot.Zones[0].Deployment.Name = "different"
		},
		"gap too long": func(b, h *FD004LiveProof) {
			shift := 25 * time.Hour
			h.Observation.StartedAtUTC = h.Observation.StartedAtUTC.Add(shift)
			h.Observation.FinishedAtUTC = h.Observation.FinishedAtUTC.Add(shift)
			h.Observation.Inventory.CapturedAtUTC = h.Observation.Inventory.CapturedAtUTC.Add(shift)
			for i := range h.Observation.Routes {
				for j := range h.Observation.Routes[i].Attempts {
					h.Observation.Routes[i].Attempts[j].AtUTC = h.Observation.Routes[i].Attempts[j].AtUTC.Add(shift)
				}
			}
			for _, c := range []*FD004LiveCollection{&h.Before, &h.After} {
				c.CapturedAtUTC = c.CapturedAtUTC.Add(shift)
				c.KubernetesSnapshot.CapturedAtUTC = c.KubernetesSnapshot.CapturedAtUTC.Add(shift)
			}
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			b, h := fd004TransitionFixture(t)
			change(&b, &h)
			if result, err := CompareFD004Transition(b, h); err == nil {
				t.Fatalf("accepted invalid pair: %+v", result)
			}
		})
	}
}

func TestFD004TransitionRefusesWrongOrderAndReplacedPod(t *testing.T) {
	b, h := fd004TransitionFixture(t)
	if _, err := CompareFD004Transition(h, b); err == nil {
		t.Fatal("accepted reversed profiles")
	}
	h.Before.KubernetesSnapshot.Zones[1].Pods[0].Name = "firedoor-app-other-abc123"
	h.After.KubernetesSnapshot.Zones[1].Pods[0].Name = "firedoor-app-other-abc123"
	if _, err := CompareFD004Transition(b, h); err == nil {
		t.Fatal("accepted a changed pod")
	}
}

func TestFD004TransitionProofJSONStrictFileReading(t *testing.T) {
	b, _ := fd004TransitionFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := ReadFD004ProofFile(path)
	if err != nil || p.SchemaVersion != FD004LiveProofSchema {
		t.Fatalf("valid saved input rejected: %v", err)
	}
	bad := map[string][]byte{
		"empty":    nil,
		"trailing": append(append([]byte(nil), data...), []byte(`{}`)...),
		"unknown":  []byte(`{"unauthorized":true}`),
		"huge":     make([]byte, fd004MaxProofFileBytes+1),
	}
	for name, contents := range bad {
		t.Run(name, func(t *testing.T) {
			fn := filepath.Join(dir, name+".json")
			if err := os.WriteFile(fn, contents, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadFD004ProofFile(fn); err == nil {
				t.Fatal("accepted malformed or oversized proof file")
			}
		})
	}
}
