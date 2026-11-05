// Package attribution defines the immutable, lab-only v0.4 experiment contract.
// This package plans experiments; it does not contact Kubernetes or change resources.
package attribution

import (
	"encoding/json"
	"errors"
	"reflect"
)

const (
	SchemaVersion   = "firedoor.v04.attribution-plan.v1"
	OwnedCluster    = "firedoor-local"
	OwnedContext    = "kind-firedoor-local"
	RequiredProfile = "hardened"
)

type Phase string

const (
	Before   Phase = "before"
	Witness  Phase = "witness_present"
	Restored Phase = "restored"
)

type Outcome string

const (
	Reachable   Outcome = "reachable"
	Unreachable Outcome = "unreachable"
)

// Endpoint is a namespace-and-Pod-label identity, not a general network target.
type Endpoint struct {
	Namespace string `json:"namespace"`
	AppName   string `json:"app_name"`
}

// WitnessSpec is the intended *future* scoped temporary allow policy.
// It is not a Kubernetes manifest and cannot be applied by this package.
type WitnessSpec struct {
	PolicyName           string `json:"policy_name"`
	PolicyNamespace      string `json:"policy_namespace"`
	NamespaceSelectorKey string `json:"namespace_selector_key"`
	SourcePodSelectorKey string `json:"source_pod_selector_key"`
	TargetPodSelectorKey string `json:"target_pod_selector_key"`
	Protocol             string `json:"protocol"`
	TargetPort           int    `json:"target_port"`
}

type PhaseExpectation struct {
	Phase           Phase   `json:"phase"`
	ExpectedOutcome Outcome `json:"expected_outcome"`
}

type Experiment struct {
	ID       string             `json:"id"`
	RouteID  string             `json:"route_id"`
	Source   Endpoint           `json:"source"`
	Target   Endpoint           `json:"target"`
	Witness  WitnessSpec        `json:"witness"`
	Sequence []PhaseExpectation `json:"sequence"`
}

// Plan is intentionally closed to arbitrary clusters, profiles, routes, or manifests.
// Later runtime slices must independently verify cluster identity, ownership,
// current Kubernetes state, and explicit human consent before any mutation.
type Plan struct {
	Schema              string       `json:"schema"`
	Cluster             string       `json:"cluster"`
	Context             string       `json:"context"`
	Profile             string       `json:"profile"`
	MaxAttemptsPerPhase int          `json:"max_attempts_per_phase"`
	AttemptTimeoutMS    int          `json:"attempt_timeout_ms"`
	MaxSettleSeconds    int          `json:"max_settle_seconds"`
	Experiments         []Experiment `json:"experiments"`
}

func sequence() []PhaseExpectation {
	return []PhaseExpectation{
		{Phase: Before, ExpectedOutcome: Unreachable},
		{Phase: Witness, ExpectedOutcome: Reachable},
		{Phase: Restored, ExpectedOutcome: Unreachable},
	}
}

// ApprovedPlan returns a fresh, independent copy of the only supported lab plan.
func ApprovedPlan() Plan {
	const appLabel = "app.kubernetes.io/name"
	const namespaceLabel = "kubernetes.io/metadata.name"
	return Plan{
		Schema:              SchemaVersion,
		Cluster:             OwnedCluster,
		Context:             OwnedContext,
		Profile:             RequiredProfile,
		MaxAttemptsPerPhase: 2,
		AttemptTimeoutMS:    5000,
		MaxSettleSeconds:    30,
		Experiments: []Experiment{
			{
				ID: "witness-edge-to-data", RouteID: "edge-to-data",
				Source: Endpoint{Namespace: "firedoor-edge", AppName: "firedoor-edge"},
				Target: Endpoint{Namespace: "firedoor-data", AppName: "firedoor-data"},
				Witness: WitnessSpec{
					PolicyName: "firedoor-v04-witness-edge-data", PolicyNamespace: "firedoor-data",
					NamespaceSelectorKey: namespaceLabel, SourcePodSelectorKey: appLabel,
					TargetPodSelectorKey: appLabel, Protocol: "TCP", TargetPort: 8080,
				},
				Sequence: sequence(),
			},
			{
				ID: "witness-data-to-app", RouteID: "data-to-app",
				Source: Endpoint{Namespace: "firedoor-data", AppName: "firedoor-data"},
				Target: Endpoint{Namespace: "firedoor-app", AppName: "firedoor-app"},
				Witness: WitnessSpec{
					PolicyName: "firedoor-v04-witness-data-app", PolicyNamespace: "firedoor-app",
					NamespaceSelectorKey: namespaceLabel, SourcePodSelectorKey: appLabel,
					TargetPodSelectorKey: appLabel, Protocol: "TCP", TargetPort: 8080,
				},
				Sequence: sequence(),
			},
		},
	}
}

// Validate is deliberately fail-closed. No caller may silently widen the
// experimental plan. A later version must be deliberately reviewed and tested.
func Validate(plan Plan) error {
	if !reflect.DeepEqual(plan, ApprovedPlan()) {
		return errors.New("attribution plan is not the exact approved Firedoor v0.4 owned-lab contract")
	}
	return nil
}

// JSON returns the approved plan in a stable, human-reviewable representation.
func JSON() ([]byte, error) {
	p := ApprovedPlan()
	if err := Validate(p); err != nil {
		return nil, err
	}
	return json.MarshalIndent(p, "", "  ")
}
