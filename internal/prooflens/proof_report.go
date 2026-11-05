package prooflens

// Proof Lens report projection. All inputs are independently validated,
// and historical evidence is presented solely as *context*, never as
// contemporaneous proof of Kubernetes policy enforcement.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ProofReportSchema = "firedoor.v05.proof-report.v1"
	ProofReportMode   = "VERIFIED_LOCAL_EVIDENCE_TIMELINE_READ_ONLY"
	ProofReportStatus = "HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN"
)

var safeReportReceiptID = regexp.MustCompile(`^TFPLAN-[A-Za-z0-9-]{8,121}$`)

type ProofReport struct {
	SchemaVersion             string                        `json:"schema_version"`
	Mode                      string                        `json:"mode"`
	Status                    string                        `json:"status"`
	Verdict                   string                        `json:"verdict"`
	ReceiptID                 string                        `json:"receipt_id"`
	ReceiptSHA256             string                        `json:"receipt_sha256"`
	PlanCapturedUTC           string                        `json:"plan_captured_utc"`
	PlannedChanges            TerraformUIChangeSummary      `json:"planned_changes"`
	SourceTreeSHA256          string                        `json:"source_tree_sha256"`
	Cluster                   string                        `json:"cluster"`
	Context                   string                        `json:"context"`
	Profile                   string                        `json:"profile"`
	ObservationCapturedUTC    string                        `json:"observation_captured_utc"`
	ObservationSHA256         string                        `json:"observation_sha256"`
	NamespaceCount            int                           `json:"namespace_count"`
	ReadyPodCount             int                           `json:"ready_pod_count"`
	PolicyCount               int                           `json:"policy_count"`
	HistoricalWitnesses       []HistoricalBehaviorReference `json:"historical_witnesses"`
	HistoricalWitnessCount    int                           `json:"historical_witness_count"`
	HistoricalContextSHA256   string                        `json:"historical_context_sha256"`
	CorrelationSHA256         string                        `json:"correlation_sha256"`
	TerraformApplied          bool                          `json:"terraform_applied"`
	CurrentTrafficProbesRun   bool                          `json:"current_traffic_probes_run"`
	WitnessExperimentReplayed bool                          `json:"witness_experiment_replayed"`
	ContemporaneousEvidence   bool                          `json:"contemporaneous_evidence"`
	LocalChecksOnly           bool                          `json:"local_checks_only"`
	Limitations               []string                      `json:"limitations"`
	ReportSHA256              string                        `json:"report_sha256"`
}

func proofReportLimitations() []string {
	return []string{
		"A speculative Terraform plan proposes changes; it does not apply infrastructure or establish Kubernetes enforcement.",
		"A fresh Kubernetes inventory is a configuration snapshot, not a live network traffic test.",
		"Historical v0.4 A/B/A HTTP observations predate this plan and are NOT contemporaneous or proof of a single causal chain.",
		"Restoration and journal checks are local record validations; no historical witness is replayed.",
		"SHA256 values only detect local inconsistency and are not signatures, remote attestations, or independently authenticated evidence.",
		"The overall Firedoor Proof Lens security verdict is inconclusive.",
	}
}

func proofReportChecksum(r ProofReport) (string, error) {
	r.ReportSHA256 = ""
	bytes, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(bytes)
	return hex.EncodeToString(h[:]), nil
}

// BuildProofReport refuses unverified or mismatched inputs. It does not run
// commands, access state, read files, alter the cluster, or write evidence.
func BuildProofReport(c ReceiptObservationCorrelation, h HistoricalBehaviorContext) (ProofReport, error) {
	if VerifyReceiptObservationCorrelation(c) != nil || VerifyHistoricalBehaviorContext(h) != nil {
		return ProofReport{}, errors.New("unverified evidence inputs")
	}
	if c.ReceiptID != h.ReceiptID || c.ReceiptSHA256 != h.ReceiptSHA256 ||
		c.SourceTreeSHA256 != h.SourceTreeSHA256 || c.PlanCapturedUTC != h.PlanCapturedUTC ||
		c.ObservationUTC != h.ObservationCapturedUTC || c.ObservationSHA256 != h.ObservationSHA256 ||
		c.CorrelationSHA256 != h.CorrelationSHA256 || c.Cluster != h.Cluster || c.Context != h.Context ||
		c.Profile != h.Profile || !safeReportReceiptID.MatchString(c.ReceiptID) {
		return ProofReport{}, errors.New("timeline has unrelated input records")
	}
	r := ProofReport{
		SchemaVersion: ProofReportSchema, Mode: ProofReportMode, Status: ProofReportStatus,
		Verdict: Inconclusive, ReceiptID: c.ReceiptID, ReceiptSHA256: c.ReceiptSHA256,
		PlanCapturedUTC: c.PlanCapturedUTC, PlannedChanges: c.PlanChanges,
		SourceTreeSHA256: c.SourceTreeSHA256, Cluster: c.Cluster, Context: c.Context, Profile: c.Profile,
		ObservationCapturedUTC: c.ObservationUTC, ObservationSHA256: c.ObservationSHA256,
		NamespaceCount: c.NamespaceCount, ReadyPodCount: c.ReadyPodCount, PolicyCount: c.PolicyCount,
		HistoricalWitnesses:    append([]HistoricalBehaviorReference(nil), h.HistoricalWitnesses...),
		HistoricalWitnessCount: h.HistoricalWitnessCount, HistoricalContextSHA256: h.ContextSHA256,
		CorrelationSHA256: c.CorrelationSHA256, LocalChecksOnly: true, Limitations: proofReportLimitations(),
	}
	var err error
	r.ReportSHA256, err = proofReportChecksum(r)
	if err != nil {
		return ProofReport{}, err
	}
	if err := VerifyProofReport(r); err != nil {
		return ProofReport{}, err
	}
	return r, nil
}

