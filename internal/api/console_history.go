package api

// Historical activity is a pure, minimized projection of the same vetted,
// explicitly approved detached Proof Lens summary. This is NOT an execution
// log, a control evaluation, a drift comparison, or a current inventory.
// No file, database, Terraform, Kubernetes, or network access occurs here.

import (
	"errors"
	"sort"
	"time"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const HistoricalActivitySource = "approved_detached_historical_report_only"

// HistoricalRecord intentionally omits receipt identifiers, hashes, raw
// observations, and original evidence paths.
type HistoricalRecord struct {
	RecordID          string `json:"record_id"`
	Kind              string `json:"kind"`
	CapturedUTC       string `json:"captured_utc"`
	Subject           string `json:"subject"`
	EvidenceClass     string `json:"evidence_class"`
	ExecutionVerified bool   `json:"execution_verified"`
}

type DriftAvailability struct {
	Status              string `json:"status"`
	ComparableSnapshots int    `json:"comparable_snapshots"`
	CurrentInventory    bool   `json:"current_inventory"`
}

type HistoricalActivity struct {
	SchemaVersion   string             `json:"schema_version"`
	Source          string             `json:"source"`
	ActivityStatus  string             `json:"activity_status"`
	SecurityVerdict string             `json:"security_verdict"`
	Records         []HistoricalRecord `json:"records"`
	Drift           DriftAvailability  `json:"drift"`
	Limitations     []string           `json:"limitations"`
}

// projectHistoricalActivity has no side effects. The upstream CLI can only
// supply the approved, verified in-memory console summary. Check key safety
// invariants again to prevent accidental misrepresentation by future callers.
func projectHistoricalActivity(proof *prooflens.ConsoleProofSummary) (HistoricalActivity, error) {
	if proof == nil || proof.SchemaVersion != prooflens.ConsoleProofSchema ||
		proof.Verdict != prooflens.Inconclusive ||
		proof.EvidenceStatus != "historical_context_only_not_a_proof_chain" ||
		!proof.LocalConsistencyOnly || proof.ProofChainEstablished || proof.IndependentlyAttested ||
		proof.Terraform.Applied || proof.Kubernetes.CurrentRead ||
		proof.Terraform.EvidenceType != "speculative_plan_not_apply" ||
		proof.Kubernetes.EvidenceType != "read_only_snapshot_at_collection_time" ||
		len(proof.HistoricalWitnesses) != 2 {
		return HistoricalActivity{}, errors.New("approved historical activity unavailable")
	}
	planTime, err := time.Parse(time.RFC3339Nano, proof.Terraform.CapturedUTC)
	if err != nil {
		return HistoricalActivity{}, errors.New("invalid historical activity timestamp")
	}
	kubeTime, err := time.Parse(time.RFC3339Nano, proof.Kubernetes.CapturedUTC)
	if err != nil || kubeTime.Before(planTime) {
		return HistoricalActivity{}, errors.New("invalid historical activity chronology")
	}
	records := []HistoricalRecord{
		{RecordID: "plan", Kind: "terraform_plan", CapturedUTC: proof.Terraform.CapturedUTC,
			Subject: "Speculative Terraform plan receipt", EvidenceClass: "plan_not_applied"},
		{RecordID: "snapshot", Kind: "kubernetes_snapshot", CapturedUTC: proof.Kubernetes.CapturedUTC,
			Subject: "Read-only Kubernetes observation", EvidenceClass: "historical_not_current"},
	}
	seen := map[string]bool{}
	for _, w := range proof.HistoricalWitnesses {
		if (w.RouteID != "data-to-app" && w.RouteID != "edge-to-data") || seen[w.RouteID] || !w.HistoricalOnly || !w.RestorationVerified ||
			len(w.Phases) != 3 ||
			w.Phases[0].Phase != "before" || w.Phases[0].Observed != "unreachable" ||
			w.Phases[1].Phase != "witness_present" || w.Phases[1].Observed != "reachable" ||
			w.Phases[2].Phase != "restored" || w.Phases[2].Observed != "unreachable" {
			return HistoricalActivity{}, errors.New("unrecognized historical witness")
		}
		seen[w.RouteID] = true
		at, err := time.Parse(time.RFC3339Nano, w.CapturedUTC)
		if err != nil || !at.Before(planTime) {
			return HistoricalActivity{}, errors.New("invalid witness chronology")
		}
		records = append(records, HistoricalRecord{
			RecordID: "witness-" + w.RouteID, Kind: "historical_witness", CapturedUTC: w.CapturedUTC,
			Subject: w.RouteID + " A/B/A historical observation", EvidenceClass: "historical_not_causal",
		})
	}
	if len(seen) != 2 {
		return HistoricalActivity{}, errors.New("missing historical witness")
	}
	sort.Slice(records, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, records[i].CapturedUTC)
		b, _ := time.Parse(time.RFC3339Nano, records[j].CapturedUTC)
		if a.Equal(b) {
			return records[i].RecordID < records[j].RecordID
		}
		return a.After(b)
	})
	return HistoricalActivity{
		SchemaVersion: SchemaVersion, Source: HistoricalActivitySource,
		ActivityStatus: "historical_records_not_run_history", SecurityVerdict: prooflens.Inconclusive,
		Records: records,
		Drift:   DriftAvailability{Status: "not_assessed", ComparableSnapshots: 0, CurrentInventory: false},
		Limitations: []string{
			"These four entries summarize separate historical records, not four Firedoor execution runs.",
			"The speculative Terraform plan was not applied and cannot establish deployed changes.",
			"Historical network witnesses predate the plan and do not form a causal enforcement chain.",
			"Only one approved Kubernetes snapshot is available; no drift comparison or live inventory was performed.",
		},
	}, nil
}
