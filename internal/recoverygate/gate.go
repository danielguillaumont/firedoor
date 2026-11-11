// Package recoverygate audits retained FD-004 evidence before any deployment design.
// The historical saved plan is NEVER promoted to an apply authorization.
// This package is offline: no filesystem, Terraform or Kubernetes side effects.
package recoverygate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	Cluster   = "firedoor-rh-3c138b60a075"
	Context   = "kind-" + Cluster
	StageLeaf = "20261110-231253933-3c138b60a075"
	Schema    = "firedoor.v10.fd004-controlled-deployment-readonly-gate.v1"
	Status    = "historical_plan_integrity_verified_deployment_blocked"

	binaryPlanSHA         = "8bfcb2f17d8bd96cd3faeed5505284ea5808f3e15d6be95889bed9bcd62dff5e"
	planJSONSHA           = "17d803a909d90d3bf904a559bba2851397e7f935b9a4c69d9c4570f533242c99"
	admissionSHA          = "eb14d8a724a3ef9b95f9f43aa722f14ac084b328358413542695f5750b1d5ba5"
	bootstrapSHA          = "b124bc70bc93aeaeb907f888fb799a28cead597af23f8fefbe6b7cd3ea5036c2"
	planReconciliationSHA = "7213cfbd02b341704ab24af4ccd5a7bdb5154767509ea64feda3aa4a0b9e3f60"
)

var hashRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var driveRE = regexp.MustCompile(`^[a-zA-Z]:/`)

type Inputs struct {
	StagePath               string
	StageIntent             []byte
	BootstrapReconciliation []byte
	BinaryPlan              []byte
	PlanJSON                []byte
	AdmissionReport         []byte
	PlanReconciliation      []byte
}

type pins struct{ binary, planJSON, admission, bootstrap, reconciliation string }

var retainedPins = pins{binaryPlanSHA, planJSONSHA, admissionSHA, bootstrapSHA, planReconciliationSHA}

type Report struct {
	SchemaVersion                  string `json:"schema_version"`
	Status                         string `json:"status"`
	Cluster                        string `json:"cluster"`
	Context                        string `json:"context"`
	Profile                        string `json:"profile"`
	HistoricalPlanSHA256           string `json:"historical_plan_sha256"`
	HistoricalPlanRunnerSucceeded  bool   `json:"historical_plan_runner_succeeded"`
	IndependentlyReviewedResources int    `json:"independently_reviewed_resources"`
	HardenedPoliciesReviewed       int    `json:"hardened_policies_reviewed"`
	HistoricalEvidenceHashMatched  bool   `json:"historical_evidence_hash_matched"`
	CurrentClusterAuthenticated    bool   `json:"current_cluster_authenticated"`
	CurrentTerraformStateVerified  bool   `json:"current_terraform_state_verified"`
	FreshPlanBoundToState          bool   `json:"fresh_plan_bound_to_state"`
	ApplyAuthorized                bool   `json:"apply_authorized"`
	ApplyExecuted                  bool   `json:"apply_executed"`
	RecoveryProven                 bool   `json:"recovery_proven"`
	DeploymentBlocked              bool   `json:"deployment_blocked"`
	NextBoundary                   string `json:"next_boundary"`
}

type stageIntent struct {
	SchemaVersion   string            `json:"schema_version"`
	Status          string            `json:"status"`
	SourceCommit    string            `json:"source_commit"`
	Cluster         string            `json:"cluster"`
	Context         string            `json:"context"`
	KubeconfigPath  string            `json:"kubeconfig_path"`
	Profile         string            `json:"profile"`
	ClusterCreated  bool              `json:"cluster_created"`
	ApplyAuthorized bool              `json:"apply_authorized"`
	RecoveryTested  bool              `json:"recovery_tested"`
	SourceSHA256    map[string]string `json:"source_sha256"`
}