// VerifyProofReport checks a detached projection's internal consistency only;
// it cannot authenticate the local evidence sources. Rebuilding from original
// records is required for a fresh validation.
func VerifyProofReport(r ProofReport) error {
	if r.SchemaVersion != ProofReportSchema || r.Mode != ProofReportMode || r.Status != ProofReportStatus ||
		r.Verdict != Inconclusive || !safeReportReceiptID.MatchString(r.ReceiptID) ||
		!validHex(r.ReceiptSHA256, 64) || !validHex(r.SourceTreeSHA256, 64) ||
		!validHex(r.ObservationSHA256, 64) || !validHex(r.HistoricalContextSHA256, 64) ||
		!validHex(r.CorrelationSHA256, 64) || !validHex(r.ReportSHA256, 64) ||
		r.Cluster != "firedoor-local" || r.Context != "kind-firedoor-local" || r.Profile != "hardened" ||
		r.NamespaceCount != 3 || r.ReadyPodCount != 3 || r.PolicyCount != 2 ||
		r.PlannedChanges.Add < 0 || r.PlannedChanges.Change < 0 || r.PlannedChanges.Remove < 0 ||
		r.HistoricalWitnessCount != 2 || len(r.HistoricalWitnesses) != 2 ||
		r.TerraformApplied || r.CurrentTrafficProbesRun || r.WitnessExperimentReplayed ||
		r.ContemporaneousEvidence || !r.LocalChecksOnly ||
		strings.Join(r.Limitations, "\n") != strings.Join(proofReportLimitations(), "\n") {
		return errors.New("invalid or overstated Proof Lens report")
	}
	pt, err := time.Parse(time.RFC3339Nano, r.PlanCapturedUTC)
	if err != nil {
		return errors.New("invalid plan timestamp")
	}
	ot, err := time.Parse(time.RFC3339Nano, r.ObservationCapturedUTC)
	if err != nil || ot.Before(pt) {
		return errors.New("invalid evidence chronology")
	}
	for i, w := range r.HistoricalWitnesses {
		route := "data-to-app"
		if i == 1 {
			route = "edge-to-data"
		}
		when, e := time.Parse(time.RFC3339Nano, w.CapturedUTC)
		if e != nil || !when.Before(pt) || w.RouteID != route || w.ExperimentID != "witness-"+route ||
			!w.HistoricalOnly || w.RecoveryEvents != 6 || !w.RestorationVerified ||
			!validHex(w.ReportSHA256, 64) || !validHex(w.JournalSHA256, 64) ||
			!validHex(w.RecordSHA256, 64) || !expectedHistoricalPhase(w) {
			return errors.New("invalid historical context in report")
		}
		fromName, nameTime, e := historicalNameInfo(w.ReportFile)
		if e != nil || fromName != route || !when.Truncate(time.Second).Equal(nameTime) {
			return errors.New("historical record filename inconsistent")
		}
	}
	actual, err := proofReportChecksum(r)
	if err != nil || actual != r.ReportSHA256 {
		return errors.New("Proof Lens report SHA256 mismatch")
	}
	return nil
}

func shortProofHash(s string) string { return s[:16] }

