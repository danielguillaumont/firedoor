// Package recoverylivegate checks a bounded, read-only FD-004 cluster snapshot.
// A healthy cluster never implies an authorized Terraform deployment.
package recoverylivegate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

const (
	Cluster = "firedoor-rh-3c138b60a075"
	Context = "kind-" + Cluster
	Schema  = "firedoor.v10.fd004-live-readonly-preflight.v1"
	Status  = "live_disposable_cluster_preflight_observed_no_apply"
)

var portPattern = regexp.MustCompile(`^[0-9]{2,5}$`)

type Container struct {
	Name         string `json:"name"`
	ClusterLabel string `json:"cluster_label"`
	Running      bool   `json:"running"`
}

type Node struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

type Snapshot struct {
	Cluster                            string      `json:"cluster"`
	Context                            string      `json:"context"`
	KindClusters                       []string    `json:"kind_clusters"`
	Containers                         []Container `json:"containers"`
	ControlPlanePort                   string      `json:"control_plane_port"`
	KubeconfigContext                  string      `json:"kubeconfig_context"`
	KubeconfigServer                   string      `json:"kubeconfig_server"`
	PrivateKubeconfigHashMatched       bool        `json:"private_kubeconfig_hash_matched"`
	Nodes                              []Node      `json:"nodes"`
	CalicoDesired                      int         `json:"calico_desired"`
	CalicoReady                        int         `json:"calico_ready"`
	CalicoControllersReady             int         `json:"calico_controllers_ready"`
	CoreDNSReady                       int         `json:"coredns_ready"`
	CoreDNSCount                       int         `json:"coredns_count"`
	Namespaces                         []string    `json:"namespaces"`
	DefaultKubeconfigUnchanged         bool        `json:"default_kubeconfig_unchanged"`
	StagedTerraformLockfileHashMatched bool        `json:"staged_terraform_lockfile_hash_matched"`
	LocalStateFilesFound               bool        `json:"local_state_files_found"`
	TerraformStateVerified             bool        `json:"terraform_state_verified"`
	FreshPlanBoundToState              bool        `json:"fresh_plan_bound_to_state"`
	ApplyAuthorized                    bool        `json:"apply_authorized"`
	ApplyExecuted                      bool        `json:"apply_executed"`
}

type Report struct {
	SchemaVersion                 string `json:"schema_version"`
	Status                        string `json:"status"`
	Cluster                       string `json:"cluster"`
	Context                       string `json:"context"`
	CurrentClusterAuthenticated   bool   `json:"current_cluster_authenticated"`
	CurrentClusterHealthy         bool   `json:"current_cluster_healthy"`
	CurrentTerraformStateVerified bool   `json:"current_terraform_state_verified"`
	FreshPlanBoundToState         bool   `json:"fresh_plan_bound_to_state"`
	ApplyAuthorized               bool   `json:"apply_authorized"`
	ApplyExecuted                 bool   `json:"apply_executed"`
	DeploymentBlocked             bool   `json:"deployment_blocked"`
	RecoveryProven                bool   `json:"recovery_proven"`
	NextBoundary                  string `json:"next_boundary"`
}

func checkJSONValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return errors.New("duplicate or malformed JSON property")
			}
			seen[key] = true
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return errors.New("invalid JSON object")
		}
	case '[':
		for d.More() {
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return errors.New("invalid JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func Decode(raw []byte) (Snapshot, error) {
	var s Snapshot
	if len(raw) == 0 || len(raw) > 256*1024 || bytes.IndexByte(raw, 0) >= 0 {
		return s, errors.New("missing or unbounded snapshot")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := checkJSONValue(d); err != nil {
		return s, err
	}
	if _, err := d.Token(); err != io.EOF {
		return s, errors.New("trailing JSON after snapshot")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, err
	}
	if _, err := d.Token(); err != io.EOF {
		return s, errors.New("unexpected extra JSON")
	}
	return s, nil
}

func sameExact(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	a, e := append([]string{}, actual...), append([]string{}, expected...)
	sort.Strings(a)
	sort.Strings(e)
	for i := range a {
		if a[i] != e[i] {
			return false
		}
	}
	return true
}

func Evaluate(s Snapshot) (Report, error) {
	var empty Report
	if s.Cluster != Cluster || s.Context != Context || s.KubeconfigContext != Context {
		return empty, errors.New("cluster or kubeconfig context mismatch")
	}
	if len(s.KindClusters) < 2 || len(s.KindClusters) > 20 {
		return empty, errors.New("unexpected KIND inventory size")
	}
	clusters := map[string]bool{}
	for _, name := range s.KindClusters {
		if name == "" || clusters[name] {
			return empty, errors.New("duplicate or empty KIND cluster")
		}
		clusters[name] = true
	}
	if !clusters[Cluster] || !clusters["firedoor-local"] {
		return empty, errors.New("disposable or protected cluster missing")
	}
	expectedContainers := []string{Cluster + "-control-plane", Cluster + "-worker"}
	names := make([]string, 0, len(s.Containers))
	for _, c := range s.Containers {
		if !c.Running || c.ClusterLabel != Cluster {
			return empty, errors.New("stopped or unowned Docker container")
		}
		names = append(names, c.Name)
	}
	if !sameExact(names, expectedContainers) {
		return empty, errors.New("unexpected Docker container inventory")
	}
	if !portPattern.MatchString(s.ControlPlanePort) {
		return empty, errors.New("invalid owned control plane port")
	}
	port, err := strconv.Atoi(s.ControlPlanePort)
	if err != nil || port < 1024 || port > 65535 {
		return empty, errors.New("control plane port out of range")
	}
	if s.KubeconfigServer != fmt.Sprintf("https://127.0.0.1:%d", port) {
		return empty, errors.New("private API endpoint not owned by retained control plane")
	}
	if !s.PrivateKubeconfigHashMatched || !s.DefaultKubeconfigUnchanged || !s.StagedTerraformLockfileHashMatched {
		return empty, errors.New("private identity or immutable files changed")
	}
	nodes := make([]string, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		if !n.Ready {
			return empty, errors.New("Kubernetes node not Ready")
		}
		nodes = append(nodes, n.Name)
	}
	if !sameExact(nodes, expectedContainers) {
		return empty, errors.New("unexpected Kubernetes node inventory")
	}
	if s.CalicoDesired != 2 || s.CalicoReady != 2 || s.CalicoControllersReady != 1 || s.CoreDNSCount != 2 || s.CoreDNSReady != 2 {
		return empty, errors.New("Calico or CoreDNS not fully healthy")
	}
	if len(s.Namespaces) < 4 || len(s.Namespaces) > 250 {
		return empty, errors.New("unexpected namespace inventory")
	}
	ns := map[string]bool{}
	for _, name := range s.Namespaces {
		if name == "" || ns[name] {
			return empty, errors.New("duplicate or empty namespace")
		}
		ns[name] = true
	}
	for _, n := range []string{"default", "kube-system", "kube-public", "kube-node-lease"} {
		if !ns[n] {
			return empty, errors.New("expected Kubernetes namespace missing")
		}
	}
	for _, n := range []string{"firedoor-app", "firedoor-data", "firedoor-edge"} {
		if ns[n] {
			return empty, errors.New("managed namespace already exists: initial deployment boundary violated")
		}
	}
	if s.LocalStateFilesFound || s.TerraformStateVerified || s.FreshPlanBoundToState || s.ApplyAuthorized || s.ApplyExecuted {
		return empty, errors.New("unexpected Terraform state, plan, or apply claim")
	}
	return Report{
		SchemaVersion: Schema, Status: Status, Cluster: Cluster, Context: Context,
		CurrentClusterAuthenticated: true, CurrentClusterHealthy: true,
		CurrentTerraformStateVerified: false, FreshPlanBoundToState: false,
		ApplyAuthorized: false, ApplyExecuted: false, DeploymentBlocked: true,
		RecoveryProven: false,
		NextBoundary:   "10C3C3: separately verify Terraform state and create a fresh plan; no apply authorized",
	}, nil
}
