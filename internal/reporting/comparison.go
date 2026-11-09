package reporting

// Slice 8C1 compares two separately supplied, structurally valid Boundary
// Proofs. Digest equality/difference establishes neither matching security
// behavior nor verified policy enforcement. No source files are opened here.
import (
	"errors"
	"fmt"
)

const (
	ComparisonSchemaVersion = "firedoor.v08.boundary-comparison.v1"
	RelationNotComparable   = "not_comparable"
	RelationSameDigest      = "same_artifact_digest"
	RelationDifferentDigest = "different_artifact_digests"
	DriftNotAssessed        = "not_assessed"
)

var comparisonLimitations = []string{
	"This comparison uses two separately supplied detached Boundary Proofs; source authenticity is not established.",
	"Equal artifact digests do not prove equal network behavior, policy configuration, or enforcement.",
	"Different artifact digests do not by themselves establish infrastructure drift or a security improvement.",
	"Capture timestamps belong to separate evidence layers and do not establish a causal before/after experiment.",
	"Missing or noncomparable evidence cannot be interpreted as a passing or failing security control.",
	"No current Kubernetes state, verified containment, or policy causation is established by this comparison.",
}

// LayerComparison is a comparison of the provenance of two detached evidence
// layers, never an assessment of the actual contents of their artifacts.
type LayerComparison struct {
	Layer            string `json:"layer"`
	Baseline         Layer  `json:"baseline"`
	Hardened         Layer  `json:"hardened"`
	EvidenceRelation string `json:"evidence_relation"`
}

func newLayerComparison(label string, baseline, hardened Layer) LayerComparison {
	return LayerComparison{
		Layer: label, Baseline: baseline, Hardened: hardened,
		EvidenceRelation: digestRelation(baseline, hardened),
	}
}

func digestRelation(a, b Layer) string {
	if a.Status != StatusRecorded || b.Status != StatusRecorded {
		return RelationNotComparable
	}
	if a.SHA256 == b.SHA256 {
		return RelationSameDigest
	}
	return RelationDifferentDigest
}

func (l LayerComparison) validate(label string) error {
	if l.Layer != label {
		return fmt.Errorf("%s: unsupported layer name", label)
	}
	if err := l.Baseline.validate(label + " baseline"); err != nil {
		return err
	}
	if err := l.Hardened.validate(label + " hardened"); err != nil {
		return err
	}
	if l.EvidenceRelation != digestRelation(l.Baseline, l.Hardened) {
		return fmt.Errorf("%s: unsupported or contradictory digest relation", label)
	}
	return nil
}

// ComparisonDocument is strictly a digest-provenance comparison. Its verdict
// and drift assessment must remain inconclusive/not assessed even if every
// evidence layer contains identical digests.
type ComparisonDocument struct {
	SchemaVersion         string          `json:"schema_version"`
	ScenarioID            string          `json:"scenario_id"`
	Route                 string          `json:"route"`
	BaselineProofID       string          `json:"baseline_proof_id"`
	HardenedProofID       string          `json:"hardened_proof_id"`
	Verdict               string          `json:"verdict"`
	DriftAssessment       string          `json:"drift_assessment"`
	TerraformIntent       LayerComparison `json:"terraform_intent"`
	KubernetesObservation LayerComparison `json:"kubernetes_observation"`
	NetworkBehavior       LayerComparison `json:"network_behavior"`
	Limitations           []string        `json:"limitations"`
}

// CompareBoundaryProofs requires two explicitly supplied, different proofs
// with the same scenario and route and exactly baseline/hardened profiles.
// It does not infer missing profiles, compare infrastructure contents, read
// files or claim that an experiment was run.
func CompareBoundaryProofs(baseline, hardened Document) (ComparisonDocument, error) {
	reject := errors.New("Boundary Proof comparison requires valid baseline and hardened documents with matching scenario and route")
	if baseline.Validate() != nil || hardened.Validate() != nil ||
		baseline.Profile != "baseline" || hardened.Profile != "hardened" ||
		baseline.ScenarioID != hardened.ScenarioID || baseline.Route != hardened.Route ||
		baseline.ProofID == hardened.ProofID {
		return ComparisonDocument{}, reject
	}
	result := ComparisonDocument{
		SchemaVersion:         ComparisonSchemaVersion,
		ScenarioID:            baseline.ScenarioID,
		Route:                 baseline.Route,
		BaselineProofID:       baseline.ProofID,
		HardenedProofID:       hardened.ProofID,
		Verdict:               VerdictInconclusive,
		DriftAssessment:       DriftNotAssessed,
		TerraformIntent:       newLayerComparison("terraform_intent", baseline.TerraformIntent, hardened.TerraformIntent),
		KubernetesObservation: newLayerComparison("kubernetes_observation", baseline.KubernetesObservation, hardened.KubernetesObservation),
		NetworkBehavior:       newLayerComparison("network_behavior", baseline.NetworkBehavior, hardened.NetworkBehavior),
		Limitations:           append([]string(nil), comparisonLimitations...),
	}
	return result, result.Validate()
}

// Validate rejects forged conclusions or unsupported comparison claims, and
// independently recalculates digest relations to catch altered projections.
// This is contract validation, not cryptographic source authentication.
func (d ComparisonDocument) Validate() error {
	if d.SchemaVersion != ComparisonSchemaVersion || !validScenario(d.ScenarioID) ||
		!allowedRoutes[d.Route] || !proofIDPattern.MatchString(d.BaselineProofID) ||
		!proofIDPattern.MatchString(d.HardenedProofID) || d.BaselineProofID == d.HardenedProofID ||
		d.Verdict != VerdictInconclusive || d.DriftAssessment != DriftNotAssessed {
		return errors.New("unsupported Boundary Proof comparison identity or conclusion")
	}
	for _, item := range []struct {
		layer LayerComparison
		label string
	}{{d.TerraformIntent, "terraform_intent"}, {d.KubernetesObservation, "kubernetes_observation"}, {d.NetworkBehavior, "network_behavior"}} {
		if err := item.layer.validate(item.label); err != nil {
			return err
		}
	}
	if len(d.Limitations) != len(comparisonLimitations) {
		return errors.New("required comparison limitations missing or modified")
	}
	for i, want := range comparisonLimitations {
		if d.Limitations[i] != want {
			return errors.New("required comparison limitations missing or modified")
		}
	}
	return nil
}
