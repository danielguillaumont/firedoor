// Package recoveryfreshplan also independently reconciles a completed,
// PRIVATE, fresh Terraform plan. This procedure is read-only and never
// confers ownership of authoritative state or permits applying any plan.
package recoveryfreshplan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	FreshRunSchema    = "firedoor.v10.fd004-fresh-private-plan-session.v1"
	FreshRunStatus    = "fresh_hardened_plan_shape_admitted_no_apply"
	FreshReportSchema = "firedoor.v10.fd004-fresh-plan-reconciliation.v1"
)

var runLeafPattern = regexp.MustCompile(`^\d{8}-\d{9}-[a-f0-9]{12}$`)

// Session describes the exact immutable record written by slice 10C3D2.
type Session struct {
	SchemaVersion                         string `json:"schema_version"`
	Status                                string `json:"status"`
	Cluster                               string `json:"cluster"`
	Context                               string `json:"context"`
	Profile                               string `json:"profile"`
	SourceStageManifestSHA256             string `json:"source_stage_manifest_sha256"`
	SourceStagePath                       string `json:"source_stage_path"`
	OriginalBootstrapReconciliationSHA256 string `json:"original_bootstrap_reconciliation_sha256"`
	HistoricalPlanReconciliationSHA256    string `json:"historical_plan_reconciliation_sha256"`
	SavedPlanSHA256                       string `json:"saved_plan_sha256"`
	PlanJSONSHA256                        string `json:"plan_json_sha256"`
	AdmissionSHA256                       string `json:"admission_sha256"`
	ResourceCount                         int    `json:"resource_count"`
	NetworkPolicyCount                    int    `json:"network_policy_count"`
	LiveClusterChecked                    bool   `json:"live_cluster_checked"`
	TerraformPlanExecuted                 bool   `json:"terraform_plan_executed"`
	LocalManagedStateFilesFound           bool   `json:"local_managed_state_files_found"`
	AuthoritativeStateOwnershipVerified   bool   `json:"authoritative_state_ownership_verified"`
	FreshPlanBoundToAuthoritativeState    bool   `json:"fresh_plan_bound_to_authoritative_state"`
	ApplyAuthorized                       bool   `json:"apply_authorized"`
	ApplyExecuted                         bool   `json:"apply_executed"`
	NetworkRecoveryProven                 bool   `json:"network_recovery_proven"`
	CreatedUTC                            string `json:"created_utc"`
	Limitations                           string `json:"limitations"`
}

