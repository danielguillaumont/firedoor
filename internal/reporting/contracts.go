// Package reporting defines the v0.8 Boundary Proof export contract.
//
// This first slice formats already normalized, detached provenance only. It does
// not read evidence, inspect Kubernetes, apply Terraform, execute a scenario,
// verify a source's authenticity, or establish policy enforcement.
package reporting

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	SchemaVersion = "firedoor.v08.boundary-proof.v1"
	MaxJSONBytes  = 32 * 1024
	MaxHTMLBytes  = 64 * 1024
)

const (
	VerdictInconclusive = "inconclusive"
	StatusMissing       = "not_available"
	StatusRecorded      = "recorded"
)

var (
	proofIDPattern   = regexp.MustCompile(`^BP-[A-Za-z0-9][A-Za-z0-9-]{0,62}$`)
	referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)
	digestPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Layer contains only an opaque reference, a digest and a timestamp; it is not
// a raw Terraform plan, Kubernetes object, environment dump or probe log.
// A digest is an integrity value, not authentication or causal attribution.
type Layer struct {
	Status        string `json:"status"`
	Reference     string `json:"reference,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	CapturedAtUTC string `json:"captured_at_utc,omitempty"`
}

func (l Layer) validate(label string) error {
	switch l.Status {
	case StatusMissing:
		if l.Reference != "" || l.SHA256 != "" || l.CapturedAtUTC != "" {
			return fmt.Errorf("%s: missing evidence cannot carry provenance", label)
		}
	case StatusRecorded:
		if !referencePattern.MatchString(l.Reference) || !digestPattern.MatchString(l.SHA256) {
			return fmt.Errorf("%s: invalid opaque reference or SHA-256", label)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, l.CapturedAtUTC)
		if err != nil || !strings.HasSuffix(l.CapturedAtUTC, "Z") || timestamp.IsZero() ||
			timestamp.UTC().Format(time.RFC3339Nano) != l.CapturedAtUTC {
			return fmt.Errorf("%s: captured_at_utc must be canonical UTC", label)
		}
	default:
		return fmt.Errorf("%s: unsupported layer status", label)
	}
	return nil
}

// Document explicitly separates three evidence layers. The only verdict in
// Slice 8A is INCONCLUSIVE: even three recorded layers do not, by themselves,
// prove one causal enforcement chain. A later verified attribution contract
// would require a separate, explicitly reviewed schema/version.
type Document struct {
	SchemaVersion         string   `json:"schema_version"`
	ProofID               string   `json:"proof_id"`
	ScenarioID            string   `json:"scenario_id"`
	Profile               string   `json:"profile"`
	Route                 string   `json:"route"`
	Verdict               string   `json:"verdict"`
	TerraformIntent       Layer    `json:"terraform_intent"`
	KubernetesObservation Layer    `json:"kubernetes_observation"`
	NetworkBehavior       Layer    `json:"network_behavior"`
	Limitations           []string `json:"limitations"`
}

var allowedRoutes = map[string]bool{
	"edge-to-app":  true,
	"app-to-edge":  true,
	"app-to-data":  true,
	"data-to-app":  true,
	"edge-to-data": true,
	"data-to-edge": true,
}

var canonicalLimitations = []string{
	"This detached report is not a live Kubernetes inventory or an authenticated attestation.",
	"Terraform intent, Kubernetes observation, and network behavior are separate evidence layers.",
	"A plan is not an apply; a policy's presence or failed connection alone does not prove enforcement.",
	"No policy causation, current drift, or verified containment is established by this export.",
}

// NewDocument initializes the fixed conservative report contract. Callers must
// supply only independently reviewed, normalized references; no source files
// are opened here.
func NewDocument(id, scenario, profile, route string, intent, observed, behavior Layer) (Document, error) {
	doc := Document{
		SchemaVersion:         SchemaVersion,
		ProofID:               id,
		ScenarioID:            scenario,
		Profile:               profile,
		Route:                 route,
		Verdict:               VerdictInconclusive,
		TerraformIntent:       intent,
		KubernetesObservation: observed,
		NetworkBehavior:       behavior,
		Limitations:           append([]string(nil), canonicalLimitations...),
	}
	return doc, doc.Validate()
}

// Validate rejects unbounded identifiers, paths, URLs, unsupported verdicts,
// fabricated timestamps, arbitrary limitation text and incomplete provenance.
// Validation guarantees structure only, never the actual source's authenticity.
func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion || !proofIDPattern.MatchString(d.ProofID) ||
		!validScenario(d.ScenarioID) || (d.Profile != "baseline" && d.Profile != "hardened") ||
		!allowedRoutes[d.Route] || d.Verdict != VerdictInconclusive {
		return errors.New("unsupported or out-of-scope Boundary Proof contract")
	}
	if err := d.TerraformIntent.validate("terraform intent"); err != nil {
		return err
	}
	if err := d.KubernetesObservation.validate("kubernetes observation"); err != nil {
		return err
	}
	if err := d.NetworkBehavior.validate("network behavior"); err != nil {
		return err
	}
	if len(d.Limitations) != len(canonicalLimitations) {
		return errors.New("required limitations missing or modified")
	}
	for i, limitation := range canonicalLimitations {
		if d.Limitations[i] != limitation {
			return errors.New("required limitations missing or modified")
		}
	}
	return nil
}

func validScenario(id string) bool {
	return id == "FD-001" || id == "FD-002" || id == "FD-003" || id == "FD-004" || id == "FD-005"
}
