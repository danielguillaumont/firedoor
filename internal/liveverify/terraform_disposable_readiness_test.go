package liveverify

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

func fd004DisposableReadinessFixture() (FD004DisposableReadinessInput, time.Time) {
	now := time.Date(2026, 11, 10, 20, 30, 0, 0, time.UTC)
	cluster := "firedoor-rh-112233aabbcc"
	policy := func(zone, from string) FD004DisposablePolicy {
		protocol := "TCP"
		p := kubeinspect.Policy{Name: "firedoor-" + zone + "-ingress", PolicyTypes: []string{"Ingress"}, PodSelector: kubeinspect.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "firedoor-" + zone}}}
		p.Ingress = []kubeinspect.PolicyRule{{Peers: []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{MatchLabels: map[string]string{"firedoor.dev/zone": from}}}}, Ports: []kubeinspect.Port{{Protocol: &protocol, Port: 8080}}}}
		return FD004DisposablePolicy{Zone: zone, Namespace: "firedoor-" + zone, UID: "00000000-policy-" + zone, ManagedBy: "terraform", Configuration: p}
	}
	in := FD004DisposableReadinessInput{
		SchemaVersion: FD004DisposableReadinessSchema, Scenario: "FD-004", Cluster: cluster, Context: "kind-" + cluster,
		CapturedAtUTC: now.Add(-time.Minute), KindClusters: []string{OwnedCluster, cluster},
		DockerNodes:              []FD004DisposableNode{{Name: cluster + "-control-plane", DockerID: strings.Repeat("a", 64), KindClusterLabel: cluster, Running: true, APIHostPort: 54321}, {Name: cluster + "-worker", DockerID: strings.Repeat("b", 64), KindClusterLabel: cluster, Running: true}},
		KubeconfigCurrentContext: "kind-" + cluster, KubeconfigAPIServer: "https://127.0.0.1:54321",
		KubernetesNodes:         []string{cluster + "-control-plane", cluster + "-worker"},
		TerraformStateAddresses: fd004ExpectedStateAddresses(), TerraformStateSHA256: strings.Repeat("c", 64),
		TerraformProfile: "hardened", TerraformKubeContext: "kind-" + cluster,
		Pods: []FD004DisposablePod{}, Policies: []FD004DisposablePolicy{policy("app", "edge"), policy("data", "app")},
	}
	for i, zone := range []string{"edge", "app", "data"} {
		in.Pods = append(in.Pods, FD004DisposablePod{Zone: zone, Namespace: "firedoor-" + zone, Name: "firedoor-" + zone + "-abc12-def34", UID: fmt.Sprintf("00000000-pod-%s", zone), IP: fmt.Sprintf("10.244.1.%d", 10+i), Ready: true,
			Labels: map[string]string{"app.kubernetes.io/name": "firedoor-" + zone, "firedoor.dev/managed-by": "terraform"}})
	}
	return in, now
}

