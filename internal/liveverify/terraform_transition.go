package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// FD004TransitionSchema identifies a comparison of two already recorded local
// observations. This is neither an apply receipt nor an enforcement attestation.
const FD004TransitionSchema = "firedoor.v10.fd004-transition.v1"
const fd004MaxPairGap = 24 * time.Hour
const fd004MaxProofFileBytes = 8 << 20

type FD004RouteTransition struct {
	ID             string      `json:"id"`
	Source         Zone        `json:"source"`
	Destination    Zone        `json:"destination"`
	Baseline       Observation `json:"baseline"`
	Hardened       Observation `json:"hardened"`
	Classification string      `json:"classification"`
}

type FD004TransitionReport struct {
	SchemaVersion          string                 `json:"schema_version"`
	Scenario               string                 `json:"scenario"`
	Cluster                string                 `json:"cluster"`
	Context                string                 `json:"context"`
	BaselineFinishedAtUTC  time.Time              `json:"baseline_finished_at_utc"`
	HardenedStartedAtUTC   time.Time              `json:"hardened_started_at_utc"`
	ComparedRoutes         int                    `json:"compared_routes"`
	ChangedProtectedRoutes int                    `json:"changed_protected_routes"`
	PreservedAllowedRoutes int                    `json:"preserved_allowed_routes"`
	Routes                 []FD004RouteTransition `json:"routes"`
	Status                 string                 `json:"status"`
	TerraformAuthenticated bool                   `json:"terraform_authenticated"`
	PolicyCausalityProven  bool                   `json:"policy_causality_proven"`
	ApplyReceiptVerified   bool                   `json:"apply_receipt_verified"`
	Limitations            string                 `json:"limitations"`
}

// validateFD004TransitionInput independently rechecks the saved proof contract
// rather than treating its status flag as trustworthy. Raw local JSON remains
// operator-controlled and is NOT independently authenticated by this function.
func validateFD004TransitionInput(p FD004LiveProof, profile Profile) error {
	if p.SchemaVersion != FD004LiveProofSchema || p.Scenario != "FD-004" ||
		p.Cluster != OwnedCluster || p.Context != OwnedContext || p.Profile != profile ||
		p.Status != "observed_behavior_matched_cause_unproven" || !p.TrafficVerified ||
		p.TerraformAuthenticated || p.PolicyCausalityProven || strings.TrimSpace(p.Limitations) == "" {
		return errors.New("incomplete or unsupported saved proof claim")
	}
	if err := validateFD004Collection(p.Before, profile); err != nil {
		return fmt.Errorf("before configuration rejected: %w", err)
	}
	if err := validateFD004Collection(p.After, profile); err != nil {
		return fmt.Errorf("after configuration rejected: %w", err)
	}
	if p.Observation.StartedAtUTC.Before(p.Before.CapturedAtUTC) ||
		p.Observation.StartedAtUTC.Sub(p.Before.CapturedAtUTC) > fd004ProofCollectionWindow ||
		p.After.CapturedAtUTC.Before(p.Observation.FinishedAtUTC) ||
		p.After.CapturedAtUTC.Sub(p.Observation.FinishedAtUTC) > fd004ProofCollectionWindow ||
		!p.After.CapturedAtUTC.After(p.Before.CapturedAtUTC) {
		return errors.New("saved proof has invalid collection/observation chronology")
	}
	if !bytes.Equal(p.Before.TerraformOutputs, p.After.TerraformOutputs) ||
		p.Before.TerraformOutputsSHA256 != p.After.TerraformOutputsSHA256 ||
		!reflect.DeepEqual(p.Before.TerraformStateAddresses, p.After.TerraformStateAddresses) ||
		!reflect.DeepEqual(p.Before.KubernetesSnapshot.Zones, p.After.KubernetesSnapshot.Zones) {
		return errors.New("configuration changed during saved route proof")
	}
	if err := matchFD004Pods(p.Before.KubernetesSnapshot, p.Observation.Inventory); err != nil {
		return fmt.Errorf("before workload identity rejected: %w", err)
	}
	if err := matchFD004Pods(p.After.KubernetesSnapshot, p.Observation.Inventory); err != nil {
		return fmt.Errorf("after workload identity rejected: %w", err)
	}
	plan, err := NewPlan(profile)
	if err != nil {
		return err
	}
	verified, err := Verify(plan, p.Observation)
	if err != nil || verified.Status != "matched" || verified.Summary.Match != 6 ||
		verified.Summary.Mismatch != 0 || verified.Summary.Inconclusive != 0 {
		return errors.New("saved proof observations do not match their declared profile")
	}
	bridge, err := AssembleFD004Bridge(plan, p.Before.TerraformOutputs, p.Before.KubernetesSnapshot, p.Observation)
	if err != nil || !reflect.DeepEqual(bridge, p.Bridge) || bridge.Status != "consistent" || bridge.RoutesMatched != 6 {
		return errors.New("saved Terraform/policy/probe bridge disagrees with source evidence")
	}
	return nil
}

