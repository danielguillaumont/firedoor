package liveverify

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// FD004RecoveryEvidenceSchema represents an offline comparison of three saved
// observations. It is not a receipt for an authorized Terraform mutation or a
// causal CNI/NetworkPolicy enforcement attestation.
const FD004RecoveryEvidenceSchema = "firedoor.v10.fd004-recovery-evidence.v1"

// FD004RecoveryRoute summarizes a fixed, cross-profile HTTP observation.
type FD004RecoveryRoute struct {
	ID               string      `json:"id"`
	InitialHardened  Observation `json:"initial_hardened"`
	Baseline         Observation `json:"baseline"`
	RestoredHardened Observation `json:"restored_hardened"`
	Classification   string      `json:"classification"`
}

type FD004RecoveryEvidenceReport struct {
	SchemaVersion                      string               `json:"schema_version"`
	Scenario                           string               `json:"scenario"`
	Cluster                            string               `json:"cluster"`
	Context                            string               `json:"context"`
	InitialFinishedAtUTC               time.Time            `json:"initial_finished_at_utc"`
	BaselineFinishedAtUTC              time.Time            `json:"baseline_finished_at_utc"`
	RestoredStartedAtUTC               time.Time            `json:"restored_started_at_utc"`
	ComparedRoutes                     int                  `json:"compared_routes"`
	ReversedAndRestoredProtectedRoutes int                  `json:"reversed_and_restored_protected_routes"`
	PreservedAllowedRoutes             int                  `json:"preserved_allowed_routes"`
	Routes                             []FD004RecoveryRoute `json:"routes"`
	Status                             string               `json:"status"`
	TerraformAuthenticated             bool                 `json:"terraform_authenticated"`
	ApplyReceiptVerified               bool                 `json:"apply_receipt_verified"`
	PolicyCausalityProven              bool                 `json:"policy_causality_proven"`
	Limitations                        string               `json:"limitations"`
}

