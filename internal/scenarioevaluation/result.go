package scenarioevaluation

// This result contract is deliberately narrower than a scenario executor.
// H2 only consumes validated H1 historical reviews. It NEVER upgrades
// those reviews into a passed/failed FD-004 scenario verdict.

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const ResultSchema = "firedoor.v10.scenario-result.v1"

type Outcome string

const (
	OutcomeNotEvaluated Outcome = "not_evaluated"
	OutcomeIncomplete   Outcome = "incomplete"
	OutcomePassed       Outcome = "passed"
	OutcomeFailed       Outcome = "failed"
)

// ValidOutcome defines the vocabulary for future assessment implementations.
// The H2 historical-review adapter can ONLY emit OutcomeNotEvaluated.
func ValidOutcome(o Outcome) bool {
	switch o {
	case OutcomeNotEvaluated, OutcomeIncomplete, OutcomePassed, OutcomeFailed:
		return true
	default:
		return false
	}
}

type Requirement struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type ScenarioResult struct {
	SchemaVersion                string        `json:"schema_version"`
	ScenarioID                   string        `json:"scenario_id"`
	ScenarioClaim                string        `json:"scenario_claim"`
	Outcome                      Outcome       `json:"outcome"`
	EvidenceRelation             string        `json:"evidence_relation"`
	EvidenceSource               string        `json:"evidence_source"`
	ObservedUTC                  string        `json:"observed_utc"`
	SupportingRoutes             []string      `json:"supporting_routes"`
	PositiveControls             []string      `json:"positive_controls"`
	MappedControlsNotEvaluated   []string      `json:"mapped_controls_not_evaluated"`
	Requirements                 []Requirement `json:"requirements"`
	ScenarioEvaluationExecuted   bool          `json:"scenario_evaluation_executed"`
	ContractScopeVerified        bool          `json:"contract_scope_verified"`
	CurrentClusterChecked        bool          `json:"current_cluster_checked"`
	OriginalReceiptAuthenticated bool          `json:"original_receipt_authenticated"`
	ScenarioFullyCertified       bool          `json:"scenario_fully_certified"`
	Limitations                  []string      `json:"limitations"`
}

var historicalReviewFields = []string{
	"schema_version", "scenario_id", "scenario_claim", "evidence_source", "captured_utc",
	"historical_assertion", "supporting_routes", "observed_positive_controls",
	"scenario_contract_status", "scenario_verdict", "contract_cluster",
	"evidence_identifies_cluster", "contract_scope_verified", "current_cluster_checked",
	"current_enforcement_verified", "original_receipt_authenticated",
	"scenario_fully_certified", "limitations",
}

var expectedH1Limitations = []string{
	"A sanitized G1 projection omits the actual cluster identifier; the FD-004 contract cluster scope cannot be independently matched.",
	"The FD-004 source contract remains spec_only_not_implemented; this review does not execute or certify the scenario or mapped controls.",
	"HTTP timeouts and successful controls support a bounded historical observation, not general policy causation or isolation.",
	"A local hash match is not independent authentication of the private F5 receipt. No current cluster or network test was run.",
}

var expectedH2Requirements = []Requirement{
	{"scenario_implemented", "not_met", "FD-004 remains spec_only_not_implemented."},
	{"contract_cluster_scope", "unverified", "The sanitized projection does not identify the contract's kind-firedoor-local cluster."},
	{"evidence_origin", "unverified", "The private F5 receipt is not independently authenticated by this review."},
	{"current_cluster_assessment", "not_performed", "No current cluster inspection or traffic tests were executed."},
}

var expectedH2Limitations = []string{
	"Historical traffic observations support one bounded assertion, not a complete scenario assessment.",
	"The FD-004 cluster scope and source receipt have not been independently verified.",
	"Mapped controls C-004 and C-005 were not evaluated by this review.",
	"This result does not establish current enforcement, policy causation, recovery, or production security.",
}

