package recoverypostapply

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func good() Observation {
	return Observation{
		SchemaVersion: "firedoor.v10.fd004-post-apply-observation.v1",
		Cluster:       Cluster, Context: "kind-" + Cluster,
		BackendType: "local", BackendStatePath: "firedoor-owned.tfstate", Workspace: "default",
		ApplyIntentHashVerified: true, ApplyResultHashVerified: true, ApplyExitCode: 0,
		StateHashVerified: true, SavedPlanHashVerified: true, ReviewReceiptHashVerified: true,
		KubeconfigHashVerified: true, NodesReady: true, CalicoReady: true, CoreDNSReady: true,
		StateSerial: 1, StateLineage: "12345678-1234-1234-1234-123456789012",
		TerraformAddresses: ExpectedAddresses(),
		Zones: []Zone{
			{Zone: "edge", Namespace: "firedoor-edge", NamespaceVerified: true, ServiceAccountVerified: true, DeploymentReady: true, PodReady: true, ServiceVerified: true, EndpointReady: true, NetworkPolicyVerified: true, NetworkPolicyCount: 0},
			{Zone: "app", Namespace: "firedoor-app", NamespaceVerified: true, ServiceAccountVerified: true, DeploymentReady: true, PodReady: true, ServiceVerified: true, EndpointReady: true, NetworkPolicyVerified: true, NetworkPolicyCount: 1},
			{Zone: "data", Namespace: "firedoor-data", NamespaceVerified: true, ServiceAccountVerified: true, DeploymentReady: true, PodReady: true, ServiceVerified: true, EndpointReady: true, NetworkPolicyVerified: true, NetworkPolicyCount: 1},
		},
	}
}
func TestHealthyAndNegativeCases(t *testing.T) {
	o := good()
	r, err := Evaluate(o)
	if err != nil || !r.ManagedStateInventoryVerified || !r.LiveWorkloadsVerified || !r.IngressPolicyShapeVerified || r.NetworkTrafficEnforcementVerified || r.ApplyExecutedByVerifier {
		t.Fatalf("healthy: %+v %v", r, err)
	}
	cases := map[string]func(*Observation){
		"protected target":         func(o *Observation) { o.Cluster = "firedoor-local" },
		"wrong context":            func(o *Observation) { o.Context = "kind-firedoor-local" },
		"wrong backend":            func(o *Observation) { o.BackendStatePath = "terraform.tfstate" },
		"other workspace":          func(o *Observation) { o.Workspace = "unsafe" },
		"wrong result hash":        func(o *Observation) { o.ApplyResultHashVerified = false },
		"wrong intent":             func(o *Observation) { o.ApplyIntentHashVerified = false },
		"failed apply":             func(o *Observation) { o.ApplyExitCode = 1 },
		"changed state":            func(o *Observation) { o.StateHashVerified = false },
		"missing kubeconfig proof": func(o *Observation) { o.KubeconfigHashVerified = false },
		"missing plan proof":       func(o *Observation) { o.SavedPlanHashVerified = false },
		"missing review receipt":   func(o *Observation) { o.ReviewReceiptHashVerified = false },
		"unready Calico":           func(o *Observation) { o.CalicoReady = false },
		"unready CoreDNS":          func(o *Observation) { o.CoreDNSReady = false },
		"unready nodes":            func(o *Observation) { o.NodesReady = false },
		"no state serial":          func(o *Observation) { o.StateSerial = 0 },
		"missing resource":         func(o *Observation) { o.TerraformAddresses = o.TerraformAddresses[1:] },
		"extra resource": func(o *Observation) {
			o.TerraformAddresses = append(o.TerraformAddresses, "module.malicious.other.foo")
		},
		"duplicate resource": func(o *Observation) { o.TerraformAddresses[0] = o.TerraformAddresses[1] },
		"missing namespace":  func(o *Observation) { o.Zones = o.Zones[1:] },
		"wrong namespace":    func(o *Observation) { o.Zones[0].Namespace = "default" },
		"unready deployment": func(o *Observation) { o.Zones[1].DeploymentReady = false },
		"unready pod":        func(o *Observation) { o.Zones[2].PodReady = false },
		"bad SA":             func(o *Observation) { o.Zones[0].ServiceAccountVerified = false },
		"bad service":        func(o *Observation) { o.Zones[0].ServiceVerified = false },
		"empty endpoint":     func(o *Observation) { o.Zones[0].EndpointReady = false },
		"policy missing":     func(o *Observation) { o.Zones[1].NetworkPolicyCount = 0 },
		"policy too broad":   func(o *Observation) { o.Zones[2].NetworkPolicyVerified = false },
		"extra edge policy":  func(o *Observation) { o.Zones[0].NetworkPolicyCount = 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := good()
			mutate(&v)
			if _, err := Evaluate(v); err == nil {
				t.Fatalf("accepted unsafe case")
			}
		})
	}
}
func TestStrictDecode(t *testing.T) {
	raw, err := json.Marshal(good())
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{raw, append([]byte{0xef, 0xbb, 0xbf}, raw...)} {
		parsed, err := Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Evaluate(parsed); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range [][]byte{[]byte("{} {}"), append(raw, []byte("{}")...), []byte(strings.Replace(string(raw), `"cluster":`, `"unknown":1,"cluster":`, 1)), append([]byte{0xef, 0xbb, 0xbf, 0xef, 0xbb, 0xbf}, raw...)} {
		if _, err := Decode(bytes.NewReader(b)); err == nil {
			t.Fatalf("accepted invalid json prefix %q", string(b[:min(25, len(b))]))
		}
	}
}