// RenderProofReportText writes a fixed-field, color-free terminal dashboard.
// No raw Terraform, Kubernetes, SQL, or HTTP input appears in this output.
func RenderProofReportText(r ProofReport) (string, error) {
	if err := VerifyProofReport(r); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "=================================================================")
	fmt.Fprintln(&b, " FIREDOOR  //  PROOF LENS                 READ-ONLY EVIDENCE")
	fmt.Fprintln(&b, "=================================================================")
	fmt.Fprintln(&b, " VERDICT          : INCONCLUSIVE")
	fmt.Fprintln(&b, " EVIDENCE STATUS  : HISTORICAL CONTEXT ONLY / NOT A PROOF CHAIN")
	fmt.Fprintf(&b, " LAB              : %s  [%s]\n", r.Cluster, r.Profile)
	fmt.Fprintf(&b, " SOURCE SHA-256   : %s...\n", shortProofHash(r.SourceTreeSHA256))
	fmt.Fprintln(&b, "-----------------------------------------------------------------")
	fmt.Fprintln(&b, " 01  TERRAFORM SPECULATIVE PLAN   [NO APPLY]")
	fmt.Fprintf(&b, "     Receipt       %s\n", r.ReceiptID)
	fmt.Fprintf(&b, "     Captured UTC  %s\n", r.PlanCapturedUTC)
	fmt.Fprintf(&b, "     Changes       +%d add / ~%d change / -%d remove\n", r.PlannedChanges.Add, r.PlannedChanges.Change, r.PlannedChanges.Remove)
	fmt.Fprintln(&b, "-----------------------------------------------------------------")
	fmt.Fprintln(&b, " 02  KUBERNETES OBSERVATION       [READ ONLY]")
	fmt.Fprintf(&b, "     Captured UTC  %s\n", r.ObservationCapturedUTC)
	fmt.Fprintf(&b, "     Inventory     %d namespaces / %d ready pods / %d NetworkPolicies\n", r.NamespaceCount, r.ReadyPodCount, r.PolicyCount)
	fmt.Fprintln(&b, "-----------------------------------------------------------------")
	fmt.Fprintln(&b, " 03  HISTORICAL v0.4 A/B/A        [NOT REPLAYED]")
	for _, w := range r.HistoricalWitnesses {
		fmt.Fprintf(&b, "     %-14s unreachable -> reachable -> unreachable\n", w.RouteID)
		fmt.Fprintf(&b, "       %s   recovery events: %d   restored: yes\n", w.CapturedUTC, w.RecoveryEvents)
	}
	fmt.Fprintln(&b, "-----------------------------------------------------------------")
	fmt.Fprintln(&b, " INTEGRITY     : Local hash checks only; NOT authenticated proof")
	fmt.Fprintln(&b, " CAUSALITY     : No Terraform apply or contemporaneous traffic test")
	fmt.Fprintf(&b, " REPORT SHA256 : %s...\n", shortProofHash(r.ReportSHA256))
	fmt.Fprintln(&b, "=================================================================")
	return b.String(), nil
}

// RenderProofReportMarkdown projects the same verified fields as a readable
// Markdown report suitable for later reviewed/export-controlled publication.
func RenderProofReportMarkdown(r ProofReport) (string, error) {
	if err := VerifyProofReport(r); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Firedoor // Proof Lens")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "**Security verdict: INCONCLUSIVE**  ")
	fmt.Fprintln(&b, "**Evidence classification: historical context only — not a causal proof chain**")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- **Owned testbed:** `%s` / `%s` / `%s`\n", r.Cluster, r.Context, r.Profile)
	fmt.Fprintf(&b, "- **Terraform source SHA-256:** `%s`\n", r.SourceTreeSHA256)
	fmt.Fprintln(&b, "- **Collection posture:** read-only correlation; no Terraform apply, network probes, or witness replay")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 1. Terraform planning (speculative only)")
	fmt.Fprintf(&b, "Receipt: `%s`  \n", r.ReceiptID)
	fmt.Fprintf(&b, "Captured (UTC): `%s`  \n", r.PlanCapturedUTC)
	fmt.Fprintf(&b, "Planned changes: **%d add, %d change, %d remove**. No apply is established.\n", r.PlannedChanges.Add, r.PlannedChanges.Change, r.PlannedChanges.Remove)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 2. Kubernetes read-only inventory")
	fmt.Fprintf(&b, "Captured (UTC): `%s`  \n", r.ObservationCapturedUTC)
	fmt.Fprintf(&b, "Observed: **%d namespaces, %d ready pods, %d NetworkPolicies**. Inventory is not network enforcement proof.\n", r.NamespaceCount, r.ReadyPodCount, r.PolicyCount)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 3. Historical v0.4 behavior (not replayed)")
	fmt.Fprintln(&b, "| Route | Captured (UTC) | Before | Witness present | Restored | Recovery events |")
	fmt.Fprintln(&b, "|---|---|---|---|---|---:|")
	for _, w := range r.HistoricalWitnesses {
		fmt.Fprintf(&b, "| `%s` | `%s` | unreachable | reachable | unreachable | %d |\n", w.RouteID, w.CapturedUTC, w.RecoveryEvents)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 4. Evidence limitations")
	for _, limit := range r.Limitations {
		fmt.Fprintf(&b, "- %s\n", limit)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "**Local report SHA-256 (consistency only):** `%s`\n", r.ReportSHA256)
	return b.String(), nil
}