type bootstrapReconciliation struct {
	SchemaVersion                  string   `json:"schema_version"`
	Status                         string   `json:"status"`
	Cluster                        string   `json:"cluster"`
	Context                        string   `json:"context"`
	SourceCommit                   string   `json:"source_commit"`
	StagePath                      string   `json:"stage_path"`
	ReconciliationUTC              string   `json:"reconciliation_utc"`
	OriginalBootstrapRunSucceeded  bool     `json:"original_bootstrap_run_succeeded"`
	InfrastructureObservedReady    bool     `json:"infrastructure_observed_ready"`
	OriginalBootstrapReceiptStatus string   `json:"original_bootstrap_receipt_status"`
	PendingBootstrapReceiptStatus  string   `json:"pending_bootstrap_receipt_status"`
	OriginalReceiptSHA256          string   `json:"original_receipt_sha256"`
	PendingReceiptSHA256           string   `json:"pending_receipt_sha256"`
	PrivateKubeconfigSHA256        string   `json:"private_kubeconfig_sha256"`
	CalicoManifestSHA256           string   `json:"calico_manifest_sha256"`
	KubernetesVersion              string   `json:"kubernetes_version"`
	NodeNames                      []string `json:"node_names"`
	CalicoNodeReadyCount           int      `json:"calico_node_ready_count"`
	CalicoControllersReadyCount    int      `json:"calico_controllers_ready_count"`
	CoreDNSReadyCount              int      `json:"coredns_ready_count"`
	TerraformExecuted              bool     `json:"terraform_executed"`
	NetworkRecoveryTested          bool     `json:"network_recovery_tested"`
	OriginalFailure                string   `json:"original_failure"`
	Note                           string   `json:"note"`
}

type admissionReport struct {
	SchemaVersion                 string `json:"schema_version"`
	Status                        string `json:"status"`
	Scenario                      string `json:"scenario"`
	Cluster                       string `json:"cluster"`
	Context                       string `json:"context"`
	Profile                       string `json:"profile"`
	SourceCommit                  string `json:"source_commit"`
	PlanSHA256                    string `json:"plan_sha256"`
	ReconciliationSHA256          string `json:"reconciliation_sha256"`
	ResourcesReviewed             int    `json:"resources_reviewed"`
	HardenedPoliciesReviewed      int    `json:"hardened_policies_reviewed"`
	HistoricalEvidenceHashMatched bool   `json:"historical_evidence_hash_matched"`
	ApplyAuthorized               bool   `json:"apply_authorized"`
	TerraformExecuted             bool   `json:"terraform_executed"`
	TerraformAuthenticated        bool   `json:"terraform_authenticated"`
	ClusterChecked                bool   `json:"cluster_checked"`
	RecoveryProven                bool   `json:"recovery_proven"`
	Limitations                   string `json:"limitations"`
}

type planReconciliation struct {
	SchemaVersion                           string `json:"schema_version"`
	Status                                  string `json:"status"`
	RecordedUTC                             string `json:"recorded_utc"`
	Cluster                                 string `json:"cluster"`
	Context                                 string `json:"context"`
	Profile                                 string `json:"profile"`
	OriginalPlanRunnerSucceeded             bool   `json:"original_plan_runner_succeeded"`
	OriginalPlanFailure                     string `json:"original_plan_failure"`
	BinaryPlanSHA256                        string `json:"binary_plan_sha256"`
	DecodedPlanJSONSHA256                   string `json:"decoded_plan_json_sha256"`
	IndependentAdmissionReportSHA256        string `json:"independent_admission_report_sha256"`
	BootstrapReconciliationSHA256           string `json:"bootstrap_reconciliation_sha256"`
	IndependentlyReviewedResources          int    `json:"independently_reviewed_resources"`
	HardenedPoliciesReviewed                int    `json:"hardened_policies_reviewed"`
	IndependentShapeReviewPassed            bool   `json:"independent_shape_review_passed"`
	ClusterLiveRecheckedDuringThisRecording bool   `json:"cluster_live_rechecked_during_this_recording"`
	TerraformPlanReexecuted                 bool   `json:"terraform_plan_reexecuted"`
	TerraformApplyExecuted                  bool   `json:"terraform_apply_executed"`
	ApplyAuthorized                         bool   `json:"apply_authorized"`
	NetworkRecoveryProven                   bool   `json:"network_recovery_proven"`
	Note                                    string `json:"note"`
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Walk every JSON value to reject duplicate property names at any depth.
func checkValue(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return errors.New("invalid or duplicated JSON property")
			}
			seen[key] = true
			if err := checkValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid JSON object")
		}
	case '[':
		for d.More() {
			if err := checkValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid JSON array")
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	return nil
}

