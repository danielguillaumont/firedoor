package prooflens

// HistoricalBehaviorContext joins independent, locally validated artifacts for
// an *explanatory* timeline. It is NOT a multi-stage attestation. In particular,
// v0.4 witness traffic did not originate from the v0.5 speculative plan run.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

const (
	HistoricalBehaviorContextSchema = "firedoor.v05.historical-behavior-context.v1"
	HistoricalBehaviorContextMode   = "HISTORICAL_CONTEXT_READ_ONLY_NON_CAUSAL"
	HistoricalBehaviorContextStatus = "HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN"
)

type HistoricalBehaviorReference struct {
	ExperimentID        string                   `json:"experiment_id"`
	RouteID             string                   `json:"route_id"`
	CapturedUTC         string                   `json:"captured_utc"`
	ReportFile          string                   `json:"report_file"`
	ReportSHA256        string                   `json:"report_sha256"`
	JournalSHA256       string                   `json:"journal_sha256"`
	RecordSHA256        string                   `json:"record_sha256"`
	Phases              []HistoricalWitnessPhase `json:"phases"`
	RecoveryEvents      int                      `json:"recovery_events"`
	RestorationVerified bool                     `json:"restoration_verified"`
	HistoricalOnly      bool                     `json:"historical_only"`
}

type HistoricalBehaviorContext struct {
	SchemaVersion               string                        `json:"schema_version"`
	Mode                        string                        `json:"mode"`
	ContextStatus               string                        `json:"context_status"`
	ReceiptID                   string                        `json:"receipt_id"`
	ReceiptSHA256               string                        `json:"receipt_sha256"`
	PlanCapturedUTC             string                        `json:"plan_captured_utc"`
	ObservationSHA256           string                        `json:"observation_sha256"`
	ObservationCapturedUTC      string                        `json:"observation_captured_utc"`
	CorrelationSHA256           string                        `json:"correlation_sha256"`
	SourceTreeSHA256            string                        `json:"source_tree_sha256"`
	Cluster                     string                        `json:"cluster"`
	Context                     string                        `json:"context"`
	Profile                     string                        `json:"profile"`
	HistoricalWitnesses         []HistoricalBehaviorReference `json:"historical_witnesses"`
	HistoricalWitnessCount      int                           `json:"historical_witness_count"`
	HistoricReportsValidated    bool                          `json:"historical_reports_validated"`
	ContemporaneousEvidence     bool                          `json:"contemporaneous_evidence"`
	TerraformApplied            bool                          `json:"terraform_applied"`
	TerraformExecutedInThisStep bool                          `json:"terraform_executed_in_this_step"`
	CurrentTrafficProbesRun     bool                          `json:"current_traffic_probes_run"`
	WitnessExperimentReplayed   bool                          `json:"witness_experiment_replayed"`
	KubernetesObservedReadOnly  bool                          `json:"kubernetes_observed_read_only"`
	ProofVerdict                string                        `json:"proof_verdict"`
	Limitations                 []string                      `json:"limitations"`
	ContextSHA256               string                        `json:"context_sha256"`
}

func historicalBehaviorLimitations() []string {
	return []string{
		"Previously saved v0.4 A/B/A reports and journals are historical local evidence; no traffic is replayed.",
		"A/B/A application-level reachable/unreachable results are bounded observations, not packet-level proof or a general security guarantee.",
		"The historical reports predate the v0.5 speculative Terraform plan and read-only Kubernetes inventory; they are not contemporaneous.",
		"Same cluster/context/profile names do not establish the same cluster incarnation, source revision, policy provenance, or a causal chain.",
		"The Terraform source hash links only the v0.5 receipt and Kubernetes observation; historic reports do not carry that validated source binding.",
		"All SHA256 checks are local consistency checks, not authenticated attestations. The overall proof verdict remains inconclusive.",
	}
}