func ValidateHistoricalReview(r Review) error {
	if r.SchemaVersion != Schema || r.ScenarioID != "FD-004" ||
		r.ScenarioClaim != claim || r.EvidenceSource != "user_supplied_sanitized_g1_projection_unattested" ||
		r.HistoricalAssertion != "edge_to_data_timeout_consistent_with_claim_at_capture" ||
		r.ScenarioContractStatus != "spec_only_not_implemented" || r.ScenarioVerdict != "not_tested" ||
		r.ContractCluster != scopeCluster || r.EvidenceIdentifiesCluster || r.ContractScopeVerified ||
		r.CurrentClusterChecked || r.CurrentEnforcementVerified || r.OriginalReceiptAuthenticated ||
		r.ScenarioFullyCertified ||
		!reflect.DeepEqual(r.SupportingRoutes, []string{"edge-to-data"}) ||
		!reflect.DeepEqual(r.ObservedControls, []string{"edge-to-app", "app-to-data", "data-to-edge-control"}) ||
		!reflect.DeepEqual(r.Limitations, expectedH1Limitations) {
		return fmt.Errorf("H2 rejected H1 review: unexpected assertion, provenance, routes, or certification fields")
	}
	stamp, err := time.Parse(time.RFC3339Nano, r.CapturedUTC)
	if err != nil || !endsInZ(r.CapturedUTC) || stamp.Year() < 2026 || len(r.CapturedUTC) > 48 {
		return fmt.Errorf("H2 rejected H1 review: invalid UTC timestamp")
	}
	return nil
}

// FromHistoricalReview strictly validates a user-supplied H1 result. A caller
// must additionally pin and verify the input file's SHA-256 before calling.
func FromHistoricalReview(data []byte) (ScenarioResult, error) {
	var r Review
	if err := decodeExact(data, &r, historicalReviewFields...); err != nil {
		return ScenarioResult{}, fmt.Errorf("H2 rejected H1 JSON: %w", err)
	}
	if err := ValidateHistoricalReview(r); err != nil {
		return ScenarioResult{}, err
	}
	result := ScenarioResult{
		SchemaVersion:              ResultSchema,
		ScenarioID:                 r.ScenarioID,
		ScenarioClaim:              r.ScenarioClaim,
		Outcome:                    OutcomeNotEvaluated,
		EvidenceRelation:           "historical_observation_consistent_with_claim_at_capture",
		EvidenceSource:             "user_supplied_hash_checked_h1_review_unattested",
		ObservedUTC:                r.CapturedUTC,
		SupportingRoutes:           append([]string(nil), r.SupportingRoutes...),
		PositiveControls:           append([]string(nil), r.ObservedControls...),
		MappedControlsNotEvaluated: []string{"C-004", "C-005"},
		Requirements:               append([]Requirement(nil), expectedH2Requirements...),
		Limitations:                append([]string(nil), expectedH2Limitations...),
	}
	if err := ValidateScenarioResult(result); err != nil {
		return ScenarioResult{}, err
	}
	return result, nil
}

// H2 is a historical-review-only result schema. Future scenario execution
// must introduce separately authenticated evidence before it can emit passed,
// failed, or incomplete; changing the outcome alone is explicitly rejected.
func ValidateScenarioResult(r ScenarioResult) error {
	if !ValidOutcome(r.Outcome) || r.Outcome != OutcomeNotEvaluated ||
		r.SchemaVersion != ResultSchema || r.ScenarioID != "FD-004" || r.ScenarioClaim != claim ||
		r.EvidenceRelation != "historical_observation_consistent_with_claim_at_capture" ||
		r.EvidenceSource != "user_supplied_hash_checked_h1_review_unattested" ||
		r.ScenarioEvaluationExecuted || r.ContractScopeVerified || r.CurrentClusterChecked ||
		r.OriginalReceiptAuthenticated || r.ScenarioFullyCertified ||
		!reflect.DeepEqual(r.SupportingRoutes, []string{"edge-to-data"}) ||
		!reflect.DeepEqual(r.PositiveControls, []string{"edge-to-app", "app-to-data", "data-to-edge-control"}) ||
		!reflect.DeepEqual(r.MappedControlsNotEvaluated, []string{"C-004", "C-005"}) ||
		!reflect.DeepEqual(r.Requirements, expectedH2Requirements) ||
		!reflect.DeepEqual(r.Limitations, expectedH2Limitations) {
		return fmt.Errorf("H2 result must remain not_evaluated with historical-only evidence and all certification gates unverified")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, r.ObservedUTC)
	if err != nil || !endsInZ(r.ObservedUTC) || timestamp.Year() < 2026 {
		return fmt.Errorf("H2 result invalid UTC capture time")
	}
	return nil
}

func endsInZ(v string) bool { return strings.HasSuffix(v, "Z") }