func strictDecode(raw []byte, dst any) error {
	if len(raw) == 0 || len(raw) > 128*1024 || bytes.IndexByte(raw, 0) != -1 {
		return errors.New("missing or unbounded evidence JSON")
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	if err := checkValue(tokens); err != nil {
		return err
	}
	if _, err := tokens.Token(); err != io.EOF {
		return errors.New("trailing JSON token")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON evidence")
	}
	return nil
}

func normalizePath(v string) (string, error) {
	if v == "" || strings.ContainsAny(v, "\x00\r\n~") {
		return "", errors.New("invalid path")
	}
	s := strings.ReplaceAll(v, "\\", "/")
	if !strings.HasPrefix(s, "/") && !driveRE.MatchString(s) {
		return "", errors.New("path not absolute")
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." {
			return "", errors.New("path traversal")
		}
	}
	return strings.ToLower(path.Clean(s)), nil
}
func samePath(a, b string) bool {
	x, e1 := normalizePath(a)
	y, e2 := normalizePath(b)
	return e1 == nil && e2 == nil && x == y
}

// Audit always returns deployment_blocked=true on success; it cannot grant a
// permit. Bad or altered evidence is an error, not an approval-ready result.
func Audit(in Inputs) (Report, error) { return auditWithPins(in, retainedPins) }

