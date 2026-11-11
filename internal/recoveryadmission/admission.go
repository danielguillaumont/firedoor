// Package recoveryadmission joins already-supplied FD-004 retained-bootstrap
// evidence to an initial hardened Terraform plan. It is strictly offline.
// Its output never authorizes a plan, apply, Kubernetes change or recovery claim.
package recoveryadmission

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

	"github.com/danielguillaumont/firedoor/internal/deployreview"
	"github.com/danielguillaumont/firedoor/internal/recoveryworkspace"
	"github.com/danielguillaumont/firedoor/internal/rehearsalworkspace"
)

const (
	Schema               = "firedoor.v10.fd004-retained-plan-admission.v1"
	Status               = "retained_hardened_initial_plan_shape_reviewed_no_apply"
	TargetCluster        = "firedoor-rh-3c138b60a075"
	StageLeaf            = "20261110-231253933-3c138b60a075"
	ReconciliationSHA256 = "b124bc70bc93aeaeb907f888fb799a28cead597af23f8fefbe6b7cd3ea5036c2"
	OriginalSHA256       = "c12f03ff7b6c586a9d80a9f4dadec4a0d1b351b2a70db3d5c840867cb7c2ed3b"
	PendingSHA256        = "ec16e6ef3b8d517020aecffbacfdb77f0da41b53e7bf4530b6c8c9311ea4cba3"
	CalicoSHA256         = "2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5"
)

var hashRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var driveRE = regexp.MustCompile(`^[a-zA-Z]:/`)

// Note: the published reconciliation checksum is fixed to one retained
// incident. Tests supply a synthetic expected SHA for offline negative cases.
// This is an integrity pin, not a statement about *current* cluster health.

