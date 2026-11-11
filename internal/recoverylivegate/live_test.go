package recoverylivegate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func goodSnapshot() Snapshot {
	return Snapshot{
		Cluster: Cluster, Context: Context,
		KindClusters:     []string{Cluster, "firedoor-local"},
		Containers:       []Container{{Name: Cluster + "-control-plane", ClusterLabel: Cluster, Running: true}, {Name: Cluster + "-worker", ClusterLabel: Cluster, Running: true}},
		ControlPlanePort: "49217", KubeconfigContext: Context, KubeconfigServer: "https://127.0.0.1:49217",
		PrivateKubeconfigHashMatched: true,
		Nodes:                        []Node{{Name: Cluster + "-worker", Ready: true}, {Name: Cluster + "-control-plane", Ready: true}},
		CalicoDesired:                2, CalicoReady: 2, CalicoControllersReady: 1,
		CoreDNSCount: 2, CoreDNSReady: 2,
		Namespaces:                 []string{"default", "kube-system", "kube-node-lease", "kube-public"},
		DefaultKubeconfigUnchanged: true, StagedTerraformLockfileHashMatched: true,
	}
}

func TestGoodReadOnlyPreflightIsBlocked(t *testing.T) {
	r, err := Evaluate(goodSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !r.CurrentClusterAuthenticated || !r.CurrentClusterHealthy || !r.DeploymentBlocked ||
		r.CurrentTerraformStateVerified || r.FreshPlanBoundToState || r.ApplyAuthorized || r.ApplyExecuted || r.RecoveryProven || r.Status != Status {
		t.Fatalf("unexpected authorization claim: %+v", r)
	}
}

func TestNegativeObservationsFailClosed(t *testing.T) {
	cases := map[string]func(*Snapshot){
		"other cluster":               func(s *Snapshot) { s.Cluster = "firedoor-local" },
		"other context":               func(s *Snapshot) { s.Context = "kind-firedoor-local" },
		"default kubectl context":     func(s *Snapshot) { s.KubeconfigContext = "kind-firedoor-local" },
		"missing protected cluster":   func(s *Snapshot) { s.KindClusters = []string{Cluster, "random"} },
		"missing target cluster":      func(s *Snapshot) { s.KindClusters = []string{"firedoor-local", "random"} },
		"duplicate cluster":           func(s *Snapshot) { s.KindClusters = []string{Cluster, "firedoor-local", Cluster} },
		"stopped control plane":       func(s *Snapshot) { s.Containers[0].Running = false },
		"foreign Docker label":        func(s *Snapshot) { s.Containers[1].ClusterLabel = "firedoor-local" },
		"unexpected container":        func(s *Snapshot) { s.Containers[0].Name = "random" },
		"too many containers":         func(s *Snapshot) { s.Containers = append(s.Containers, s.Containers[0]) },
		"port malformed":              func(s *Snapshot) { s.ControlPlanePort = "abc" },
		"port overflow":               func(s *Snapshot) { s.ControlPlanePort = "65536" },
		"port low":                    func(s *Snapshot) { s.ControlPlanePort = "30" },
		"API rebound":                 func(s *Snapshot) { s.KubeconfigServer = "https://127.0.0.1:8000" },
		"API foreign host":            func(s *Snapshot) { s.KubeconfigServer = "https://127.0.0.2:49217" },
		"private kubeconfig mutated":  func(s *Snapshot) { s.PrivateKubeconfigHashMatched = false },
		"default kubeconfig mutated":  func(s *Snapshot) { s.DefaultKubeconfigUnchanged = false },
		"Terraform lockfile mutated":  func(s *Snapshot) { s.StagedTerraformLockfileHashMatched = false },
		"node not ready":              func(s *Snapshot) { s.Nodes[1].Ready = false },
		"node replaced":               func(s *Snapshot) { s.Nodes[1].Name = "other-worker" },
		"node count":                  func(s *Snapshot) { s.Nodes = s.Nodes[:1] },
		"calico desired changed":      func(s *Snapshot) { s.CalicoDesired = 1 },
		"calico degraded":             func(s *Snapshot) { s.CalicoReady = 1 },
		"calico controller not ready": func(s *Snapshot) { s.CalicoControllersReady = 0 },
		"coredns count changed":       func(s *Snapshot) { s.CoreDNSCount = 1 },
		"coredns not ready":           func(s *Snapshot) { s.CoreDNSReady = 1 },
		"managed namespace exists":    func(s *Snapshot) { s.Namespaces = append(s.Namespaces, "firedoor-data") },
		"namespace missing":           func(s *Snapshot) { s.Namespaces = s.Namespaces[:3] },
		"duplicate namespace":         func(s *Snapshot) { s.Namespaces = append(s.Namespaces, "default") },
		"state file found":            func(s *Snapshot) { s.LocalStateFilesFound = true },
		"pretend state verified":      func(s *Snapshot) { s.TerraformStateVerified = true },
		"pretend fresh plan":          func(s *Snapshot) { s.FreshPlanBoundToState = true },
		"pretend apply permit":        func(s *Snapshot) { s.ApplyAuthorized = true },
		"pretend apply executed":      func(s *Snapshot) { s.ApplyExecuted = true },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s := goodSnapshot()
			change(&s)
			if report, err := Evaluate(s); err == nil {
				t.Fatalf("should reject: %+v", report)
			}
		})
	}
}

func TestStrictJSON(t *testing.T) {
	raw, err := json.Marshal(goodSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(dec); err != nil {
		t.Fatal(err)
	}
	bad := [][]byte{
		nil,
		[]byte(`{"cluster":"a","cluster":"b"}`),
		[]byte(`{"namespaces":["default"],"containers":[{"name":"x","name":"y"}]}`),
		append(append([]byte(nil), raw...), []byte(` true`)...),
		bytes.Repeat([]byte{'x'}, 256*1024+1),
		append(append([]byte(nil), raw...), 0),
		[]byte(`{"unrecognized":true}`),
	}
	for i, b := range bad {
		if _, err := Decode(b); err == nil {
			t.Fatalf("accepted malformed snapshot %d", i)
		}
	}
	if _, err := Decode([]byte(strings.Repeat(" ", 20))); err == nil {
		t.Fatal("accepted empty JSON")
	}
}