// CompareFD004Transition compares a baseline and hardened proof that were each
// independently validated. It verifies the two expected denied transitions
// while preserving the other four allowed routes. It intentionally CANNOT prove
// Terraform apply causality or authenticate separately saved evidence files.
func CompareFD004Transition(baseline, hardened FD004LiveProof) (FD004TransitionReport, error) {
	if err := validateFD004TransitionInput(baseline, ProfileBaseline); err != nil {
		return FD004TransitionReport{}, fmt.Errorf("baseline proof refused: %w", err)
	}
	if err := validateFD004TransitionInput(hardened, ProfileHardened); err != nil {
		return FD004TransitionReport{}, fmt.Errorf("hardened proof refused: %w", err)
	}
	if !baseline.After.CapturedAtUTC.Before(hardened.Before.CapturedAtUTC) ||
		!baseline.Observation.FinishedAtUTC.Before(hardened.Observation.StartedAtUTC) ||
		hardened.Observation.StartedAtUTC.Sub(baseline.Observation.FinishedAtUTC) > fd004MaxPairGap {
		return FD004TransitionReport{}, errors.New("proofs not ordered as baseline then hardened, or more than 24h apart")
	}
	// These Terraform profiles alter only the two ingress policies; a changed
	// workload, service, or namespace invalidates a like-for-like comparison.
	baseZones := baseline.After.KubernetesSnapshot.Zones
	hardZones := hardened.Before.KubernetesSnapshot.Zones
	if len(baseZones) != 3 || len(hardZones) != 3 {
		return FD004TransitionReport{}, errors.New("incomplete zone inventory")
	}
	for i := range baseZones {
		b, h := baseZones[i], hardZones[i]
		b.NetworkPolicies, h.NetworkPolicies = nil, nil
		if !reflect.DeepEqual(b, h) {
			return FD004TransitionReport{}, fmt.Errorf("non-policy resource changed across profiles in zone %s", b.Name)
		}
	}
	for _, zone := range []Zone{ZoneEdge, ZoneApp, ZoneData} {
		b, err := baseline.Observation.Inventory.pod(zone)
		if err != nil {
			return FD004TransitionReport{}, err
		}
		h, err := hardened.Observation.Inventory.pod(zone)
		if err != nil {
			return FD004TransitionReport{}, err
		}
		if b != h {
			return FD004TransitionReport{}, fmt.Errorf("Pod identity or IP changed across profile switch for zone %s", zone)
		}
	}
	if reflect.DeepEqual(baseline.After.KubernetesSnapshot.Zones, hardened.Before.KubernetesSnapshot.Zones) ||
		bytes.Equal(baseline.After.TerraformOutputs, hardened.Before.TerraformOutputs) {
		return FD004TransitionReport{}, errors.New("profile transition has no distinct configuration evidence")
	}

	report := FD004TransitionReport{
		SchemaVersion: FD004TransitionSchema, Scenario: "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		BaselineFinishedAtUTC: baseline.Observation.FinishedAtUTC, HardenedStartedAtUTC: hardened.Observation.StartedAtUTC,
		Routes:                 make([]FD004RouteTransition, 0, 6),
		Status:                 "observed_transition_cause_unproven",
		TerraformAuthenticated: false, PolicyCausalityProven: false, ApplyReceiptVerified: false,
		Limitations: "LOCAL LAB EVIDENCE PAIR ONLY. Validated local JSON records show two routes changed from reachable to unreachable while four remained reachable. Saved files and Terraform outputs are unauthenticated; no plan/apply receipt, explicit mutation authorization, CNI causality, or enforcement attestation was verified. HTTP failures are repeated observations, not attribution to NetworkPolicy. No Terraform, Kubernetes, or network operations were performed by this comparison.",
	}
	protected := map[string]bool{"edge-to-data": true, "data-to-app": true}
	for i, route := range baseline.Observation.Routes {
		other := hardened.Observation.Routes[i]
		if route.ID != other.ID || route.Source != other.Source || route.Destination != other.Destination {
			return FD004TransitionReport{}, errors.New("route identity changed across saved observations")
		}
		step := FD004RouteTransition{ID: route.ID, Source: route.Source, Destination: route.Destination,
			Baseline: route.Observed, Hardened: other.Observed}
		if protected[route.ID] {
			if route.Observed != ObservedReachable || other.Observed != ObservedUnreachable {
				return FD004TransitionReport{}, fmt.Errorf("protected route %s did not transition reachable-to-unreachable", route.ID)
			}
			step.Classification = "newly_unreachable"
			report.ChangedProtectedRoutes++
		} else {
			if route.Observed != ObservedReachable || other.Observed != ObservedReachable {
				return FD004TransitionReport{}, fmt.Errorf("allowed route %s was not preserved", route.ID)
			}
			step.Classification = "reachability_preserved"
			report.PreservedAllowedRoutes++
		}
		report.Routes = append(report.Routes, step)
	}
	report.ComparedRoutes = len(report.Routes)
	if report.ComparedRoutes != 6 || report.ChangedProtectedRoutes != 2 || report.PreservedAllowedRoutes != 4 {
		return FD004TransitionReport{}, errors.New("incomplete expected transition matrix")
	}
	return report, nil
}

// ReadFD004ProofFile accepts bounded local JSON for offline comparison only.
// A successful decode does not authenticate the file or its recorded events.
func ReadFD004ProofFile(path string) (FD004LiveProof, error) {
	if strings.TrimSpace(path) == "" {
		return FD004LiveProof{}, errors.New("empty proof file path")
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return FD004LiveProof{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() == 0 || stat.Size() > fd004MaxProofFileBytes {
		return FD004LiveProof{}, errors.New("proof JSON must be a nonempty regular file <=8 MiB")
	}
	raw, err := io.ReadAll(io.LimitReader(f, fd004MaxProofFileBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > fd004MaxProofFileBytes {
		return FD004LiveProof{}, errors.New("could not read bounded proof JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var proof FD004LiveProof
	if err := decoder.Decode(&proof); err != nil {
		return FD004LiveProof{}, fmt.Errorf("decode proof JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return FD004LiveProof{}, errors.New("trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return FD004LiveProof{}, fmt.Errorf("trailing JSON: %w", err)
	}
	return proof, nil
}
