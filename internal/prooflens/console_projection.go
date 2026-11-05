package prooflens

// Console Proof Lens is an intentionally small, inert view model over the
// independently verified v0.5 ProofReport. It NEVER loads files, opens the
// local evidence database, invokes commands or queries Kubernetes.
//
// The projection is historical context, not an attestation or an assertion
// that a security control is enforced at the current time.

import "errors"

const ConsoleProofSchema = "firedoor.v06.console-proof.v1"

type ConsolePlanView struct {
	ReceiptID    string `json:"receipt_id"`
	CapturedUTC  string `json:"captured_utc"`
	SourceSHA256 string `json:"source_sha256"`
	Add          int    `json:"add"`
	Change       int    `json:"change"`
	Remove       int    `json:"remove"`
	Applied      bool   `json:"applied"`
	EvidenceType string `json:"evidence_type"`
}

type ConsoleKubernetesView struct {
	Cluster        string `json:"cluster"`
	Context        string `json:"context"`
	Profile        string `json:"profile"`
	CapturedUTC    string `json:"captured_utc"`
	NamespaceCount int    `json:"namespace_count"`
	ReadyPodCount  int    `json:"ready_pod_count"`
	PolicyCount    int    `json:"policy_count"`
	EvidenceType   string `json:"evidence_type"`
	CurrentRead    bool   `json:"current_read"`
}

type ConsoleWitnessPhase struct {
	Phase    string `json:"phase"`
	Observed string `json:"observed"`
}

type ConsoleHistoricalWitness struct {
	RouteID             string                `json:"route_id"`
	CapturedUTC         string                `json:"captured_utc"`
	HistoricalOnly      bool                  `json:"historical_only"`
	RestorationVerified bool                  `json:"restoration_verified"`
	Phases              []ConsoleWitnessPhase `json:"phases"`
}

type ConsoleProofSummary struct {
	SchemaVersion         string                     `json:"schema_version"`
	SourceReportSchema    string                     `json:"source_report_schema"`
	SourceReportSHA256    string                     `json:"source_report_sha256"`
	Verdict               string                     `json:"verdict"`
	EvidenceStatus        string                     `json:"evidence_status"`
	ProofChainEstablished bool                       `json:"proof_chain_established"`
	IndependentlyAttested bool                       `json:"independently_attested"`
	LocalConsistencyOnly  bool                       `json:"local_consistency_only"`
	Terraform             ConsolePlanView            `json:"terraform"`
	Kubernetes            ConsoleKubernetesView      `json:"kubernetes"`
	HistoricalWitnesses   []ConsoleHistoricalWitness `json:"historical_witnesses"`
	Limitations           []string                   `json:"limitations"`
}

// ProjectConsoleProofReport is a pure projection. Its input must have been
// independently produced by the existing explicit-consent collector (outside
// this function). VerifyProofReport checks DETACHED report consistency and
// cannot authenticate original evidence or prove policy causality.
func ProjectConsoleProofReport(report ProofReport) (ConsoleProofSummary, error) {
	if err := VerifyProofReport(report); err != nil {
		return ConsoleProofSummary{}, errors.New("unverified or overstated detached Proof Lens report")
	}
	witnesses := make([]ConsoleHistoricalWitness, 0, len(report.HistoricalWitnesses))
	for _, witness := range report.HistoricalWitnesses {
		phases := make([]ConsoleWitnessPhase, 0, len(witness.Phases))
		for _, phase := range witness.Phases {
			phases = append(phases, ConsoleWitnessPhase{Phase: phase.Phase, Observed: phase.Observed})
		}
		witnesses = append(witnesses, ConsoleHistoricalWitness{
			RouteID:             witness.RouteID,
			CapturedUTC:         witness.CapturedUTC,
			HistoricalOnly:      witness.HistoricalOnly,
			RestorationVerified: witness.RestorationVerified,
			Phases:              phases,
		})
	}
	return ConsoleProofSummary{
		SchemaVersion:         ConsoleProofSchema,
		SourceReportSchema:    report.SchemaVersion,
		SourceReportSHA256:    report.ReportSHA256,
		Verdict:               report.Verdict,
		EvidenceStatus:        "historical_context_only_not_a_proof_chain",
		ProofChainEstablished: false,
		IndependentlyAttested: false,
		LocalConsistencyOnly:  report.LocalChecksOnly,
		Terraform: ConsolePlanView{
			ReceiptID:    report.ReceiptID,
			CapturedUTC:  report.PlanCapturedUTC,
			SourceSHA256: report.SourceTreeSHA256,
			Add:          report.PlannedChanges.Add,
			Change:       report.PlannedChanges.Change,
			Remove:       report.PlannedChanges.Remove,
			Applied:      report.TerraformApplied,
			EvidenceType: "speculative_plan_not_apply",
		},
		Kubernetes: ConsoleKubernetesView{
			Cluster:        report.Cluster,
			Context:        report.Context,
			Profile:        report.Profile,
			CapturedUTC:    report.ObservationCapturedUTC,
			NamespaceCount: report.NamespaceCount,
			ReadyPodCount:  report.ReadyPodCount,
			PolicyCount:    report.PolicyCount,
			EvidenceType:   "read_only_snapshot_at_collection_time",
			CurrentRead:    false,
		},
		HistoricalWitnesses: witnesses,
		Limitations:         append([]string(nil), report.Limitations...),
	}, nil
}
