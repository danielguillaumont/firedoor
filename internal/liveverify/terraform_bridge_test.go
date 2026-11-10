package liveverify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

func fixtureFD004Terraform(t *testing.T, profile Profile) []byte {
	t.Helper()
	value := func(v any) any { return map[string]any{"sensitive": false, "type": "string", "value": v} }
	object := func(m map[string]string) any {
		types := make(map[string]string, len(m))
		for k := range m {
			types[k] = "string"
		}
		return map[string]any{"sensitive": false, "type": []any{"object", types}, "value": m}
	}
	data := map[string]any{
		"active_profile":           value(string(profile)),
		"owned_namespaces":         object(map[string]string{"edge": "firedoor-edge", "app": "firedoor-app", "data": "firedoor-data"}),
		"services":                 object(map[string]string{"edge": "firedoor-edge.firedoor-edge.svc.cluster.local:8080", "app": "firedoor-app.firedoor-app.svc.cluster.local:8080", "data": "firedoor-data.firedoor-data.svc.cluster.local:8080"}),
		"managed_ingress_policies": object(map[string]string{"app": "firedoor-app-ingress", "data": "firedoor-data-ingress"}),
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixtureFD004Snapshot(profile Profile, at time.Time) kubeinspect.Snapshot {
	proto := "TCP"
	zones := []kubeinspect.Zone{}
	for _, zone := range []string{"edge", "app", "data"} {
		z := kubeinspect.Zone{Name: zone, Namespace: "firedoor-" + zone}
		if zone != "edge" {
			labels := map[string]string{}
			if profile == ProfileHardened {
				src := "edge"
				if zone == "data" {
					src = "app"
				}
				labels["firedoor.dev/zone"] = src
			}
			z.NetworkPolicies = []kubeinspect.Policy{{
				Name:        "firedoor-" + zone + "-ingress",
				PodSelector: kubeinspect.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "firedoor-" + zone}},
				PolicyTypes: []string{"Ingress"},
				Ingress: []kubeinspect.PolicyRule{{
					Peers: []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{MatchLabels: labels}}},
					Ports: []kubeinspect.Port{{Protocol: &proto, Port: float64(8080)}},
				}},
			}}
		}
		zones = append(zones, z)
	}
	return kubeinspect.Snapshot{SchemaVersion: kubeinspect.Schema, Cluster: kubeinspect.Cluster,
		Context: kubeinspect.Context, CapturedAtUTC: at, Zones: zones,
		Limitations: "synthetic local test, no CNI causality"}
}

func fixtureFD004Inputs(t *testing.T) (Plan, []byte, kubeinspect.Snapshot, SuiteObservation) {
	t.Helper()
	plan, obs := verifiedFixture(t)
	return plan, fixtureFD004Terraform(t, plan.Profile), fixtureFD004Snapshot(plan.Profile, obs.StartedAtUTC), obs
}

func TestFD004BridgeConsistentOfflineDoesNotClaimAttestation(t *testing.T) {
	plan, tf, snap, obs := fixtureFD004Inputs(t)
	r, err := AssembleFD004Bridge(plan, tf, snap, obs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "consistent" || r.PolicyStatus != "consistent" || r.BehaviorStatus != "matched" ||
		r.Scenario != "FD-004" || r.RoutesMatched != 6 || r.TerraformAuthenticated ||
		!strings.Contains(r.Limitations, "unauthenticated") {
		t.Fatalf("misleading offline bridge result: %+v", r)
	}
	b, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(b), `"terraform_authenticated":false`) {
		t.Fatal("missing transparent JSON limitations")
	}
}

func TestFD004BridgeRejectsMalformedTerraformClaims(t *testing.T) {
	plan, tf, snap, obs := fixtureFD004Inputs(t)
	changes := map[string]func(map[string]any){
		"wrong profile": func(m map[string]any) { m["active_profile"].(map[string]any)["value"] = "baseline" },
		"foreign namespace": func(m map[string]any) {
			m["owned_namespaces"].(map[string]any)["value"].(map[string]any)["edge"] = "production"
		},
		"wrong policy name": func(m map[string]any) {
			m["managed_ingress_policies"].(map[string]any)["value"].(map[string]any)["app"] = "other"
		},
		"wrong service": func(m map[string]any) {
			m["services"].(map[string]any)["value"].(map[string]any)["data"] = "external.example:443"
		},
		"sensitive value": func(m map[string]any) { m["active_profile"].(map[string]any)["sensitive"] = true },
		"missing output":  func(m map[string]any) { delete(m, "active_profile") },
		"unknown output":  func(m map[string]any) { m["unknown"] = 1 },
		"null type":       func(m map[string]any) { m["active_profile"].(map[string]any)["type"] = nil },
		"wrong type":      func(m map[string]any) { m["active_profile"].(map[string]any)["type"] = "number" },
	}
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal(tf, &input); err != nil {
				t.Fatal(err)
			}
			mutate(input)
			payload, _ := json.Marshal(input)
			if _, err := AssembleFD004Bridge(plan, payload, snap, obs); err == nil {
				t.Fatal("accepted invalid Terraform claim")
			}
		})
	}
	for name, payload := range map[string][]byte{"invalid json": []byte(`{"active_profile":`), "trailing": append(append([]byte{}, tf...), []byte(` {}`)...), "oversize": make([]byte, maxTerraformOutput+1)} {
		t.Run(name, func(t *testing.T) {
			if _, err := AssembleFD004Bridge(plan, payload, snap, obs); err == nil {
				t.Fatal("accepted invalid JSON")
			}
		})
	}
}

