package recoverygate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func fixture(t *testing.T) (Inputs, pins) {
	t.Helper()
	stage := `C:\Users\Tester\AppData\Local\Firedoor\live-recovery-staging\` + StageLeaf
	source := map[string]string{}
	for i := 0; i < 13; i++ {
		source[string(rune('a'+i))] = strings.Repeat("a", 64)
	}
	marshal := func(v any) []byte {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	intent := stageIntent{SchemaVersion: "firedoor.v10.fd004-live-recovery-stage.v1", Status: "hardened_source_staged_no_infrastructure", SourceCommit: "a22c36e", Cluster: Cluster, Context: Context, Profile: "hardened", KubeconfigPath: stage + `\kubeconfig`, SourceSHA256: source}
	bootstrap := bootstrapReconciliation{SchemaVersion: "firedoor.v10.fd004-bootstrap-reconciliation.v1", Status: "retained_infrastructure_verified_original_receipt_finalization_failed", Cluster: Cluster, Context: Context, SourceCommit: "a22c36e", StagePath: stage, ReconciliationUTC: "2026-11-11T04:00:00Z", InfrastructureObservedReady: true, OriginalBootstrapReceiptStatus: "creation_started_retain_on_failure", PendingBootstrapReceiptStatus: "bootstrap_ready_retained_no_terraform", OriginalReceiptSHA256: strings.Repeat("a", 64), PendingReceiptSHA256: strings.Repeat("b", 64), PrivateKubeconfigSHA256: strings.Repeat("c", 64), CalicoManifestSHA256: strings.Repeat("d", 64), NodeNames: []string{Cluster + "-control-plane", Cluster + "-worker"}, CalicoNodeReadyCount: 2, CalicoControllersReadyCount: 1, CoreDNSReadyCount: 2, OriginalFailure: "receipt rename failed"}
	binary := []byte("synthetic-plan-not-executable")
	plan := []byte(`{"synthetic":true}`)
	p := pins{binary: digest(binary), planJSON: digest(plan), bootstrap: digest(marshal(bootstrap))}
	admitted := admissionReport{SchemaVersion: "firedoor.v10.fd004-retained-plan-admission.v1", Status: "retained_hardened_initial_plan_shape_reviewed_no_apply", Scenario: "FD-004", Cluster: Cluster, Context: Context, Profile: "hardened", SourceCommit: "a22c36e", PlanSHA256: p.planJSON, ReconciliationSHA256: p.bootstrap, ResourcesReviewed: 14, HardenedPoliciesReviewed: 2, HistoricalEvidenceHashMatched: true, Limitations: "offline shape only"}
	p.admission = digest(marshal(admitted))
	receipt := planReconciliation{SchemaVersion: "firedoor.v10.fd004-retained-plan-reconciliation.v1", Status: "retained_plan_shape_admitted_original_plan_runner_failed_no_apply", RecordedUTC: "2026-11-11T05:00:00Z", Cluster: Cluster, Context: Context, Profile: "hardened", OriginalPlanFailure: "literal filename", BinaryPlanSHA256: p.binary, DecodedPlanJSONSHA256: p.planJSON, IndependentAdmissionReportSHA256: p.admission, BootstrapReconciliationSHA256: p.bootstrap, IndependentlyReviewedResources: 14, HardenedPoliciesReviewed: 2, IndependentShapeReviewPassed: true, Note: "not an execution permit"}
	rawReceipt := marshal(receipt)
	p.reconciliation = digest(rawReceipt)
	return Inputs{StagePath: stage, StageIntent: marshal(intent), BootstrapReconciliation: marshal(bootstrap), BinaryPlan: binary, PlanJSON: plan, AdmissionReport: marshal(admitted), PlanReconciliation: rawReceipt}, p
}

func TestHistoricalEvidenceAlwaysBlocked(t *testing.T) {
	in, p := fixture(t)
	result, err := auditWithPins(in, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != Schema || result.Status != Status || !result.DeploymentBlocked || !result.HistoricalEvidenceHashMatched ||
		result.ApplyAuthorized || result.ApplyExecuted || result.CurrentClusterAuthenticated ||
		result.CurrentTerraformStateVerified || result.FreshPlanBoundToState || result.RecoveryProven ||
		result.IndependentlyReviewedResources != 14 || result.HardenedPoliciesReviewed != 2 {
		t.Fatalf("unexpected output: %+v", result)
	}
}

func TestTamperedRawEvidenceRejected(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Inputs)
	}{
		{"binary", func(in *Inputs) { in.BinaryPlan = append(bytes.Clone(in.BinaryPlan), '!') }},
		{"plan JSON", func(in *Inputs) { in.PlanJSON = append(bytes.Clone(in.PlanJSON), '!') }},
		{"admission", func(in *Inputs) { in.AdmissionReport = append(bytes.Clone(in.AdmissionReport), '!') }},
		{"bootstrap", func(in *Inputs) { in.BootstrapReconciliation = append(bytes.Clone(in.BootstrapReconciliation), '!') }},
		{"reconciliation", func(in *Inputs) { in.PlanReconciliation = append(bytes.Clone(in.PlanReconciliation), '!') }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := fixture(t)
			tc.mutate(&in)
			if _, err := auditWithPins(in, p); err == nil {
				t.Fatal("tamper accepted")
			}
		})
	}
}

func TestInvalidStageAndIntentRejected(t *testing.T) {
	t.Run("foreign cluster path", func(t *testing.T) {
		in, p := fixture(t)
		in.StagePath = `C:\Users\Tester\AppData\Local\Firedoor\live-recovery-staging\firedoor-rh-eeeeeeeeeeee`
		if _, err := auditWithPins(in, p); err == nil {
			t.Fatal("foreign stage accepted")
		}
	})
	t.Run("path traversal", func(t *testing.T) {
		in, p := fixture(t)
		in.StagePath += `\..\wrong`
		if _, err := auditWithPins(in, p); err == nil {
			t.Fatal("traversal accepted")
		}
	})
	t.Run("duplicate nested JSON key", func(t *testing.T) {
		in, p := fixture(t)
		in.StageIntent = []byte(`{"schema_version":"ok","schema_version":"evil"}`)
		if _, err := auditWithPins(in, p); err == nil {
			t.Fatal("duplicate key accepted")
		}
	})
	t.Run("unknown authorization key", func(t *testing.T) {
		in, p := fixture(t)
		in.StageIntent = bytes.Replace(in.StageIntent, []byte(`"apply_authorized":false`), []byte(`"apply_authorized":false,"apply_executed":true`), 1)
		if _, err := auditWithPins(in, p); err == nil {
			t.Fatal("unknown field accepted")
		}
	})
	t.Run("stage authorization", func(t *testing.T) {
		in, p := fixture(t)
		in.StageIntent = bytes.Replace(in.StageIntent, []byte(`"apply_authorized":false`), []byte(`"apply_authorized":true`), 1)
		if _, err := auditWithPins(in, p); err == nil {
			t.Fatal("stage permission accepted")
		}
	})
}

func TestForgedHistoricalAuthorizationRejectedDespiteResignedChecksums(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Inputs, *pins)
	}{
		{"bootstrap performed Terraform", func(in *Inputs, p *pins) {
			in.BootstrapReconciliation = bytes.Replace(in.BootstrapReconciliation, []byte(`"terraform_executed":false`), []byte(`"terraform_executed":true`), 1)
			p.bootstrap = digest(in.BootstrapReconciliation)
		}},
		{"admission authorized apply", func(in *Inputs, p *pins) {
			in.AdmissionReport = bytes.Replace(in.AdmissionReport, []byte(`"apply_authorized":false`), []byte(`"apply_authorized":true`), 1)
			p.admission = digest(in.AdmissionReport)
		}},
		{"receipt claims success", func(in *Inputs, p *pins) {
			in.PlanReconciliation = bytes.Replace(in.PlanReconciliation, []byte(`"original_plan_runner_succeeded":false`), []byte(`"original_plan_runner_succeeded":true`), 1)
			p.reconciliation = digest(in.PlanReconciliation)
		}},
		{"receipt claims apply", func(in *Inputs, p *pins) {
			in.PlanReconciliation = bytes.Replace(in.PlanReconciliation, []byte(`"apply_authorized":false`), []byte(`"apply_authorized":true`), 1)
			p.reconciliation = digest(in.PlanReconciliation)
		}},
		{"receipt claims live check", func(in *Inputs, p *pins) {
			in.PlanReconciliation = bytes.Replace(in.PlanReconciliation, []byte(`"cluster_live_rechecked_during_this_recording":false`), []byte(`"cluster_live_rechecked_during_this_recording":true`), 1)
			p.reconciliation = digest(in.PlanReconciliation)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := fixture(t)
			tc.edit(&in, &p)
			if _, err := auditWithPins(in, p); err == nil {
				t.Fatal("forged authorization accepted")
			}
		})
	}
}

func TestProductionPinsCannotBeReplacedBySyntheticEvidence(t *testing.T) {
	in, _ := fixture(t)
	if _, err := Audit(in); err == nil {
		t.Fatal("synthetic evidence accepted by production gate")
	}
}
