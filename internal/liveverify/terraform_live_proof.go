package liveverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

// FD004LiveProofSchema is the first combined Terraform/Kubernetes/HTTP
// observation contract. It is NOT a causal NetworkPolicy attestation.
const FD004LiveProofSchema = "firedoor.v10.fd004-live-proof.v1"

const fd004ProofCollectionWindow = 90 * time.Second

// FD004ConfigurationReader only exposes the existing bounded read-only
// collector; it cannot apply or change Terraform-managed resources.
type FD004ConfigurationReader interface {
	Collect(context.Context, Profile) (FD004LiveCollection, error)
}

// FD004TrafficRunner is implemented by the existing owned-Pod HTTP executor.
// Its Run invokes only fixed, previously reviewed six-route probes.
type FD004TrafficRunner interface {
	Run(context.Context, Plan) (SuiteObservation, error)
}

type FD004LiveProver struct {
	Configuration FD004ConfigurationReader
	Traffic       FD004TrafficRunner
}

type FD004LiveProof struct {
	SchemaVersion          string              `json:"schema_version"`
	Scenario               string              `json:"scenario"`
	Cluster                string              `json:"cluster"`
	Context                string              `json:"context"`
	Profile                Profile             `json:"profile"`
	Before                 FD004LiveCollection `json:"before"`
	Observation            SuiteObservation    `json:"observation"`
	After                  FD004LiveCollection `json:"after"`
	Bridge                 FD004BridgeReport   `json:"bridge"`
	Status                 string              `json:"status"`
	TrafficVerified        bool                `json:"traffic_verified"`
	TerraformAuthenticated bool                `json:"terraform_authenticated"`
	PolicyCausalityProven  bool                `json:"policy_causality_proven"`
	Limitations            string              `json:"limitations"`
}

// validateFD004Collection never trusts a supplied collection simply because
// its JSON status claims "consistent". All critical offline contract fields
// are checked again before permitting network probes or accepting a result.
func validateFD004Collection(c FD004LiveCollection, profile Profile) error {
	if c.SchemaVersion != FD004LiveCollectionSchema || c.Scenario != "FD-004" ||
		c.Cluster != OwnedCluster || c.Context != OwnedContext || c.Profile != profile ||
		c.TerraformRoot != FD004TerraformRoot || c.CapturedAtUTC.IsZero() ||
		c.TerraformAuthenticated || c.TrafficVerified || !c.StateAddressInventoryChecked {
		return errors.New("configuration collection has an invalid identity or unsupported attestation claim")
	}
	want := fd004ExpectedStateAddresses()
	if !reflect.DeepEqual(c.TerraformStateAddresses, want) {
		return errors.New("configuration collection has unexpected Terraform state addresses")
	}
	if err := parseTerraformFD004Outputs(c.TerraformOutputs, profile); err != nil {
		return fmt.Errorf("configuration collection has invalid Terraform outputs: %w", err)
	}
	digest := sha256.Sum256(c.TerraformOutputs)
	if c.TerraformOutputsSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("configuration collection output digest mismatch")
	}
	if c.KubernetesSnapshot.Cluster != OwnedCluster || c.KubernetesSnapshot.Context != OwnedContext ||
		c.KubernetesSnapshot.SchemaVersion != kubeinspect.Schema || c.KubernetesSnapshot.CapturedAtUTC.IsZero() {
		return errors.New("configuration collection Kubernetes identity is invalid")
	}
	if delta := c.CapturedAtUTC.Sub(c.KubernetesSnapshot.CapturedAtUTC); delta < 0 || delta > 2*time.Minute {
		return errors.New("configuration collection Kubernetes snapshot is stale or future-dated")
	}
	policyStatus, err := fd004PolicyStatus(c.KubernetesSnapshot, profile)
	if err != nil {
		return fmt.Errorf("configuration collection policies invalid: %w", err)
	}
	if policyStatus != "consistent" || c.PolicyStatus != "consistent" ||
		c.Status != "configuration_consistent_not_behaviorally_verified" {
		return errors.New("configuration drift or inconsistent status: no healthy security proof permitted")
	}
	return nil
}

// matchFD004Pods requires the owned Pod names captured by the read-only
// inventory to be identical to the Pods used by the live HTTP executor.
// The executor additionally verifies source/target UID and IP before exec.
func matchFD004Pods(snapshot kubeinspect.Snapshot, inv Inventory) error {
	if err := inv.Validate(); err != nil {
		return err
	}
	if len(snapshot.Zones) != 3 {
		return errors.New("incomplete three-zone Kubernetes inventory")
	}
	seen := map[Zone]bool{}
	for _, zone := range snapshot.Zones {
		z := Zone(zone.Name)
		if z != ZoneEdge && z != ZoneApp && z != ZoneData {
			return errors.New("unknown zone in Kubernetes snapshot")
		}
		if seen[z] {
			return errors.New("duplicate zone in Kubernetes snapshot")
		}
		seen[z] = true
		if zone.Namespace != "firedoor-"+zone.Name || len(zone.Pods) != 1 || !zone.Pods[0].Ready || zone.Pods[0].Phase != "Running" {
			return fmt.Errorf("owned workload Pod in %s missing or not Ready", zone.Name)
		}
		pod, err := inv.pod(z)
		if err != nil || pod.PodName != zone.Pods[0].Name || pod.Namespace != zone.Namespace {
			return fmt.Errorf("owned workload Pod in %s changed during capture", zone.Name)
		}
		if !ownedLabels(zone.Pods[0].Labels, z) || zone.Pods[0].Labels["app.kubernetes.io/name"] != "firedoor-"+zone.Name {
			return fmt.Errorf("owned workload labels in %s invalid", zone.Name)
		}
	}
	return nil
}