func TestFD004DisposableReadinessScopedAndNonAuthorizing(t *testing.T) {
	in, now := fd004DisposableReadinessFixture()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReviewFD004DisposableReadinessJSON(raw, in.Cluster, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "offline_identity_consistent_live_authentication_pending" || got.NodeCount != 2 || got.PodCount != 3 || got.HardenedPolicyCount != 2 || got.StateResourceCount != 14 || got.ProbeRouteCount != 6 || got.ApplyAuthorized || got.RecoveryTested || got.TerraformAuthenticated || got.PolicyCausalityProven || !strings.Contains(got.Limitations, "UNAUTHENTICATED") {
		t.Fatalf("overclaim in review: %+v", got)
	}
}

func TestFD004DisposableReadinessFailClosed(t *testing.T) {
	cases := map[string]func(*FD004DisposableReadinessInput){
		"adopt firedoor local":       func(i *FD004DisposableReadinessInput) { i.Cluster = OwnedCluster },
		"historical deleted cluster": func(i *FD004DisposableReadinessInput) { i.Cluster = fd004DeletedHistoricalCluster },
		"changed kube context":       func(i *FD004DisposableReadinessInput) { i.KubeconfigCurrentContext = "kind-firedoor-local" },
		"wrong terraform target":     func(i *FD004DisposableReadinessInput) { i.TerraformKubeContext = OwnedContext },
		"missing protected cluster":  func(i *FD004DisposableReadinessInput) { i.KindClusters = i.KindClusters[1:] },
		"duplicated kind cluster":    func(i *FD004DisposableReadinessInput) { i.KindClusters = append(i.KindClusters, i.Cluster) },
		"extra Docker node":          func(i *FD004DisposableReadinessInput) { i.DockerNodes = append(i.DockerNodes, i.DockerNodes[0]) },
		"foreign Docker label":       func(i *FD004DisposableReadinessInput) { i.DockerNodes[0].KindClusterLabel = OwnedCluster },
		"reused Docker ID":           func(i *FD004DisposableReadinessInput) { i.DockerNodes[0].DockerID = i.DockerNodes[1].DockerID },
		"stopped worker":             func(i *FD004DisposableReadinessInput) { i.DockerNodes[1].Running = false },
		"API endpoint mismatch":      func(i *FD004DisposableReadinessInput) { i.KubeconfigAPIServer = "https://127.0.0.1:6443" },
		"remote API server":          func(i *FD004DisposableReadinessInput) { i.KubeconfigAPIServer = "https://10.0.0.1:54321" },
		"foreign Kube node":          func(i *FD004DisposableReadinessInput) { i.KubernetesNodes[1] = "firedoor-local-worker" },
		"missing Terraform resource": func(i *FD004DisposableReadinessInput) { i.TerraformStateAddresses = i.TerraformStateAddresses[:13] },
		"extra Terraform resource": func(i *FD004DisposableReadinessInput) {
			i.TerraformStateAddresses = append(i.TerraformStateAddresses, "module.foreign.this")
		},
		"duplicate Terraform resource": func(i *FD004DisposableReadinessInput) { i.TerraformStateAddresses[1] = i.TerraformStateAddresses[0] },
		"wrong Terraform profile":      func(i *FD004DisposableReadinessInput) { i.TerraformProfile = "baseline" },
		"fake state digest":            func(i *FD004DisposableReadinessInput) { i.TerraformStateSHA256 = "not-a-hash" },
		"missing Pod":                  func(i *FD004DisposableReadinessInput) { i.Pods = i.Pods[:2] },
		"foreign Pod namespace":        func(i *FD004DisposableReadinessInput) { i.Pods[0].Namespace = "default" },
		"Pod not ready":                func(i *FD004DisposableReadinessInput) { i.Pods[0].Ready = false },
		"replaced Pod UID":             func(i *FD004DisposableReadinessInput) { i.Pods[0].UID = i.Pods[1].UID },
		"duplicate Pod IP":             func(i *FD004DisposableReadinessInput) { i.Pods[1].IP = i.Pods[0].IP },
		"Pod loopback IP":              func(i *FD004DisposableReadinessInput) { i.Pods[1].IP = "127.0.0.1" },
		"foreign pod labels":           func(i *FD004DisposableReadinessInput) { i.Pods[2].Labels["firedoor.dev/managed-by"] = "foreign" },
		"missing policy":               func(i *FD004DisposableReadinessInput) { i.Policies = i.Policies[:1] },
		"weakened policy selector": func(i *FD004DisposableReadinessInput) {
			i.Policies[0].Configuration.Ingress[0].Peers[0].NamespaceSelector.MatchLabels = map[string]string{}
		},
		"policy owned by foreign":          func(i *FD004DisposableReadinessInput) { i.Policies[0].ManagedBy = "manual" },
		"policy changed UID":               func(i *FD004DisposableReadinessInput) { i.Policies[0].UID = i.Pods[0].UID },
		"claimed apply authorization":      func(i *FD004DisposableReadinessInput) { i.ApplyAuthorized = true },
		"claimed Terraform authentication": func(i *FD004DisposableReadinessInput) { i.TerraformAuthenticated = true },
		"claimed CNI causality":            func(i *FD004DisposableReadinessInput) { i.PolicyCausalityProven = true },
		"stale inventory":                  func(i *FD004DisposableReadinessInput) { i.CapturedAtUTC = i.CapturedAtUTC.Add(-15 * time.Minute) },
		"future inventory":                 func(i *FD004DisposableReadinessInput) { i.CapturedAtUTC = i.CapturedAtUTC.Add(3 * time.Minute) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in, now := fd004DisposableReadinessFixture()
			mutate(&in)
			raw, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := ReviewFD004DisposableReadinessJSON(raw, "firedoor-rh-112233aabbcc", now); err == nil {
				t.Fatalf("accepted unsafe %s: %+v", name, result)
			}
		})
	}
}

func TestFD004DisposableReadinessJSONStrict(t *testing.T) {
	in, now := fd004DisposableReadinessFixture()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		`{}`,
		string(raw) + `{}`,
		strings.Replace(string(raw), `"schema_version":`, `"schema_version":"double", "schema_version":`, 1),
		strings.Replace(string(raw), `"scenario":`, `"foreign_field":1,"scenario":`, 1),
		strings.Replace(string(raw), `"apply_authorized":false,`, "", 1),
		strings.Replace(string(raw), `"terraform_authenticated":false,`, "", 1),
		strings.Repeat("a", FD004MaxDisposableReadinessJSON+1),
	}
	for _, bad := range cases {
		if _, err := ReviewFD004DisposableReadinessJSON([]byte(bad), in.Cluster, now); err == nil {
			t.Fatal("accepted malformed input")
		}
	}
	if _, err := ReviewFD004DisposableReadinessJSON(raw, "firedoor-local", now); err == nil {
		t.Fatal("accepted unconfirmed scope")
	}
}
