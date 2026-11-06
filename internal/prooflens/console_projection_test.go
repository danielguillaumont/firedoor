package prooflens

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConsoleProofProjectionPreservesUncertaintyAndMinimizesFields(t *testing.T) {
	correlation, history := reportFixture(t)
	report, err := BuildProofReport(correlation, history)
	if err != nil {
		t.Fatal(err)
	}
	console, err := ProjectConsoleProofReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if console.SchemaVersion != ConsoleProofSchema ||
		console.SourceReportSchema != ProofReportSchema ||
		console.SourceReportSHA256 != report.ReportSHA256 ||
		console.Verdict != Inconclusive ||
		console.ProofChainEstablished || console.IndependentlyAttested ||
		!console.LocalConsistencyOnly || console.Terraform.Applied ||
		console.Kubernetes.CurrentRead || len(console.HistoricalWitnesses) != 2 {
		t.Fatal("projection misrepresented report evidence")
	}
	if console.Terraform.ReceiptID != report.ReceiptID ||
		console.Terraform.Add != report.PlannedChanges.Add ||
		console.Terraform.Change != report.PlannedChanges.Change ||
		console.Terraform.Remove != report.PlannedChanges.Remove ||
		console.Kubernetes.NamespaceCount != report.NamespaceCount ||
		console.Kubernetes.PolicyCount != report.PolicyCount ||
		console.Kubernetes.ReadyPodCount != report.ReadyPodCount {
		t.Fatal("projection drifted from verified report")
	}
	for i, witness := range console.HistoricalWitnesses {
		if !witness.HistoricalOnly || !witness.RestorationVerified || len(witness.Phases) != 3 ||
			witness.RouteID != report.HistoricalWitnesses[i].RouteID {
			t.Fatal("historical witness misrepresented")
		}
		want := []string{"unreachable", "reachable", "unreachable"}
		for j, phase := range witness.Phases {
			if phase.Observed != want[j] {
				t.Fatal("A/B/A witness outcomes changed")
			}
		}
	}
	raw, err := json.Marshal(console)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"report_file", "journal_sha256", "record_sha256", "recovery_events", "experiment_id", "observation_sha256", "correlation_sha256"} {
		if strings.Contains(string(raw), `"`+field+`"`) {
			t.Fatalf("unsafe field included in console projection: %s", field)
		}
	}
	// No caller may mutate the verified report by changing this view model.
	console.Limitations[0] = "overwritten"
	console.HistoricalWitnesses[0].Phases[0].Observed = "reachable"
	if report.Limitations[0] == "overwritten" || report.HistoricalWitnesses[0].Phases[0].Observed != "unreachable" {
		t.Fatal("projected data shares mutable input slices")
	}
}

func TestConsoleProofProjectionRejectsAlteredDetachedReports(t *testing.T) {
	correlation, history := reportFixture(t)
	original, err := BuildProofReport(correlation, history)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*ProofReport)
	}{
		{"unproven-upgraded", func(r *ProofReport) { r.Verdict = "CONTAINED" }},
		{"false-apply", func(r *ProofReport) { r.TerraformApplied = true }},
		{"false-live-probe", func(r *ProofReport) { r.CurrentTrafficProbesRun = true }},
		{"false-causal-chain", func(r *ProofReport) { r.ContemporaneousEvidence = true }},
		{"changed-source-hash", func(r *ProofReport) { r.SourceTreeSHA256 = strings.Repeat("0", 64) }},
		{"changed-plan-count", func(r *ProofReport) { r.PlannedChanges.Change++ }},
		{"file-path-injection", func(r *ProofReport) { r.HistoricalWitnesses[0].ReportFile = "../../outside-secret.json" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copyReport := original
			copyReport.HistoricalWitnesses = append([]HistoricalBehaviorReference(nil), original.HistoricalWitnesses...)
			tc.mutate(&copyReport)
			if _, err := ProjectConsoleProofReport(copyReport); err == nil {
				t.Fatal("accepted altered or overclaimed report")
			}
		})
	}
}