// Run performs non-mutating configuration reads around a bounded HTTP suite.
// Kubernetes exec runs ONLY fixed wget probes inside three owned demo-http
// Pods. Neither Terraform nor Kubernetes configuration is changed here.
// An inconclusive or mismatching result is serialized transparently; it is
// NEVER promoted to a matching/healthy result.
func (p FD004LiveProver) Run(ctx context.Context, profile Profile) (FD004LiveProof, error) {
	plan, err := NewPlan(profile)
	if err != nil {
		return FD004LiveProof{}, err
	}
	if p.Configuration == nil || p.Traffic == nil {
		return FD004LiveProof{}, errors.New("live configuration reader or traffic runner missing")
	}

	before, err := p.Configuration.Collect(ctx, profile)
	if err != nil {
		return FD004LiveProof{}, fmt.Errorf("before-traffic read-only inventory failed: %w", err)
	}
	if err := validateFD004Collection(before, profile); err != nil {
		return FD004LiveProof{}, fmt.Errorf("before-traffic gate refused: %w", err)
	}

	suite, err := p.Traffic.Run(ctx, plan)
	if err != nil {
		return FD004LiveProof{}, fmt.Errorf("bounded owned-Pod HTTP suite failed: %w", err)
	}
	// Verify before performing another collection. Invalid observations are
	// not considered network denials or evidence of an effective boundary.
	if _, err := Verify(plan, suite); err != nil {
		return FD004LiveProof{}, fmt.Errorf("live HTTP observation rejected: %w", err)
	}

	after, err := p.Configuration.Collect(ctx, profile)
	if err != nil {
		return FD004LiveProof{}, fmt.Errorf("after-traffic read-only inventory failed: %w", err)
	}
	if err := validateFD004Collection(after, profile); err != nil {
		return FD004LiveProof{}, fmt.Errorf("after-traffic gate refused: %w", err)
	}

	if suite.StartedAtUTC.Before(before.CapturedAtUTC) || suite.StartedAtUTC.Sub(before.CapturedAtUTC) > fd004ProofCollectionWindow ||
		after.CapturedAtUTC.Before(suite.FinishedAtUTC) || after.CapturedAtUTC.Sub(suite.FinishedAtUTC) > fd004ProofCollectionWindow ||
		!after.CapturedAtUTC.After(before.CapturedAtUTC) {
		return FD004LiveProof{}, errors.New("configuration and HTTP observations not contemporaneous")
	}
	if !bytes.Equal(before.TerraformOutputs, after.TerraformOutputs) ||
		before.TerraformOutputsSHA256 != after.TerraformOutputsSHA256 ||
		!reflect.DeepEqual(before.TerraformStateAddresses, after.TerraformStateAddresses) ||
		!reflect.DeepEqual(before.KubernetesSnapshot.Zones, after.KubernetesSnapshot.Zones) {
		return FD004LiveProof{}, errors.New("Terraform outputs or Kubernetes-owned resources changed across HTTP suite")
	}
	if err := matchFD004Pods(before.KubernetesSnapshot, suite.Inventory); err != nil {
		return FD004LiveProof{}, fmt.Errorf("before-probe owned workload mismatch: %w", err)
	}
	if err := matchFD004Pods(after.KubernetesSnapshot, suite.Inventory); err != nil {
		return FD004LiveProof{}, fmt.Errorf("after-probe owned workload mismatch: %w", err)
	}

	bridge, err := AssembleFD004Bridge(plan, before.TerraformOutputs, before.KubernetesSnapshot, suite)
	if err != nil {
		return FD004LiveProof{}, fmt.Errorf("FD-004 evidence bridge rejected: %w", err)
	}
	status := "behavior_inconclusive"
	if bridge.Status == "consistent" && bridge.RoutesMatched == 6 && bridge.RoutesMismatch == 0 && bridge.RoutesInconclusive == 0 {
		status = "observed_behavior_matched_cause_unproven"
	} else if bridge.Status == "mismatch" {
		status = "behavior_mismatch"
	} else if bridge.Status != "inconclusive" {
		return FD004LiveProof{}, errors.New("unexpected FD-004 bridge status")
	}
	return FD004LiveProof{
		SchemaVersion: FD004LiveProofSchema, Scenario: "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		Profile: profile, Before: before, Observation: suite, After: after, Bridge: bridge,
		Status: status, TrafficVerified: status == "observed_behavior_matched_cause_unproven",
		TerraformAuthenticated: false, PolicyCausalityProven: false,
		Limitations: "LOCAL LAB ONLY. Fixed direct Pod-IP HTTP behavior was observed inside three owned kind workloads. Consistent behavior does not independently prove Calico or NetworkPolicy causality. Local Terraform state/output readings are not authenticated, refreshed, or proof of Terraform apply causality. No configuration or Terraform mutation was performed; kubectl exec ran bounded wget probes inside owned Pods. Not production monitoring or a compliance certification.",
	}, nil
}