type Evidence struct {
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

type Report struct {
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

func value(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return errors.New("duplicate or invalid evidence key")
			}
			seen[key] = true
			if err := value(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid evidence object")
		}
	case '[':
		for dec.More() {
			if err := value(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid evidence array")
		}
	default:
		return errors.New("invalid evidence delimiter")
	}
	return nil
}

func strictEvidence(raw []byte, dst any) error {
	if len(raw) == 0 || len(raw) > 128*1024 || bytes.IndexByte(raw, 0) >= 0 {
		return errors.New("evidence absent, oversized or contains NUL")
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	if err := value(tokens); err != nil {
		return err
	}
	if _, err := tokens.Token(); err != io.EOF {
		return errors.New("trailing evidence JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing evidence JSON")
	}
	return nil
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// normalizePath performs lexical comparison only; it never proves a path is
// on disk, not linked, or owned by the claimed cluster.
func normalizePath(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") || strings.Contains(raw, "~") {
		return "", errors.New("invalid private path")
	}
	s := strings.ReplaceAll(raw, "\\", "/")
	if !strings.HasPrefix(s, "/") && !driveRE.MatchString(s) {
		return "", errors.New("private path must be absolute")
	}
	for _, piece := range strings.Split(s, "/") {
		if piece == "." || piece == ".." {
			return "", errors.New("private path traversal")
		}
	}
	clean := strings.ToLower(path.Clean(s))
	if strings.Contains(clean, "//") {
		return "", errors.New("invalid private path")
	}
	return clean, nil
}

func samePath(a, b string) bool {
	x, e1 := normalizePath(a)
	y, e2 := normalizePath(b)
	return e1 == nil && e2 == nil && x == y
}

func stageAllowed(stage string) bool {
	normalized, err := normalizePath(stage)
	if err != nil {
		return false
	}
	return strings.HasSuffix(normalized, "/firedoor/live-recovery-staging/"+StageLeaf)
}

// Review checks only the consistency of operator-provided bytes. The caller
// independently supplies the plan and expected stage. No live context, state,
// HCL source, API endpoint or saved plan is authenticated by this function.
func Review(plan, intentRaw, reconciliationRaw []byte, stage, expectedCluster, expectedReconciliationSHA string) (Report, error) {
	fail := func(s string) (Report, error) { return Report{}, errors.New(s) }
	if expectedCluster != TargetCluster || !stageAllowed(stage) || !hashRE.MatchString(expectedReconciliationSHA) {
		return fail("unrecognized retained incident, stage or expected evidence digest")
	}
	if digest(reconciliationRaw) != expectedReconciliationSHA {
		return fail("reconciliation bytes differ from pinned digest")
	}

	var intent recoveryworkspace.Intent
	if err := strictEvidence(intentRaw, &intent); err != nil {
		return Report{}, fmt.Errorf("stage intent: %w", err)
	}
	if intent.SchemaVersion != recoveryworkspace.Schema || intent.Status != recoveryworkspace.Status ||
		intent.SourceCommit != recoveryworkspace.BaseCommit || intent.Cluster != expectedCluster ||
		intent.Context != "kind-"+expectedCluster || intent.Profile != "hardened" ||
		intent.ClusterCreated || intent.ApplyAuthorized || intent.RecoveryTested ||
		!samePath(intent.KubeconfigPath, stage+"/kubeconfig") {
		return fail("stage identity, profile or intent flags mismatch")
	}
	if len(intent.SourceSHA256) != len(rehearsalworkspace.SourcePaths) {
		return fail("wrong staged source cardinality")
	}
	for _, src := range rehearsalworkspace.SourcePaths {
		if !hashRE.MatchString(intent.SourceSHA256[src]) {
			return fail("missing or invalid staged source hash")
		}
	}

	var r Evidence
	if err := strictEvidence(reconciliationRaw, &r); err != nil {
		return Report{}, fmt.Errorf("reconciliation: %w", err)
	}
	if r.SchemaVersion != "firedoor.v10.fd004-bootstrap-reconciliation.v1" ||
		r.Status != "retained_infrastructure_verified_original_receipt_finalization_failed" ||
		r.Cluster != expectedCluster || r.Context != intent.Context ||
		r.SourceCommit != intent.SourceCommit || !samePath(r.StagePath, stage) ||
		r.OriginalBootstrapRunSucceeded || !r.InfrastructureObservedReady || r.TerraformExecuted || r.NetworkRecoveryTested ||
		r.OriginalBootstrapReceiptStatus != "creation_started_retain_on_failure" ||
		r.PendingBootstrapReceiptStatus != "bootstrap_ready_retained_no_terraform" ||
		r.OriginalReceiptSHA256 != OriginalSHA256 || r.PendingReceiptSHA256 != PendingSHA256 ||
		r.CalicoManifestSHA256 != CalicoSHA256 || !hashRE.MatchString(r.PrivateKubeconfigSHA256) ||
		r.KubernetesVersion != "v1.36.4" || r.CalicoNodeReadyCount != 2 ||
		r.CalicoControllersReadyCount != 1 || r.CoreDNSReadyCount != 2 ||
		len(r.NodeNames) != 2 || r.NodeNames[0] != expectedCluster+"-control-plane" ||
		r.NodeNames[1] != expectedCluster+"-worker" || r.OriginalFailure == "" {
		return fail("retained reconciliation facts or historical safety flags mismatch")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, r.ReconciliationUTC); err != nil || parsed.IsZero() {
		return fail("invalid reconciliation timestamp")
	}

	reviewed, err := deployreview.ReviewJSON(plan, expectedCluster, intent.KubeconfigPath)
	if err != nil {
		return Report{}, fmt.Errorf("initial hardened plan rejected: %w", err)
	}
	if reviewed.ResourceCount != 14 || reviewed.PolicyCount != 2 || reviewed.ApplyAuthorized || reviewed.TerraformAuthenticated || reviewed.ClusterChecked || reviewed.RecoveryProven {
		return fail("initial plan reviewer changed its no-apply contract")
	}
	return Report{
		SchemaVersion: Schema, Status: Status, Scenario: "FD-004", Cluster: expectedCluster,
		Context: intent.Context, Profile: "hardened", SourceCommit: intent.SourceCommit,
		PlanSHA256: digest(plan), ReconciliationSHA256: digest(reconciliationRaw),
		ResourcesReviewed: reviewed.ResourceCount, HardenedPoliciesReviewed: reviewed.PolicyCount,
		HistoricalEvidenceHashMatched: true,
		ApplyAuthorized:               false, TerraformExecuted: false, TerraformAuthenticated: false,
		ClusterChecked: false, RecoveryProven: false,
		Limitations: "Offline review of supplied JSON only. Historical readiness is not current readiness. Does not authenticate live KIND/Kubernetes identity, Terraform state, provider configuration, plan provenance or applyability; does not authorize apply, weaken policies or establish recovery proof.",
	}, nil
}