func auditWithPins(in Inputs, p pins) (Report, error) {
	fail := func(msg string) (Report, error) { return Report{}, errors.New(msg) }
	stage, err := normalizePath(in.StagePath)
	if err != nil || !strings.HasSuffix(stage, "/firedoor/live-recovery-staging/"+StageLeaf) {
		return fail("unrecognized retained stage path")
	}
	if len(in.BinaryPlan) != 10424 && p.binary == binaryPlanSHA {
		return fail("historical binary plan size mismatch")
	}
	for _, check := range []struct{ label, hash, expected string }{
		{"binary plan", digest(in.BinaryPlan), p.binary},
		{"decoded plan", digest(in.PlanJSON), p.planJSON},
		{"independent admission", digest(in.AdmissionReport), p.admission},
		{"bootstrap reconciliation", digest(in.BootstrapReconciliation), p.bootstrap},
		{"plan reconciliation", digest(in.PlanReconciliation), p.reconciliation},
	} {
		if !hashRE.MatchString(check.expected) || check.hash != check.expected {
			return fail(check.label + " differs from pinned SHA256")
		}
	}
	if len(in.PlanJSON) == 0 || len(in.PlanJSON) > 8*1024*1024 {
		return fail("decoded plan size invalid")
	}
	// Shape admission previously reviewed the pinned bytes, but this gate does
	// not parse Terraform JSON into an executable/authorized plan.
	var intent stageIntent
	if err := strictDecode(in.StageIntent, &intent); err != nil {
		return Report{}, fmt.Errorf("stage intent: %w", err)
	}
	if intent.SchemaVersion != "firedoor.v10.fd004-live-recovery-stage.v1" ||
		intent.Status != "hardened_source_staged_no_infrastructure" || intent.SourceCommit != "a22c36e" ||
		intent.Cluster != Cluster || intent.Context != Context || intent.Profile != "hardened" ||
		intent.ClusterCreated || intent.ApplyAuthorized || intent.RecoveryTested ||
		!samePath(intent.KubeconfigPath, in.StagePath+"/kubeconfig") || len(intent.SourceSHA256) != 13 {
		return fail("stage incident identity or permission flags mismatch")
	}
	for _, v := range intent.SourceSHA256 {
		if !hashRE.MatchString(v) {
			return fail("invalid stage source digest")
		}
	}

	var bootstrap bootstrapReconciliation
	if err := strictDecode(in.BootstrapReconciliation, &bootstrap); err != nil {
		return Report{}, fmt.Errorf("bootstrap: %w", err)
	}
	if bootstrap.SchemaVersion != "firedoor.v10.fd004-bootstrap-reconciliation.v1" ||
		bootstrap.Status != "retained_infrastructure_verified_original_receipt_finalization_failed" ||
		bootstrap.Cluster != Cluster || bootstrap.Context != Context || bootstrap.SourceCommit != intent.SourceCommit ||
		!samePath(bootstrap.StagePath, in.StagePath) || bootstrap.OriginalBootstrapRunSucceeded ||
		!bootstrap.InfrastructureObservedReady || bootstrap.TerraformExecuted || bootstrap.NetworkRecoveryTested ||
		bootstrap.OriginalBootstrapReceiptStatus != "creation_started_retain_on_failure" ||
		bootstrap.PendingBootstrapReceiptStatus != "bootstrap_ready_retained_no_terraform" ||
		!hashRE.MatchString(bootstrap.OriginalReceiptSHA256) || !hashRE.MatchString(bootstrap.PendingReceiptSHA256) ||
		!hashRE.MatchString(bootstrap.PrivateKubeconfigSHA256) || !hashRE.MatchString(bootstrap.CalicoManifestSHA256) ||
		len(bootstrap.NodeNames) != 2 || bootstrap.NodeNames[0] != Cluster+"-control-plane" ||
		bootstrap.NodeNames[1] != Cluster+"-worker" || bootstrap.CalicoNodeReadyCount != 2 ||
		bootstrap.CalicoControllersReadyCount != 1 || bootstrap.CoreDNSReadyCount != 2 || bootstrap.OriginalFailure == "" {
		return fail("bootstrap reconciliation status or historical facts mismatch")
	}
	if t, err := time.Parse(time.RFC3339Nano, bootstrap.ReconciliationUTC); err != nil || t.IsZero() {
		return fail("bootstrap reconciliation time invalid")
	}

	var admitted admissionReport
	if err := strictDecode(in.AdmissionReport, &admitted); err != nil {
		return Report{}, fmt.Errorf("admission: %w", err)
	}
	if admitted.SchemaVersion != "firedoor.v10.fd004-retained-plan-admission.v1" ||
		admitted.Status != "retained_hardened_initial_plan_shape_reviewed_no_apply" ||
		admitted.Scenario != "FD-004" || admitted.Cluster != Cluster || admitted.Context != Context ||
		admitted.Profile != "hardened" || admitted.SourceCommit != intent.SourceCommit ||
		admitted.PlanSHA256 != p.planJSON || admitted.ReconciliationSHA256 != p.bootstrap ||
		admitted.ResourcesReviewed != 14 || admitted.HardenedPoliciesReviewed != 2 ||
		!admitted.HistoricalEvidenceHashMatched || admitted.ApplyAuthorized || admitted.TerraformExecuted ||
		admitted.TerraformAuthenticated || admitted.ClusterChecked || admitted.RecoveryProven || admitted.Limitations == "" {
		return fail("admission shape or execution permissions mismatch")
	}
	var reconciled planReconciliation
	if err := strictDecode(in.PlanReconciliation, &reconciled); err != nil {
		return Report{}, fmt.Errorf("plan reconciliation: %w", err)
	}
	if reconciled.SchemaVersion != "firedoor.v10.fd004-retained-plan-reconciliation.v1" ||
		reconciled.Status != "retained_plan_shape_admitted_original_plan_runner_failed_no_apply" ||
		reconciled.Cluster != Cluster || reconciled.Context != Context || reconciled.Profile != "hardened" ||
		reconciled.OriginalPlanRunnerSucceeded || reconciled.OriginalPlanFailure == "" ||
		reconciled.BinaryPlanSHA256 != p.binary || reconciled.DecodedPlanJSONSHA256 != p.planJSON ||
		reconciled.IndependentAdmissionReportSHA256 != p.admission ||
		reconciled.BootstrapReconciliationSHA256 != p.bootstrap ||
		reconciled.IndependentlyReviewedResources != 14 || reconciled.HardenedPoliciesReviewed != 2 ||
		!reconciled.IndependentShapeReviewPassed || reconciled.ClusterLiveRecheckedDuringThisRecording ||
		reconciled.TerraformPlanReexecuted || reconciled.TerraformApplyExecuted ||
		reconciled.ApplyAuthorized || reconciled.NetworkRecoveryProven || reconciled.Note == "" {
		return fail("plan reconciliation facts or execution flags mismatch")
	}
	if t, err := time.Parse(time.RFC3339Nano, reconciled.RecordedUTC); err != nil || t.IsZero() {
		return fail("plan reconciliation time invalid")
	}
	return Report{
		SchemaVersion: Schema, Status: Status, Cluster: Cluster, Context: Context,
		Profile: "hardened", HistoricalPlanSHA256: p.binary,
		HistoricalPlanRunnerSucceeded: false, IndependentlyReviewedResources: 14,
		HardenedPoliciesReviewed: 2, HistoricalEvidenceHashMatched: true,
		CurrentClusterAuthenticated: false, CurrentTerraformStateVerified: false,
		FreshPlanBoundToState: false, ApplyAuthorized: false, ApplyExecuted: false,
		RecoveryProven: false, DeploymentBlocked: true,
		NextBoundary: "Requires separate live cluster ownership and state authentication, a fresh attributable saved Terraform plan, re-admission, and explicit deployment approval. Never apply the historical failed-run plan.",
	}, nil
}
