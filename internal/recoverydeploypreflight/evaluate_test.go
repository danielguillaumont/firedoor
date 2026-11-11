package recoverydeploypreflight

import (
	"encoding/json"
	"strings"
	"testing"
)

func valid() Observation {
	return Observation{
		SchemaVersion: Schema, Cluster: Cluster, Context: Context,
		BackendType: "local", BackendStatePath: "firedoor-owned.tfstate", Workspace: "default",
		VerifiedEvidence: true, SavedBinaryMatchesJSON: true, SourceHashesVerified: 12,
		PlannedCreates: 14, PlannedUpdates: 0, PlannedDestroys: 0, IngressPolicies: 2,
		KubeconfigIdentityVerified: true,
		DockerNodes:                []Node{{Name: Cluster + "-control-plane", Running: true, ClusterLabel: Cluster}, {Name: Cluster + "-worker", Running: true, ClusterLabel: Cluster}},
		KubernetesNodes:            []Node{{Name: Cluster + "-control-plane", Ready: true}, {Name: Cluster + "-worker", Ready: true}},
		CalicoDesired:              2, CalicoReady: 2, CalicoControllerReady: 1, CoreDNSReady: 2, CoreDNSCount: 2,
		Namespaces: []string{"default", "kube-system", "kube-public", "kube-node-lease", "calico-system"},
	}
}

func TestAcceptIsNotAuthorization(t *testing.T) {
	r, err := Evaluate(valid())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "deployment_review_preflight_passed_no_apply" || !r.EligibleForSeparateDeploymentReview ||
		r.ApplyAuthorized || r.ApplyExecuted || r.RecoveryProven || r.TerraformManagedResourceOwnershipVerified {
		t.Fatalf("unsafe approval: %+v", r)
	}
}

func TestRejectUnsafeObservation(t *testing.T) {
	cases := map[string]func(*Observation){
		"protected cluster":                func(x *Observation) { x.Cluster = "firedoor-local" },
		"foreign context":                  func(x *Observation) { x.Context = "default" },
		"bad schema":                       func(x *Observation) { x.SchemaVersion = "changed" },
		"remote backend":                   func(x *Observation) { x.BackendType = "s3" },
		"wrong state path":                 func(x *Observation) { x.BackendStatePath = "terraform.tfstate" },
		"nondefault workspace":             func(x *Observation) { x.Workspace = "stale" },
		"unverified evidence":              func(x *Observation) { x.VerifiedEvidence = false },
		"binary mismatch":                  func(x *Observation) { x.SavedBinaryMatchesJSON = false },
		"source mismatch":                  func(x *Observation) { x.SourceHashesVerified = 11 },
		"not create only":                  func(x *Observation) { x.PlannedUpdates = 1 },
		"destroy":                          func(x *Observation) { x.PlannedDestroys = 1 },
		"missing policy":                   func(x *Observation) { x.IngressPolicies = 1 },
		"existing state":                   func(x *Observation) { x.TerraformManagedStatePresent = true },
		"state lock":                       func(x *Observation) { x.TerraformStateLockPresent = true },
		"unverified kubeconfig":            func(x *Observation) { x.KubeconfigIdentityVerified = false },
		"unowned Docker":                   func(x *Observation) { x.DockerNodes[0].ClusterLabel = "another" },
		"stopped Docker":                   func(x *Observation) { x.DockerNodes[0].Running = false },
		"duplicate Docker":                 func(x *Observation) { x.DockerNodes[1] = x.DockerNodes[0] },
		"unready kube node":                func(x *Observation) { x.KubernetesNodes[0].Ready = false },
		"foreign kube node":                func(x *Observation) { x.KubernetesNodes[1].Name = "unknown" },
		"Calico unready":                   func(x *Observation) { x.CalicoReady = 1 },
		"Calico controller unready":        func(x *Observation) { x.CalicoControllerReady = 0 },
		"CoreDNS unready":                  func(x *Observation) { x.CoreDNSReady = 1 },
		"CoreDNS unexpected replicas":      func(x *Observation) { x.CoreDNSCount = 3 },
		"app namespace present":            func(x *Observation) { x.Namespaces = append(x.Namespaces, "firedoor-app") },
		"other firedoor namespace present": func(x *Observation) { x.Namespaces = append(x.Namespaces, "firedoor-legacy") },
		"system namespace missing":         func(x *Observation) { x.Namespaces = []string{"default", "kube-system", "kube-public"} },
		"duplicate namespace":              func(x *Observation) { x.Namespaces = append(x.Namespaces, "default") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := valid()
			mutate(&x)
			if _, err := Evaluate(x); err == nil {
				t.Fatal("unsafe observation accepted")
			}
		})
	}
}

func TestDecodeStrict(t *testing.T) {
	for _, raw := range []string{`{"unknown":true}`, `{} {}`, ``, `null true`} {
		if _, err := Decode(strings.NewReader(raw)); err == nil {
			t.Fatalf("unexpected parse acceptance: %q", raw)
		}
	}
}

// Regression: Windows PowerShell 5.1 may prepend a UTF-8 BOM when
// piping JSON to the native Go executable. Accept only one at byte zero.
func TestDecodeOneUTF8BOMOnly(t *testing.T) {
	raw, err := json.Marshal(valid())
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range [][]byte{raw, append([]byte{0xEF, 0xBB, 0xBF}, raw...)} {
		observed, err := Decode(strings.NewReader(string(payload)))
		if err != nil {
			t.Fatalf("expected strict decode acceptance: %v", err)
		}
		if _, err := Evaluate(observed); err != nil {
			t.Fatalf("expected safe read-only evaluation: %v", err)
		}
	}
	for name, payload := range map[string]string{
		"two BOMs":          string([]byte{0xEF, 0xBB, 0xBF, 0xEF, 0xBB, 0xBF}) + string(raw),
		"BOM after space":   " " + string([]byte{0xEF, 0xBB, 0xBF}) + string(raw),
		"BOM then unknown":  string([]byte{0xEF, 0xBB, 0xBF}) + `{"unknown": true}`,
		"BOM then trailing": string([]byte{0xEF, 0xBB, 0xBF}) + string(raw) + " true",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(payload)); err == nil {
				t.Fatal("malformed JSON accepted")
			}
		})
	}
}
