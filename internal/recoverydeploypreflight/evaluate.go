// Package recoverydeploypreflight evaluates a read-only FD-004 observation.
// An accepted observation is eligible for a SEPARATE deployment review only.
// Nothing in this package authorizes apply, destroy, or a recovery claim.
package recoverydeploypreflight

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	Schema       = "firedoor.v10.fd004-deployment-preflight-observation.v1"
	ReportSchema = "firedoor.v10.fd004-deployment-preflight-result.v1"
	Cluster      = "firedoor-rh-3c138b60a075"
	Context      = "kind-" + Cluster
)

type Node struct {
	Name         string `json:"name"`
	Running      bool   `json:"running"`
	Ready        bool   `json:"ready"`
	ClusterLabel string `json:"cluster_label"`
}

type Observation struct {
	SchemaVersion                string   `json:"schema_version"`
	Cluster                      string   `json:"cluster"`
	Context                      string   `json:"context"`
	BackendType                  string   `json:"backend_type"`
	BackendStatePath             string   `json:"backend_state_path"`
	Workspace                    string   `json:"workspace"`
	VerifiedEvidence             bool     `json:"verified_evidence"`
	SavedBinaryMatchesJSON       bool     `json:"saved_binary_matches_json"`
	SourceHashesVerified         int      `json:"source_hashes_verified"`
	PlannedCreates               int      `json:"planned_creates"`
	PlannedUpdates               int      `json:"planned_updates"`
	PlannedDestroys              int      `json:"planned_destroys"`
	IngressPolicies              int      `json:"ingress_policies"`
	TerraformManagedStatePresent bool     `json:"terraform_managed_state_present"`
	TerraformStateLockPresent    bool     `json:"terraform_state_lock_present"`
	KubeconfigIdentityVerified   bool     `json:"kubeconfig_identity_verified"`
	DockerNodes                  []Node   `json:"docker_nodes"`
	KubernetesNodes              []Node   `json:"kubernetes_nodes"`
	CalicoDesired                int      `json:"calico_desired"`
	CalicoReady                  int      `json:"calico_ready"`
	CalicoControllerReady        int      `json:"calico_controller_ready"`
	CoreDNSReady                 int      `json:"coredns_ready"`
	CoreDNSCount                 int      `json:"coredns_count"`
	Namespaces                   []string `json:"namespaces"`
}

type Report struct {
	SchemaVersion                             string `json:"schema_version"`
	Status                                    string `json:"status"`
	Cluster                                   string `json:"cluster"`
	Context                                   string `json:"context"`
	PlanShapeVerified                         bool   `json:"plan_shape_verified"`
	LiveClusterChecked                        bool   `json:"live_cluster_checked"`
	EligibleForSeparateDeploymentReview       bool   `json:"eligible_for_separate_deployment_review"`
	TerraformManagedResourceOwnershipVerified bool   `json:"terraform_managed_resource_ownership_verified"`
	ApplyAuthorized                           bool   `json:"apply_authorized"`
	ApplyExecuted                             bool   `json:"apply_executed"`
	RecoveryProven                            bool   `json:"recovery_proven"`
}

func evaluateNodeSet(nodes []Node, docker bool) error {
	if len(nodes) != 2 {
		return errors.New("exactly two nodes required")
	}
	required := map[string]bool{Cluster + "-control-plane": false, Cluster + "-worker": false}
	for _, n := range nodes {
		seen, ok := required[n.Name]
		if !ok || seen {
			return fmt.Errorf("unknown or duplicate node %q", n.Name)
		}
		if docker {
			if !n.Running || n.ClusterLabel != Cluster {
				return fmt.Errorf("unowned or stopped Docker node %q", n.Name)
			}
		} else if !n.Ready {
			return fmt.Errorf("Kubernetes node not Ready: %q", n.Name)
		}
		required[n.Name] = true
	}
	return nil
}

func Evaluate(o Observation) (Report, error) {
	if o.SchemaVersion != Schema || o.Cluster != Cluster || o.Context != Context {
		return Report{}, errors.New("unexpected observation schema or disposable cluster identity")
	}
	if o.BackendType != "local" || o.BackendStatePath != "firedoor-owned.tfstate" || o.Workspace != "default" {
		return Report{}, errors.New("wrong backend or Terraform workspace")
	}
	if !o.VerifiedEvidence || !o.SavedBinaryMatchesJSON || o.SourceHashesVerified != 12 {
		return Report{}, errors.New("unverified backend, source or saved plan evidence")
	}
	if o.PlannedCreates != 14 || o.PlannedUpdates != 0 || o.PlannedDestroys != 0 || o.IngressPolicies != 2 {
		return Report{}, errors.New("plan is not the admitted hardened create-only shape")
	}
	if o.TerraformManagedStatePresent || o.TerraformStateLockPresent {
		return Report{}, errors.New("state or lock now present; prior zero-state plan cannot be reviewed")
	}
	if !o.KubeconfigIdentityVerified {
		return Report{}, errors.New("private kubeconfig ownership unverified")
	}
	if err := evaluateNodeSet(o.DockerNodes, true); err != nil {
		return Report{}, err
	}
	if err := evaluateNodeSet(o.KubernetesNodes, false); err != nil {
		return Report{}, err
	}
	if o.CalicoDesired != 2 || o.CalicoReady != 2 || o.CalicoControllerReady != 1 ||
		o.CoreDNSCount != 2 || o.CoreDNSReady != 2 {
		return Report{}, errors.New("Calico or CoreDNS not healthy on the owned two-node cluster")
	}
	requiredNamespaces := map[string]bool{"default": false, "kube-system": false, "kube-public": false, "kube-node-lease": false}
	seen := make(map[string]bool)
	for _, ns := range o.Namespaces {
		if ns == "" || seen[ns] || strings.HasPrefix(strings.ToLower(ns), "firedoor-") {
			return Report{}, fmt.Errorf("duplicate/invalid or already managed namespace %q", ns)
		}
		seen[ns] = true
		if _, exists := requiredNamespaces[ns]; exists {
			requiredNamespaces[ns] = true
		}
	}
	for ns, present := range requiredNamespaces {
		if !present {
			return Report{}, fmt.Errorf("required system namespace missing: %s", ns)
		}
	}
	return Report{
		SchemaVersion:                       ReportSchema,
		Status:                              "deployment_review_preflight_passed_no_apply",
		Cluster:                             Cluster,
		Context:                             Context,
		PlanShapeVerified:                   true,
		LiveClusterChecked:                  true,
		EligibleForSeparateDeploymentReview: true,
		TerraformManagedResourceOwnershipVerified: false,
		ApplyAuthorized: false,
		ApplyExecuted:   false,
		RecoveryProven:  false,
	}, nil
}

// Decode consumes exactly one small, strict observation. No source of authority
// can be delegated through unknown JSON fields or trailing data.
func Decode(r io.Reader) (Observation, error) {
	// Windows PowerShell 5.1 can prefix piped JSON with a UTF-8 BOM.
	// Accept at most one BOM at byte zero; retain all strict JSON checks.
	input := bufio.NewReader(io.LimitReader(r, 2<<20))
	if prefix, err := input.Peek(3); err == nil && bytes.Equal(prefix, []byte{0xEF, 0xBB, 0xBF}) {
		if _, err := input.Discard(3); err != nil {
			return Observation{}, err
		}
	}
	dec := json.NewDecoder(input)
	dec.DisallowUnknownFields()
	var o Observation
	if err := dec.Decode(&o); err != nil {
		return Observation{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Observation{}, errors.New("unexpected trailing observation data")
	}
	return o, nil
}
