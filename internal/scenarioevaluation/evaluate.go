// Package scenarioevaluation reviews a single historical FD-004 assertion
// against a locally supplied sanitized G1 projection. It neither authenticates
// the original private receipt nor certifies or executes the scenario.
package scenarioevaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/consoleliveproof"
)

const Schema = "firedoor.v10.fd004-h1-historical-assertion-review.v1"
const ProjectionSHA256 = "97E790031DFBE044327035B96FFD82FAA41A23DF28D229DBEE9D0BA51C67A944"
const ContractSHA256 = "A278E9354F25A90F32590090D9498964CBB202DD101B5050750969A2AB871FF0"

const claim = "Frontend cannot directly reach protected data service after hardening"
const scopeCluster = "kind-firedoor-local"

type contract struct {
	SchemaVersion  string   `json:"schema_version"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	Claim          string   `json:"claim"`
	MappedControls []string `json:"mapped_controls"`
	Scope          struct {
		Cluster   string `json:"cluster"`
		Ownership string `json:"ownership"`
		Operation string `json:"operation"`
	} `json:"scope"`
	Profiles struct {
		Baseline string `json:"baseline"`
		Hardened string `json:"hardened"`
	} `json:"profiles"`
	Preflight               []string `json:"preflight"`
	Safety                  []string `json:"safety"`
	ImplementationMilestone string   `json:"implementation_milestone"`
}

type Review struct {
	SchemaVersion                string   `json:"schema_version"`
	ScenarioID                   string   `json:"scenario_id"`
	ScenarioClaim                string   `json:"scenario_claim"`
	EvidenceSource               string   `json:"evidence_source"`
	CapturedUTC                  string   `json:"captured_utc"`
	HistoricalAssertion          string   `json:"historical_assertion"`
	SupportingRoutes             []string `json:"supporting_routes"`
	ObservedControls             []string `json:"observed_positive_controls"`
	ScenarioContractStatus       string   `json:"scenario_contract_status"`
	ScenarioVerdict              string   `json:"scenario_verdict"`
	ContractCluster              string   `json:"contract_cluster"`
	EvidenceIdentifiesCluster    bool     `json:"evidence_identifies_cluster"`
	ContractScopeVerified        bool     `json:"contract_scope_verified"`
	CurrentClusterChecked        bool     `json:"current_cluster_checked"`
	CurrentEnforcementVerified   bool     `json:"current_enforcement_verified"`
	OriginalReceiptAuthenticated bool     `json:"original_receipt_authenticated"`
	ScenarioFullyCertified       bool     `json:"scenario_fully_certified"`
	Limitations                  []string `json:"limitations"`
}

func decodeExact(data []byte, output any, keys ...string) error {
	if len(data) == 0 || len(data) > 32*1024 {
		return errors.New("missing or oversized JSON")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil || len(fields) != len(keys) {
		return errors.New("unexpected top-level JSON fields")
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("missing field %s", key)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(output); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}

var contractFields = []string{
	"schema_version", "id", "name", "status", "claim", "mapped_controls", "scope",
	"profiles", "preflight", "safety", "implementation_milestone",
}
var projectionFields = []string{
	"schema_version", "source", "evidence_status", "lab_label", "observed_utc",
	"allowed_count", "denied_count", "network_verified_at_capture", "current_cluster_checked",
	"current_enforcement_verified", "terraform_executed_by_projection", "kubernetes_changed_by_projection",
	"recovery_proven", "observations", "limitations",
}

var expectedPaths = []consoleliveproof.PathObservation{
	{ID: "edge-to-app", From: "edge", To: "app", Observation: "allowed"},
	{ID: "app-to-data", From: "app", To: "data", Observation: "allowed"},
	{ID: "data-to-edge-control", From: "data", To: "edge", Observation: "allowed"},
	{ID: "edge-to-data", From: "edge", To: "data", Observation: "denied"},
	{ID: "data-to-app", From: "data", To: "app", Observation: "denied"},
}
var expectedLimitations = []string{
	"Historical F5 observations were verified at capture time; this projection does not check current cluster state.",
	"Five HTTP paths demonstrate bounded behavior in a disposable lab; they are not universal proof of isolation.",
	"A local SHA-256 match is an integrity check, not an independent or cryptographic attestation of origin.",
	"Recovery and production security posture are not established by this display projection.",
}

// Evaluate is a pure, strict evidence review. Even success is only an
// observation compatible with the FD-004 claim, NEVER a security verdict.
func Evaluate(contractJSON, projectionJSON []byte) (Review, error) {
	reject := func(reason string) (Review, error) {
		return Review{}, fmt.Errorf("H1 historical review rejected: %s", reason)
	}
	var c contract
	if err := decodeExact(contractJSON, &c, contractFields...); err != nil {
		return reject("contract JSON: " + err.Error())
	}
	if c.SchemaVersion != "1.0.0" || c.ID != "FD-004" || c.Name != "Lateral Movement" ||
		c.Claim != claim || c.Status != "spec_only_not_implemented" || c.Scope.Cluster != scopeCluster ||
		c.Scope.Operation != "edge/frontend to data/service TCP probe with independent positive control" ||
		!reflect.DeepEqual(c.MappedControls, []string{"C-004", "C-005"}) ||
		strings.TrimSpace(c.Scope.Ownership) == "" || strings.TrimSpace(c.Profiles.Baseline) == "" ||
		strings.TrimSpace(c.Profiles.Hardened) == "" || len(c.Preflight) != 5 || len(c.Safety) != 4 ||
		c.ImplementationMilestone != "v0.3" {
		return reject("contract does not match reviewed FD-004 specification")
	}
	var p consoleliveproof.Projection
	if err := decodeExact(projectionJSON, &p, projectionFields...); err != nil {
		return reject("projection JSON: " + err.Error())
	}
	if p.SchemaVersion != consoleliveproof.ProjectionSchema ||
		p.Source != "explicit_private_f5_receipt_projection" ||
		p.EvidenceStatus != "historical_verified_at_capture_not_live" ||
		p.LabLabel != "Disposable KIND network-policy lab" ||
		p.AllowedCount != 3 || p.DeniedCount != 2 || !p.NetworkVerifiedAtCapture ||
		p.CurrentClusterChecked || p.CurrentEnforcementVerified || p.TerraformExecutedByProjection ||
		p.KubernetesChangedByProjection || p.RecoveryProven ||
		!reflect.DeepEqual(p.Observations, expectedPaths) || !reflect.DeepEqual(p.Limitations, expectedLimitations) {
		return reject("projection does not meet strict historical five-flow contract")
	}
	if len(p.ObservedUTC) > 48 || !strings.HasSuffix(p.ObservedUTC, "Z") {
		return reject("invalid capture timestamp")
	}
	stamp, err := time.Parse(time.RFC3339Nano, p.ObservedUTC)
	if err != nil || stamp.Year() < 2026 {
		return reject("invalid capture timestamp")
	}
	return Review{
		SchemaVersion: Schema,
		ScenarioID:    c.ID, ScenarioClaim: c.Claim,
		EvidenceSource:         "user_supplied_sanitized_g1_projection_unattested",
		CapturedUTC:            stamp.UTC().Format(time.RFC3339Nano),
		HistoricalAssertion:    "edge_to_data_timeout_consistent_with_claim_at_capture",
		SupportingRoutes:       []string{"edge-to-data"},
		ObservedControls:       []string{"edge-to-app", "app-to-data", "data-to-edge-control"},
		ScenarioContractStatus: c.Status, ScenarioVerdict: "not_tested",
		ContractCluster: c.Scope.Cluster, EvidenceIdentifiesCluster: false,
		ContractScopeVerified: false, CurrentClusterChecked: false,
		CurrentEnforcementVerified: false, OriginalReceiptAuthenticated: false,
		ScenarioFullyCertified: false,
		Limitations: []string{
			"A sanitized G1 projection omits the actual cluster identifier; the FD-004 contract cluster scope cannot be independently matched.",
			"The FD-004 source contract remains spec_only_not_implemented; this review does not execute or certify the scenario or mapped controls.",
			"HTTP timeouts and successful controls support a bounded historical observation, not general policy causation or isolation.",
			"A local hash match is not independent authentication of the private F5 receipt. No current cluster or network test was run.",
		},
	}, nil
}