type AdmissionSummary struct {
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

type FreshRunReport struct {
	SchemaVersion                 string `json:"schema_version"`
	Status                        string `json:"status"`
	Cluster                       string `json:"cluster"`
	SourceFilesVerified           int    `json:"source_files_verified"`
	SavedPlanSHA256               string `json:"saved_plan_sha256"`
	PlanJSONSHA256                string `json:"plan_json_sha256"`
	SessionSHA256                 string `json:"session_sha256"`
	SourceManifestSHA256          string `json:"source_stage_manifest_sha256"`
	ResourcesReviewed             int    `json:"resources_reviewed"`
	HardenedPoliciesReviewed      int    `json:"hardened_policies_reviewed"`
	SourceOnlyStagePreserved      bool   `json:"source_only_stage_preserved"`
	CurrentStateOwnershipVerified bool   `json:"current_state_ownership_verified"`
	PlanBoundToAuthoritativeState bool   `json:"plan_bound_to_authoritative_state"`
	ApplyAuthorized               bool   `json:"apply_authorized"`
	ApplyExecuted                 bool   `json:"apply_executed"`
	RecoveryProven                bool   `json:"recovery_proven"`
	Limitations                   string `json:"limitations"`
}

func validateSession(s Session, m Manifest, runDir, stageDir, sessionHash, savedHash, manifestHash string) error {
	if s.SchemaVersion != FreshRunSchema || s.Status != FreshRunStatus ||
		s.Cluster != Cluster || s.Context != Context || s.Profile != "hardened" ||
		!samePathReconcile(s.SourceStagePath, stageDir) ||
		!strings.EqualFold(s.SourceStageManifestSHA256, manifestHash) ||
		!strings.EqualFold(s.OriginalBootstrapReconciliationSHA256, DefaultPins.Bootstrap) ||
		!strings.EqualFold(s.HistoricalPlanReconciliationSHA256, DefaultPins.Plan) ||
		!strings.EqualFold(s.SavedPlanSHA256, savedHash) ||
		s.ResourceCount != 14 || s.NetworkPolicyCount != 2 ||
		!s.LiveClusterChecked || !s.TerraformPlanExecuted ||
		s.LocalManagedStateFilesFound || s.AuthoritativeStateOwnershipVerified ||
		s.FreshPlanBoundToAuthoritativeState || s.ApplyAuthorized ||
		s.ApplyExecuted || s.NetworkRecoveryProven || s.Limitations == "" {
		return errors.New("fresh plan session identity, shape, hash or safety claim rejected")
	}
	if !hashName.MatchString(strings.ToLower(s.PlanJSONSHA256)) ||
		!hashName.MatchString(strings.ToLower(s.AdmissionSHA256)) {
		return errors.New("invalid plan or admission hash in session")
	}
	if _, err := time.Parse(time.RFC3339Nano, s.CreatedUTC); err != nil {
		return errors.New("invalid session creation timestamp")
	}
	if m.Cluster != s.Cluster || m.Context != s.Context || m.Profile != s.Profile ||
		!samePathReconcile(m.FreshPath, stageDir) || m.TerraformExecuted ||
		m.StateVerified || m.FreshPlanProduced || m.ApplyAuthorized ||
		m.ApplyExecuted || m.RecoveryProven {
		return errors.New("private source-only manifest no longer represents an unexecuted stage")
	}
	_ = runDir
	_ = sessionHash
	return nil
}

func samePathReconcile(a, b string) bool {
	return filepath.IsAbs(a) && filepath.IsAbs(b) &&
		strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func validateAdmission(a AdmissionSummary, s Session) error {
	if a.SchemaVersion != "firedoor.v10.fd004-retained-plan-admission.v1" ||
		a.Status != "retained_hardened_initial_plan_shape_reviewed_no_apply" ||
		a.Cluster != Cluster || a.Context != Context || a.Profile != "hardened" ||
		a.ResourcesReviewed != 14 || a.HardenedPoliciesReviewed != 2 ||
		!strings.EqualFold(a.PlanSHA256, s.PlanJSONSHA256) ||
		!strings.EqualFold(a.ReconciliationSHA256, DefaultPins.Bootstrap) ||
		!a.HistoricalEvidenceHashMatched || a.ApplyAuthorized || a.RecoveryProven {
		return errors.New("independent historical-shape admission artifact rejected")
	}
	return nil
}

func checkRunPaths(runDir, stageDir string) error {
	local := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(local) {
		return errors.New("LOCALAPPDATA must be absolute")
	}
	private := filepath.Join(local, "Firedoor")
	base := filepath.Join(private, "live-recovery-fresh-plan-runs")
	stageBase := filepath.Join(private, "live-recovery-fresh-plans")
	if !runLeafPattern.MatchString(filepath.Base(runDir)) ||
		!runLeafPattern.MatchString(filepath.Base(stageDir)) ||
		!samePathReconcile(runDir, filepath.Join(base, filepath.Base(runDir))) ||
		!samePathReconcile(stageDir, filepath.Join(stageBase, filepath.Base(stageDir))) ||
		samePathReconcile(runDir, stageDir) {
		return errors.New("run or source-stage private path is not an allowed exact descendant")
	}
	for _, d := range []string{local, private, base, stageBase, runDir, stageDir, filepath.Join(runDir, "terraform"), filepath.Join(runDir, "terraform", "testbed")} {
		if err := plain(d, true); err != nil {
			return fmt.Errorf("private directory: %w", err)
		}
	}
	return nil
}

func checkPlanSourceInventory(runDir string, manifest Manifest) error {
	if len(manifest.SourceSHA256) != len(sourceFiles) {
		return errors.New("wrong Terraform source count")
	}
	seen := map[string]bool{}
	for _, name := range sourceFiles {
		want, ok := manifest.SourceSHA256[name]
		if !ok || !hashName.MatchString(want) {
			return fmt.Errorf("source SHA missing: %s", name)
		}
		rel := strings.TrimPrefix(name, "infrastructure/")
		runFile := filepath.Join(runDir, filepath.FromSlash(rel))
		if err := sameHash(runFile, want); err != nil {
			return fmt.Errorf("source %s: %w", name, err)
		}
		seen[rel] = true
	}
	if err := sameHash(filepath.Join(runDir, "terraform", "testbed", "terraform.tfvars.json"), manifest.TFVarsSHA256); err != nil {
		return fmt.Errorf("tfvars: %w", err)
	}
	seen["terraform/testbed/terraform.tfvars.json"] = true
	for _, name := range []string{"initial-hardened.tfplan", "initial-hardened-plan.json", "fresh-plan-admission.v1.json", "fresh-plan-session.v1.json"} {
		seen[name] = true
	}
	// Providers are in the one Terraform-controlled .terraform tree, which
	// may be large. All other files, including stray plans or state, are denied.
	return filepath.WalkDir(runDir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(runDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		path := filepath.ToSlash(rel)
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked run artifact: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked run artifact: %s", path)
		}
		if d.IsDir() {
			if path == "terraform/testbed/.terraform" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(path, "terraform/testbed/.terraform/") {
				return errors.New("unexpected provider subtree")
			}
			return nil
		}
		if !info.Mode().IsRegular() || !seen[path] {
			return fmt.Errorf("unexpected private run file: %s", path)
		}
		return nil
	})
}

// ReconcileFreshRun only reads existing file bytes. It does not call native
// tools, modify the run or claim backend ownership. Caller must independently
// verify the binary-to-JSON relationship using Terraform show -json.
func ReconcileFreshRun(runDir, stageDir, sessionHash, savedHash, manifestHash string) (FreshRunReport, error) {
	if !hashName.MatchString(strings.ToLower(sessionHash)) ||
		!hashName.MatchString(strings.ToLower(savedHash)) ||
		!hashName.MatchString(strings.ToLower(manifestHash)) {
		return FreshRunReport{}, errors.New("explicit SHA-256 pins required")
	}
	if err := checkRunPaths(runDir, stageDir); err != nil {
		return FreshRunReport{}, err
	}
	if err := sameHash(filepath.Join(stageDir, "manifest.json"), strings.ToLower(manifestHash)); err != nil {
		return FreshRunReport{}, fmt.Errorf("source manifest: %w", err)
	}
	var manifest Manifest
	if err := readJSON(filepath.Join(stageDir, "manifest.json"), &manifest); err != nil {
		return FreshRunReport{}, err
	}
	if manifest.SchemaVersion != Schema || manifest.Status != Status ||
		manifest.RepoCommit != MergedSourceCommit || manifest.SourceCommit != OriginalSourceCommit ||
		manifest.BootstrapReconciliationSHA256 != DefaultPins.Bootstrap ||
		manifest.RetainedPlanReconciliationSHA256 != DefaultPins.Plan {
		return FreshRunReport{}, errors.New("unexpected source manifest origin")
	}
	if err := sameHash(filepath.Join(runDir, "fresh-plan-session.v1.json"), strings.ToLower(sessionHash)); err != nil {
		return FreshRunReport{}, fmt.Errorf("session hash: %w", err)
	}
	if err := sameHash(filepath.Join(runDir, "initial-hardened.tfplan"), strings.ToLower(savedHash)); err != nil {
		return FreshRunReport{}, fmt.Errorf("saved plan hash: %w", err)
	}
	var session Session
	if err := readJSON(filepath.Join(runDir, "fresh-plan-session.v1.json"), &session); err != nil {
		return FreshRunReport{}, err
	}
	if err := validateSession(session, manifest, runDir, stageDir, sessionHash, savedHash, manifestHash); err != nil {
		return FreshRunReport{}, err
	}
	if err := sameHash(filepath.Join(runDir, "initial-hardened-plan.json"), session.PlanJSONSHA256); err != nil {
		return FreshRunReport{}, fmt.Errorf("plan JSON: %w", err)
	}
	if err := sameHash(filepath.Join(runDir, "fresh-plan-admission.v1.json"), session.AdmissionSHA256); err != nil {
		return FreshRunReport{}, fmt.Errorf("admission: %w", err)
	}
	var a AdmissionSummary
	if err := readJSON(filepath.Join(runDir, "fresh-plan-admission.v1.json"), &a); err != nil {
		return FreshRunReport{}, err
	}
	if err := validateAdmission(a, session); err != nil {
		return FreshRunReport{}, err
	}
	if err := checkPlanSourceInventory(runDir, manifest); err != nil {
		return FreshRunReport{}, err
	}
	// Never infer ownership from absence of local state files. Explicitly
	// reject any discovered managed state at known run-root locations.
	for _, p := range []string{"terraform.tfstate", "terraform.tfstate.backup", ".terraform.tfstate.lock.info"} {
		for _, base := range []string{runDir, filepath.Join(runDir, "terraform", "testbed")} {
			if _, err := os.Lstat(filepath.Join(base, p)); err == nil {
				return FreshRunReport{}, errors.New("managed Terraform state appeared; stop")
			} else if !os.IsNotExist(err) {
				return FreshRunReport{}, err
			}
		}
	}
	return FreshRunReport{
		SchemaVersion: FreshReportSchema, Status: "fresh_plan_artifacts_reconciled_no_apply",
		Cluster: Cluster, SourceFilesVerified: len(sourceFiles),
		SavedPlanSHA256: strings.ToLower(savedHash), PlanJSONSHA256: session.PlanJSONSHA256,
		SessionSHA256: strings.ToLower(sessionHash), SourceManifestSHA256: strings.ToLower(manifestHash),
		ResourcesReviewed: 14, HardenedPoliciesReviewed: 2, SourceOnlyStagePreserved: true,
		CurrentStateOwnershipVerified: false, PlanBoundToAuthoritativeState: false,
		ApplyAuthorized: false, ApplyExecuted: false, RecoveryProven: false,
		Limitations: "Historical shape admission and local artifact consistency only; terraform init used -backend=false. Authoritative Terraform state, deployable plan binding and live network enforcement remain UNVERIFIED; DO NOT APPLY.",
	}, nil
}