// ReviewFD004RecoveryEvidence compares a chronological hardened -> baseline ->
// hardened set of complete saved six-route proofs. All three are independently
// revalidated before any readout is produced. Neither this function nor its
// CLI reads kubeconfig, invokes kubectl, or modifies the lab.
func ReviewFD004RecoveryEvidence(initial, baseline, restored FD004LiveProof) (FD004RecoveryEvidenceReport, error) {
	for _, step := range []struct {
		name    string
		proof   FD004LiveProof
		profile Profile
	}{
		{"initial hardened", initial, ProfileHardened},
		{"baseline", baseline, ProfileBaseline},
		{"restored hardened", restored, ProfileHardened},
	} {
		if err := validateFD004TransitionInput(step.proof, step.profile); err != nil {
			return FD004RecoveryEvidenceReport{}, fmt.Errorf("%s proof refused: %w", step.name, err)
		}
	}
	// The existing two-profile comparison independently checks policy-only
	// changes, workload identity, timestamps, and each route's raw attempts.
	transition, err := CompareFD004Transition(baseline, restored)
	if err != nil {
		return FD004RecoveryEvidenceReport{}, fmt.Errorf("baseline-to-restored transition refused: %w", err)
	}
	if transition.Status != "observed_transition_cause_unproven" || transition.ComparedRoutes != 6 {
		return FD004RecoveryEvidenceReport{}, errors.New("invalid baseline-to-restored transition")
	}
	if !initial.After.CapturedAtUTC.Before(baseline.Before.CapturedAtUTC) ||
		!baseline.After.CapturedAtUTC.Before(restored.Before.CapturedAtUTC) ||
		!initial.Observation.FinishedAtUTC.Before(baseline.Observation.StartedAtUTC) ||
		!baseline.Observation.FinishedAtUTC.Before(restored.Observation.StartedAtUTC) ||
		baseline.Observation.StartedAtUTC.Sub(initial.Observation.FinishedAtUTC) > fd004MaxPairGap ||
		restored.Observation.StartedAtUTC.Sub(baseline.Observation.FinishedAtUTC) > fd004MaxPairGap {
		return FD004RecoveryEvidenceReport{}, errors.New("recovery proofs out of order or separated by more than 24h per phase")
	}
	// The workload and its identity cannot be replaced as a hidden variable.
	for _, z := range []Zone{ZoneEdge, ZoneApp, ZoneData} {
		before, err := initial.Observation.Inventory.pod(z)
		if err != nil {
			return FD004RecoveryEvidenceReport{}, err
		}
		middle, err := baseline.Observation.Inventory.pod(z)
		if err != nil {
			return FD004RecoveryEvidenceReport{}, err
		}
		after, err := restored.Observation.Inventory.pod(z)
		if err != nil {
			return FD004RecoveryEvidenceReport{}, err
		}
		if before != middle || middle != after {
			return FD004RecoveryEvidenceReport{}, fmt.Errorf("Pod identity or IP changed across recovery sequence: %s", z)
		}
	}
	initialZones := initial.After.KubernetesSnapshot.Zones
	baselineZones := baseline.Before.KubernetesSnapshot.Zones
	restoredZones := restored.Before.KubernetesSnapshot.Zones
	if len(initialZones) != 3 || len(baselineZones) != 3 || len(restoredZones) != 3 {
		return FD004RecoveryEvidenceReport{}, errors.New("incomplete recovery zone inventory")
	}
	if !reflect.DeepEqual(initial.After.TerraformStateAddresses, baseline.Before.TerraformStateAddresses) ||
		!reflect.DeepEqual(initial.After.TerraformStateAddresses, restored.Before.TerraformStateAddresses) ||
		!bytes.Equal(initial.After.TerraformOutputs, restored.Before.TerraformOutputs) ||
		!bytes.Equal(initial.After.TerraformOutputs, restored.After.TerraformOutputs) ||
		!reflect.DeepEqual(initialZones, restoredZones) {
		return FD004RecoveryEvidenceReport{}, errors.New("original hardened state/configuration was not restored")
	}
	for index := range initialZones {
		first, mid := initialZones[index], baselineZones[index]
		first.NetworkPolicies = nil
		mid.NetworkPolicies = nil
		if !reflect.DeepEqual(first, mid) {
			return FD004RecoveryEvidenceReport{}, fmt.Errorf("non-policy workload changed across weakening in zone %s", first.Name)
		}
	}
	result := FD004RecoveryEvidenceReport{
		SchemaVersion: FD004RecoveryEvidenceSchema,
		Scenario:      "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		InitialFinishedAtUTC:   initial.Observation.FinishedAtUTC,
		BaselineFinishedAtUTC:  baseline.Observation.FinishedAtUTC,
		RestoredStartedAtUTC:   restored.Observation.StartedAtUTC,
		Status:                 "observed_recovery_sequence_cause_unproven",
		Routes:                 make([]FD004RecoveryRoute, 0, 6),
		TerraformAuthenticated: false, ApplyReceiptVerified: false, PolicyCausalityProven: false,
		Limitations: "OFFLINE LOCAL-LAB EVIDENCE ONLY. Three saved six-route HTTP results show the expected hardened -> baseline -> hardened changes with preserved workload identity. These files are not authenticated; no Terraform plan/apply receipts or independent Calico/NetworkPolicy causality were verified. Repeated connection failures do not prove a security control blocked traffic. This review performs no Kubernetes or Terraform operations.",
	}
	protected := map[string]bool{"edge-to-data": true, "data-to-app": true}
	for i, before := range initial.Observation.Routes {
		middle := baseline.Observation.Routes[i]
		after := restored.Observation.Routes[i]
		if before.ID != middle.ID || middle.ID != after.ID || before.Source != middle.Source ||
			middle.Source != after.Source || before.Destination != middle.Destination || middle.Destination != after.Destination {
			return FD004RecoveryEvidenceReport{}, errors.New("route identity changed across recovery sequence")
		}
		route := FD004RecoveryRoute{ID: before.ID, InitialHardened: before.Observed, Baseline: middle.Observed, RestoredHardened: after.Observed}
		if protected[before.ID] {
			if before.Observed != ObservedUnreachable || middle.Observed != ObservedReachable || after.Observed != ObservedUnreachable {
				return FD004RecoveryEvidenceReport{}, fmt.Errorf("protected route %s did not reverse and recover", before.ID)
			}
			route.Classification = "reversed_and_restored"
			result.ReversedAndRestoredProtectedRoutes++
		} else {
			if before.Observed != ObservedReachable || middle.Observed != ObservedReachable || after.Observed != ObservedReachable {
				return FD004RecoveryEvidenceReport{}, fmt.Errorf("allowed route %s lost reachability", before.ID)
			}
			route.Classification = "reachability_preserved"
			result.PreservedAllowedRoutes++
		}
		result.Routes = append(result.Routes, route)
	}
	result.ComparedRoutes = len(result.Routes)
	if result.ComparedRoutes != 6 || result.ReversedAndRestoredProtectedRoutes != 2 || result.PreservedAllowedRoutes != 4 {
		return FD004RecoveryEvidenceReport{}, errors.New("recovery evidence does not contain the exact six-route matrix")
	}
	return result, nil
}