func TestFD004BridgePolicyDriftNeverPasses(t *testing.T) {
	changes := map[string]func(*kubeinspect.Snapshot){
		"removed": func(s *kubeinspect.Snapshot) { s.Zones[2].NetworkPolicies = nil },
		"unrestricted ingress": func(s *kubeinspect.Snapshot) {
			s.Zones[2].NetworkPolicies[0].Ingress[0].Peers[0].NamespaceSelector.MatchLabels = map[string]string{}
		},
		"extra ingress": func(s *kubeinspect.Snapshot) {
			s.Zones[2].NetworkPolicies[0].Ingress = append(s.Zones[2].NetworkPolicies[0].Ingress, s.Zones[2].NetworkPolicies[0].Ingress[0])
		},
		"wrong TCP port": func(s *kubeinspect.Snapshot) { s.Zones[1].NetworkPolicies[0].Ingress[0].Ports[0].Port = float64(22) },
		"extra broad policy": func(s *kubeinspect.Snapshot) {
			s.Zones[2].NetworkPolicies = append(s.Zones[2].NetworkPolicies, s.Zones[2].NetworkPolicies[0])
		},
		"bad selector": func(s *kubeinspect.Snapshot) {
			s.Zones[1].NetworkPolicies[0].PodSelector.MatchLabels["app.kubernetes.io/name"] = "other"
		},
		"edge policy": func(s *kubeinspect.Snapshot) { s.Zones[0].NetworkPolicies = []kubeinspect.Policy{{Name: "unexpected"}} },
	}
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			plan, tf, snap, obs := fixtureFD004Inputs(t)
			mutate(&snap)
			r, err := AssembleFD004Bridge(plan, tf, snap, obs)
			if err != nil || r.Status != "drift" || r.PolicyStatus != "drift" {
				t.Fatalf("failed to detect drift: %+v %v", r, err)
			}
		})
	}
}

func TestFD004BridgeRejectsForeignOrStaleObservation(t *testing.T) {
	changes := map[string]func(*kubeinspect.Snapshot, *SuiteObservation){
		"wrong cluster":                func(s *kubeinspect.Snapshot, _ *SuiteObservation) { s.Cluster = "production" },
		"missing snapshot limitations": func(s *kubeinspect.Snapshot, _ *SuiteObservation) { s.Limitations = "" },
		"duplicate zone":               func(s *kubeinspect.Snapshot, _ *SuiteObservation) { s.Zones[0] = s.Zones[1] },
		"stale snapshot":               func(s *kubeinspect.Snapshot, _ *SuiteObservation) { s.CapturedAtUTC = s.CapturedAtUTC.Add(-time.Hour) },
		"forged probe":                 func(_ *kubeinspect.Snapshot, o *SuiteObservation) { o.Routes[0].Attempts[0].Reason = "not observed" },
		"wrong observation profile":    func(_ *kubeinspect.Snapshot, o *SuiteObservation) { o.Profile = ProfileBaseline },
	}
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			plan, tf, snap, obs := fixtureFD004Inputs(t)
			mutate(&snap, &obs)
			if _, err := AssembleFD004Bridge(plan, tf, snap, obs); err == nil {
				t.Fatal("accepted invalid snapshot or evidence")
			}
		})
	}
}

func TestFD004BridgeBehaviorRegressionSurfaced(t *testing.T) {
	plan, tf, snap, obs := fixtureFD004Inputs(t)
	route := &obs.Routes[2]
	route.Observed = ObservedReachable
	route.Reason = "HTTP reachable with exact destination marker"
	route.Attempts = route.Attempts[:1]
	route.Attempts[0].Observation = ObservedReachable
	route.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	r, err := AssembleFD004Bridge(plan, tf, snap, obs)
	if err != nil || r.Status != "mismatch" || r.BehaviorStatus != "mismatched" || r.RoutesMismatch != 1 {
		t.Fatalf("failed to surface unauthorized route: %+v %v", r, err)
	}
}

func TestFD004BridgeInconclusiveNotPassed(t *testing.T) {
	plan, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := verificationFakeExecutor(&labFake{execFails: true}).Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	r, err := AssembleFD004Bridge(plan, fixtureFD004Terraform(t, plan.Profile), fixtureFD004Snapshot(plan.Profile, obs.StartedAtUTC), obs)
	if err != nil || r.Status != "inconclusive" || r.RoutesInconclusive != 6 {
		t.Fatalf("inconclusive became healthy: %+v %v", r, err)
	}
}

func TestFD004BridgeAcceptsEmptyBaselineNamespaceSelector(t *testing.T) {
	t0 := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	s := fixtureFD004Snapshot(ProfileBaseline, t0)
	// The Kubernetes API may omit matchLabels entirely for an empty selector.
	s.Zones[1].NetworkPolicies[0].Ingress[0].Peers[0].NamespaceSelector.MatchLabels = nil
	s.Zones[2].NetworkPolicies[0].Ingress[0].Peers[0].NamespaceSelector.MatchLabels = nil
	status, err := fd004PolicyStatus(s, ProfileBaseline)
	if err != nil || status != "consistent" {
		t.Fatalf("empty selector should be accepted for baseline only: status=%s err=%v", status, err)
	}
	if err := parseTerraformFD004Outputs(fixtureFD004Terraform(t, ProfileBaseline), ProfileBaseline); err != nil {
		t.Fatalf("valid baseline Terraform outputs rejected: %v", err)
	}
}
