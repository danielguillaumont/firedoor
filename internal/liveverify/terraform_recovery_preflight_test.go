package liveverify

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

func fd004GoodPreflightFixture(t *testing.T) FD004LiveCollection {
	t.Helper()
	collector, _ := fd004GoodCollector(t, ProfileHardened)
	c, err := collector.Collect(context.Background(), ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	for i := range c.KubernetesSnapshot.Zones {
		z := &c.KubernetesSnapshot.Zones[i]
		z.Deployment.DesiredReplicas = 1
		z.Deployment.ReadyReplicas = 1
		z.Pods = []kubeinspect.Pod{{Name: "firedoor-" + z.Name + "-example", Labels: map[string]string{"app.kubernetes.io/name": "firedoor-" + z.Name, "firedoor.dev/managed-by": "terraform"}, Phase: "Running", Ready: true}}
	}
	return c
}

func TestFD004RecoveryPreflightHardenedOnlyAndManifests(t *testing.T) {
	c := fd004GoodPreflightFixture(t)
	r, err := BuildFD004RecoveryPreflight(c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "hardened_preflight_consistent_recovery_not_exercised" || r.ApplyAuthorized || r.RecoveryTested || r.TerraformAuthenticated || r.PolicyCausalityProven || r.StateAddressCount != 14 || len(r.RecoveryManifests) != 2 {
		t.Fatalf("incorrect preflight claims: %+v", r)
	}
	for i, expected := range []struct{ zone, source string }{{"app", "edge"}, {"data", "app"}} {
		p := r.RecoveryManifests[i]
		if p.APIVersion != "networking.k8s.io/v1" || p.Kind != "NetworkPolicy" || p.Metadata.Namespace != "firedoor-"+expected.zone || p.Metadata.Name != "firedoor-"+expected.zone+"-ingress" || p.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels["firedoor.dev/zone"] != expected.source || p.Spec.Ingress[0].Ports[0].Port != 8080 || p.Spec.Ingress[0].Ports[0].Protocol != "TCP" {
			t.Fatalf("unexpected recovery manifest: %+v", p)
		}
		b, err := json.Marshal(p)
		if err != nil || !json.Valid(b) {
			t.Fatalf("invalid JSON manifest: %v", err)
		}
	}
}

func TestFD004RecoveryPreflightRejectsUnsafeEvidence(t *testing.T) {
	good := fd004GoodPreflightFixture(t)
	cases := []struct {
		name   string
		mutate func(*FD004LiveCollection)
	}{
		{"baseline", func(c *FD004LiveCollection) { c.Profile = ProfileBaseline }},
		{"foreign cluster", func(c *FD004LiveCollection) { c.Cluster = "production" }},
		{"drift", func(c *FD004LiveCollection) { c.PolicyStatus = "drift" }},
		{"wrong hash", func(c *FD004LiveCollection) { c.TerraformOutputsSHA256 = "deadbeef" }},
		{"missing policy", func(c *FD004LiveCollection) { c.KubernetesSnapshot.Zones[1].NetworkPolicies = nil }},
		{"not ready", func(c *FD004LiveCollection) { c.KubernetesSnapshot.Zones[0].Pods[0].Ready = false }},
		{"replacement Pod", func(c *FD004LiveCollection) {
			c.KubernetesSnapshot.Zones[0].Pods[0].Labels["app.kubernetes.io/name"] = "foreign"
		}},
		{"wrong owned zone", func(c *FD004LiveCollection) { c.KubernetesSnapshot.Zones[0].Name = "prod" }},
		{"wrong replicas", func(c *FD004LiveCollection) { c.KubernetesSnapshot.Zones[0].Deployment.ReadyReplicas = 0 }},
		{"extra zone", func(c *FD004LiveCollection) {
			c.KubernetesSnapshot.Zones = append(c.KubernetesSnapshot.Zones, c.KubernetesSnapshot.Zones[0])
		}},
		{"unauthorized attestation", func(c *FD004LiveCollection) { c.TerraformAuthenticated = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Round-trip clones mutable maps/slices so each case is independent.
			raw, _ := json.Marshal(good)
			var c FD004LiveCollection
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&c)
			if _, err := BuildFD004RecoveryPreflight(c); err == nil {
				t.Fatal("unsafe recovery preflight accepted")
			}
		})
	}
}
