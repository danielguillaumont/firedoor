// Package recoveryownedstate inspects whether a NEW, privately isolated
// Terraform state boundary can be considered for a later explicitly approved
// workflow. Absence of a directory never proves authoritative ownership.
package recoveryownedstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Cluster       = "firedoor-rh-3c138b60a075"
	Schema        = "firedoor.v10.fd004-owned-state-candidate.v1"
	Status        = "candidate_absent_readonly_state_ownership_unverified_no_apply"
	CandidateLeaf = "live-recovery-owned-states"
)

type Report struct {
	SchemaVersion                          string `json:"schema_version"`
	Status                                 string `json:"status"`
	Cluster                                string `json:"cluster"`
	ExistingFreshPlanReconciled            bool   `json:"existing_fresh_plan_reconciled"`
	ExistingFreshPlanDeployable            bool   `json:"existing_fresh_plan_deployable"`
	CandidateRootAbsent                    bool   `json:"candidate_root_absent"`
	CandidateCanBeReviewed                 bool   `json:"candidate_can_be_reviewed"`
	CurrentTerraformStateOwnershipVerified bool   `json:"current_terraform_state_ownership_verified"`
	StateBackendInitialized                bool   `json:"state_backend_initialized"`
	StateBoundPlanGenerated                bool   `json:"state_bound_plan_generated"`
	ApplyAuthorized                        bool   `json:"apply_authorized"`
	ApplyExecuted                          bool   `json:"apply_executed"`
	NetworkRecoveryProven                  bool   `json:"network_recovery_proven"`
	DeploymentBlocked                      bool   `json:"deployment_blocked"`
	NextStep                               string `json:"next_step"`
}

// CheckOverrides rejects ambient inputs that could rebind Terraform or the
// Kubernetes client. It takes a slice so negative tests cannot contaminate
// the process environment.
func CheckOverrides(env []string) error {
	for _, assignment := range env {
		key, _, _ := strings.Cut(assignment, "=")
		k := strings.ToUpper(key)
		if k == "KUBECONFIG" || strings.HasPrefix(k, "TF_") || strings.HasPrefix(k, "KUBE_") {
			return fmt.Errorf("environment override is present: %s", key)
		}
	}
	return nil
}

func plainDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe parent directory: %s", path)
	}
	return nil
}

// CheckCandidate is read-only and rejects ANY existing root, whether it is
// empty, nonempty, linked, or a file. The later allocating workflow must
// separately recheck this boundary and establish new state ownership.
func CheckCandidate(localAppData string, env []string) error {
	if err := CheckOverrides(env); err != nil {
		return err
	}
	if !filepath.IsAbs(localAppData) || filepath.Clean(localAppData) != localAppData {
		return errors.New("LOCALAPPDATA must be a normalized absolute path")
	}
	private := filepath.Join(localAppData, "Firedoor")
	for _, dir := range []string{localAppData, private} {
		if err := plainDir(dir); err != nil {
			return fmt.Errorf("private path boundary: %w", err)
		}
	}
	candidate := filepath.Join(private, CandidateLeaf)
	if _, err := os.Lstat(candidate); err == nil {
		return errors.New("private owned-state root already exists; preserve and investigate")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot establish candidate absence: %w", err)
	}
	return nil
}

// Eligible means only that a prospective parent is unused and the separate
// *historical planning* evidence has been reconciled. It never means that
// Terraform state has been created or is authoritative.
func Eligible(localAppData string, env []string, historicalPlanReconciled bool) (Report, error) {
	if !historicalPlanReconciled {
		return Report{}, errors.New("historical plan reconciliation not independently verified")
	}
	if err := CheckCandidate(localAppData, env); err != nil {
		return Report{}, err
	}
	return Report{
		SchemaVersion:                          Schema,
		Status:                                 Status,
		Cluster:                                Cluster,
		ExistingFreshPlanReconciled:            true,
		ExistingFreshPlanDeployable:            false,
		CandidateRootAbsent:                    true,
		CandidateCanBeReviewed:                 true,
		CurrentTerraformStateOwnershipVerified: false,
		StateBackendInitialized:                false,
		StateBoundPlanGenerated:                false,
		ApplyAuthorized:                        false,
		ApplyExecuted:                          false,
		NetworkRecoveryProven:                  false,
		DeploymentBlocked:                      true,
		NextStep:                               "recheck actual cluster identity and all known state owners; independently create a new private state backend with explicit authorization, then generate and admit a NEW state-bound plan; never reuse prior planning-only plan",
	}, nil
}
