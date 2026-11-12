// Package recoverypostapply verifies read-only, post-deployment observations.
// It never initiates Terraform or Kubernetes operations.
package recoverypostapply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const Cluster = "firedoor-rh-3c138b60a075"
const PlanSHA = "0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7"
const ResultSHA = "1E390B0301308C9C646AFA10896E5282E185C1ADA8914559C7A25CAF4A37BD12"

type Zone struct {
	Zone                   string `json:"zone"`
	Namespace              string `json:"namespace"`
	NamespaceVerified      bool   `json:"namespace_verified"`
	ServiceAccountVerified bool   `json:"service_account_verified"`
	DeploymentReady        bool   `json:"deployment_ready"`
	PodReady               bool   `json:"pod_ready"`
	ServiceVerified        bool   `json:"service_verified"`
	EndpointReady          bool   `json:"endpoint_ready"`
	NetworkPolicyVerified  bool   `json:"network_policy_verified"`
	NetworkPolicyCount     int    `json:"network_policy_count"`
}

type Observation struct {
	SchemaVersion             string   `json:"schema_version"`
	Cluster                   string   `json:"cluster"`
	Context                   string   `json:"context"`
	BackendType               string   `json:"backend_type"`
	BackendStatePath          string   `json:"backend_state_path"`
	Workspace                 string   `json:"workspace"`
	ApplyIntentHashVerified   bool     `json:"apply_intent_hash_verified"`
	ApplyResultHashVerified   bool     `json:"apply_result_hash_verified"`
	ApplyExitCode             int      `json:"apply_exit_code"`
	StateHashVerified         bool     `json:"state_hash_verified"`
	SavedPlanHashVerified     bool     `json:"saved_plan_hash_verified"`
	ReviewReceiptHashVerified bool     `json:"review_receipt_hash_verified"`
	KubeconfigHashVerified    bool     `json:"kubeconfig_hash_verified"`
	NodesReady                bool     `json:"nodes_ready"`
	CalicoReady               bool     `json:"calico_ready"`
	CoreDNSReady              bool     `json:"coredns_ready"`
	StateSerial               int      `json:"state_serial"`
	StateLineage              string   `json:"state_lineage"`
	TerraformAddresses        []string `json:"terraform_addresses"`
	Zones                     []Zone   `json:"zones"`
}

type Result struct {
	SchemaVersion                     string `json:"schema_version"`
	Status                            string `json:"status"`
	Cluster                           string `json:"cluster"`
	TerraformAddressesVerified        int    `json:"terraform_addresses_verified"`
	WorkloadZonesReady                int    `json:"workload_zones_ready"`
	IngressNetworkPoliciesVerified    int    `json:"ingress_network_policies_verified"`
	ManagedStateInventoryVerified     bool   `json:"managed_state_inventory_verified"`
	LiveWorkloadsVerified             bool   `json:"live_workloads_verified"`
	IngressPolicyShapeVerified        bool   `json:"ingress_policy_shape_verified"`
	NetworkTrafficEnforcementVerified bool   `json:"network_traffic_enforcement_verified"`
	ApplyExecutedByVerifier           bool   `json:"apply_executed_by_verifier"`
}

func ExpectedAddresses() []string {
	v := make([]string, 0, 14)
	for _, zone := range []string{"app", "data", "edge"} {
		for _, typ := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			v = append(v, fmt.Sprintf("module.workloads[\"%s\"].%s.this", zone, typ))
		}
	}
	for _, zone := range []string{"app", "data"} {
		v = append(v, fmt.Sprintf("module.network_boundaries[\"%s\"].kubernetes_network_policy_v1.this", zone))
	}
	sort.Strings(v)
	return v
}

func Evaluate(o Observation) (Result, error) {
	reject := func(s string) (Result, error) {
		return Result{}, fmt.Errorf("F4 post-apply verification rejected: %s", s)
	}
	if o.SchemaVersion != "firedoor.v10.fd004-post-apply-observation.v1" || o.Cluster != Cluster || o.Context != "kind-"+Cluster || o.BackendType != "local" || o.BackendStatePath != "firedoor-owned.tfstate" || o.Workspace != "default" {
		return reject("wrong schema, cluster, context, backend or workspace")
	}
	if !o.ApplyIntentHashVerified || !o.ApplyResultHashVerified || o.ApplyExitCode != 0 || !o.StateHashVerified || !o.SavedPlanHashVerified || !o.ReviewReceiptHashVerified || !o.KubeconfigHashVerified {
		return reject("apply or private-evidence integrity was not confirmed")
	}
	if o.StateSerial < 1 || len(o.StateLineage) < 32 || len(o.StateLineage) > 64 || strings.ContainsAny(o.StateLineage, " /\\\n") {
		return reject("unexpected Terraform state serial or lineage")
	}
	if !o.NodesReady || !o.CalicoReady || !o.CoreDNSReady {
		return reject("cluster infrastructure is not ready")
	}
	expected := ExpectedAddresses()
	addresses := append([]string{}, o.TerraformAddresses...)
	sort.Strings(addresses)
	if len(addresses) != len(expected) {
		return reject("Terraform state must contain exactly 14 managed resource addresses")
	}
	for i := range expected {
		if expected[i] != addresses[i] {
			return reject("Terraform state contains a missing or unexpected address")
		}
		if i > 0 && addresses[i] == addresses[i-1] {
			return reject("duplicate Terraform address")
		}
	}
	if len(o.Zones) != 3 {
		return reject("expected exactly three zone observations")
	}
	seen := make(map[string]bool, 3)
	nPolicies := 0
	for _, z := range o.Zones {
		if z.Zone != "edge" && z.Zone != "app" && z.Zone != "data" {
			return reject("unexpected zone")
		}
		if seen[z.Zone] || z.Namespace != "firedoor-"+z.Zone {
			return reject("wrong or duplicate namespace")
		}
		seen[z.Zone] = true
		if !z.NamespaceVerified || !z.ServiceAccountVerified || !z.DeploymentReady || !z.PodReady || !z.ServiceVerified || !z.EndpointReady || !z.NetworkPolicyVerified {
			return reject("missing or unready live Kubernetes resource in " + z.Zone)
		}
		wantPolicies := 1
		if z.Zone == "edge" {
			wantPolicies = 0
		}
		if z.NetworkPolicyCount != wantPolicies {
			return reject("unexpected NetworkPolicy count or zone")
		}
		nPolicies += z.NetworkPolicyCount
	}
	if nPolicies != 2 {
		return reject("expected exactly two managed ingress policies")
	}
	return Result{SchemaVersion: "firedoor.v10.fd004-post-apply-result.v1", Status: "read_only_state_and_live_resource_verification_passed", Cluster: Cluster, TerraformAddressesVerified: 14, WorkloadZonesReady: 3, IngressNetworkPoliciesVerified: 2, ManagedStateInventoryVerified: true, LiveWorkloadsVerified: true, IngressPolicyShapeVerified: true, NetworkTrafficEnforcementVerified: false, ApplyExecutedByVerifier: false}, nil
}

func Decode(r io.Reader) (Observation, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1<<20+1))
	if err != nil {
		return Observation{}, err
	}
	if len(data) > 1<<20 {
		return Observation{}, fmt.Errorf("observation exceeds limit")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}) // PowerShell 5.1 pipeline may add one UTF-8 BOM.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var o Observation
	if err := dec.Decode(&o); err != nil {
		return Observation{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Observation{}, fmt.Errorf("trailing input or invalid JSON: %v", err)
	}
	return o, nil
}