func historicalBehaviorDigest(r HistoricalBehaviorContext) (string, error) {
	r.ContextSHA256 = ""
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func expectedHistoricalPhase(r HistoricalBehaviorReference) bool {
	if len(r.Phases) != 3 {
		return false
	}
	wantNames := [3]string{"before", "witness_present", "restored"}
	wantOutcomes := [3]string{"unreachable", "reachable", "unreachable"}
	for i, p := range r.Phases {
		if p.Phase != wantNames[i] || p.Observed != wantOutcomes[i] || p.Attempts < 1 || p.Attempts > 2 || (i != 1 && p.Attempts != 2) {
			return false
		}
	}
	return true
}

// BuildHistoricalBehaviorContext first validates the existing read-only receipt
// correlation and *exactly two* separately inspected v0.4 report/journal pairs.
// No database, Kubernetes, Terraform or file I/O occurs here.
func BuildHistoricalBehaviorContext(c ReceiptObservationCorrelation, witnesses []HistoricalWitnessRecord) (HistoricalBehaviorContext, error) {
	empty := HistoricalBehaviorContext{}
	if err := VerifyReceiptObservationCorrelation(c); err != nil {
		return empty, errors.New("receipt-to-observation correlation invalid")
	}
	if len(witnesses) != 2 {
		return empty, errors.New("exactly two approved historical A/B/A witness records required")
	}
	tPlan, err := time.Parse(time.RFC3339Nano, c.PlanCapturedUTC)
	if err != nil {
		return empty, errors.New("plan timestamp invalid")
	}
	tObs, err := time.Parse(time.RFC3339Nano, c.ObservationUTC)
	if err != nil || tObs.Before(tPlan) {
		return empty, errors.New("observation predates Terraform plan")
	}
	refs := make([]HistoricalBehaviorReference, 0, 2)
	for _, w := range witnesses {
		if err := VerifyHistoricalWitnessRecord(w); err != nil {
			return empty, errors.New("historical witness invalid")
		}
		if w.Cluster != c.Cluster || w.Context != c.Context || w.Profile != c.Profile {
			return empty, errors.New("historical environment metadata does not match approved lab")
		}
		t, err := time.Parse(time.RFC3339Nano, w.CapturedUTC)
		if err != nil || !t.Before(tPlan) {
			return empty, errors.New("historical witness must predate the actual plan capture")
		}
		phases := append([]HistoricalWitnessPhase(nil), w.Phases...)
		refs = append(refs, HistoricalBehaviorReference{
			ExperimentID: w.ExperimentID, RouteID: w.RouteID, CapturedUTC: w.CapturedUTC,
			ReportFile: w.ReportFile, ReportSHA256: w.ReportSHA256, JournalSHA256: w.JournalSHA256,
			RecordSHA256: w.RecordSHA256, Phases: phases, RecoveryEvents: w.RecoveryEventCount,
			RestorationVerified: w.RestorationVerified, HistoricalOnly: true,
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].RouteID < refs[j].RouteID })
	if refs[0].RouteID != "data-to-app" || refs[1].RouteID != "edge-to-data" || refs[0].ExperimentID != "witness-data-to-app" || refs[1].ExperimentID != "witness-edge-to-data" {
		return empty, errors.New("missing or duplicate approved A/B/A routes")
	}
	out := HistoricalBehaviorContext{
		SchemaVersion: HistoricalBehaviorContextSchema, Mode: HistoricalBehaviorContextMode,
		ContextStatus: HistoricalBehaviorContextStatus,
		ReceiptID:     c.ReceiptID, ReceiptSHA256: c.ReceiptSHA256, PlanCapturedUTC: c.PlanCapturedUTC,
		ObservationSHA256: c.ObservationSHA256, ObservationCapturedUTC: c.ObservationUTC,
		CorrelationSHA256: c.CorrelationSHA256, SourceTreeSHA256: c.SourceTreeSHA256,
		Cluster: c.Cluster, Context: c.Context, Profile: c.Profile,
		HistoricalWitnesses: refs, HistoricalWitnessCount: len(refs),
		HistoricReportsValidated: true, ContemporaneousEvidence: false,
		TerraformApplied: false, TerraformExecutedInThisStep: false,
		CurrentTrafficProbesRun: false, WitnessExperimentReplayed: false,
		KubernetesObservedReadOnly: true, ProofVerdict: Inconclusive,
		Limitations: historicalBehaviorLimitations(),
	}
	out.ContextSHA256, err = historicalBehaviorDigest(out)
	if err != nil {
		return empty, err
	}
	if err := VerifyHistoricalBehaviorContext(out); err != nil {
		return empty, err
	}
	return out, nil
}

// VerifyHistoricalBehaviorContext checks consistency of a detached summary. A
// detached summary is not a substitute for re-reading the underlying artifacts.
func VerifyHistoricalBehaviorContext(r HistoricalBehaviorContext) error {
	if r.SchemaVersion != HistoricalBehaviorContextSchema || r.Mode != HistoricalBehaviorContextMode ||
		r.ContextStatus != HistoricalBehaviorContextStatus || !validText(r.ReceiptID, 128) ||
		!validHex(r.ReceiptSHA256, 64) || !validHex(r.ObservationSHA256, 64) ||
		!validHex(r.CorrelationSHA256, 64) || !validHex(r.SourceTreeSHA256, 64) ||
		!validUTC(r.PlanCapturedUTC) || !validUTC(r.ObservationCapturedUTC) ||
		r.Cluster != "firedoor-local" || r.Context != "kind-firedoor-local" || r.Profile != "hardened" ||
		r.HistoricalWitnessCount != 2 || len(r.HistoricalWitnesses) != 2 || !r.HistoricReportsValidated ||
		r.ContemporaneousEvidence || r.TerraformApplied || r.TerraformExecutedInThisStep ||
		r.CurrentTrafficProbesRun || r.WitnessExperimentReplayed || !r.KubernetesObservedReadOnly ||
		r.ProofVerdict != Inconclusive || !reflect.DeepEqual(r.Limitations, historicalBehaviorLimitations()) ||
		!validHex(r.ContextSHA256, 64) {
		return errors.New("invalid or overstated historical behavior context")
	}
	tPlan, err := time.Parse(time.RFC3339Nano, r.PlanCapturedUTC)
	if err != nil {
		return err
	}
	tObs, err := time.Parse(time.RFC3339Nano, r.ObservationCapturedUTC)
	if err != nil || tObs.Before(tPlan) {
		return errors.New("invalid current capture order")
	}
	for i, w := range r.HistoricalWitnesses {
		wantRoute := "data-to-app"
		if i == 1 {
			wantRoute = "edge-to-data"
		}
		if w.RouteID != wantRoute || w.ExperimentID != "witness-"+wantRoute ||
			!w.HistoricalOnly || !validUTC(w.CapturedUTC) ||
			!validHex(w.ReportSHA256, 64) || !validHex(w.JournalSHA256, 64) ||
			!validHex(w.RecordSHA256, 64) || w.RecoveryEvents != 6 || !w.RestorationVerified ||
			!expectedHistoricalPhase(w) {
			return errors.New("invalid historical witness reference")
		}
		route, t, err := historicalNameInfo(w.ReportFile)
		observed, obsErr := time.Parse(time.RFC3339Nano, w.CapturedUTC)
		if err != nil || obsErr != nil || route != wantRoute || !observed.Truncate(time.Second).Equal(t) || !observed.Before(tPlan) {
			return errors.New("historical report chronology or filename invalid")
		}
	}
	digest, err := historicalBehaviorDigest(r)
	if err != nil || digest != r.ContextSHA256 {
		return errors.New("historical context checksum invalid")
	}
	return nil
}
