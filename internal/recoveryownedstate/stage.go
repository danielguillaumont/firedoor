package recoveryownedstate

// 10C3E2 checks an isolated, source-only deployment-workspace reservation.
// It never initializes Terraform, creates state, or grants apply permission.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/recoveryfreshplan"
)

const (
	StageSchema    = "firedoor.v10.fd004-owned-state-source-stage.v1"
	StageStatus    = "isolated_deployment_source_reserved_no_backend_no_state_no_plan_no_apply"
	FixedFreshLeaf = "20261111-183950682-af260cea2bed"
)

var stageLeaf = regexp.MustCompile(`^\d{8}-\d{9}-[0-9a-f]{12}$`)
var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

type StageManifest struct {
	SchemaVersion             string            `json:"schema_version"`
	Status                    string            `json:"status"`
	Cluster                   string            `json:"cluster"`
	Context                   string            `json:"context"`
	Profile                   string            `json:"profile"`
	SourceStagePath           string            `json:"source_stage_path"`
	OwnedStagePath            string            `json:"owned_stage_path"`
	SourceStageManifestSHA256 string            `json:"source_stage_manifest_sha256"`
	PreviousPlanSessionSHA256 string            `json:"previous_plan_session_sha256"`
	PreviousPlanSHA256        string            `json:"previous_plan_sha256"`
	SourceSHA256              map[string]string `json:"source_sha256"`
	TFVarsSHA256              string            `json:"tfvars_sha256"`
	StateBackendInitialized   bool              `json:"state_backend_initialized"`
	StateOwnershipVerified    bool              `json:"state_ownership_verified"`
	StateBoundPlanGenerated   bool              `json:"state_bound_plan_generated"`
	PreviousPlanDeployable    bool              `json:"previous_plan_deployable"`
	ApplyAuthorized           bool              `json:"apply_authorized"`
	ApplyExecuted             bool              `json:"apply_executed"`
	RecoveryProven            bool              `json:"recovery_proven"`
}

type StageReport struct {
	SchemaVersion           string `json:"schema_version"`
	Status                  string `json:"status"`
	Cluster                 string `json:"cluster"`
	SourceFilesVerified     int    `json:"source_files_verified"`
	ManifestSHA256          string `json:"manifest_sha256"`
	StageVerified           bool   `json:"source_only_stage_verified"`
	StateBackendInitialized bool   `json:"state_backend_initialized"`
	StateOwnershipVerified  bool   `json:"state_ownership_verified"`
	StateBoundPlanGenerated bool   `json:"state_bound_plan_generated"`
	PreviousPlanDeployable  bool   `json:"previous_plan_deployable"`
	ApplyAuthorized         bool   `json:"apply_authorized"`
	DeploymentBlocked       bool   `json:"deployment_blocked"`
}

func stageHash(p string) (string, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 8*1024*1024 {
		return "", errors.New("not a bounded regular file")
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, 8*1024*1024+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func stageHashEqual(p, expected string) error {
	if !sha256Hex.MatchString(expected) {
		return errors.New("invalid SHA-256 pin")
	}
	got, err := stageHash(p)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("SHA-256 mismatch: %s", filepath.Base(p))
	}
	return nil
}
func stageDirPlain(p string) error {
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("not a plain directory: %s", p)
	}
	return nil
}
func stageReadJSON(p string, value any) error {
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 2 || info.Size() > 256*1024 {
		return errors.New("unsafe manifest file")
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 256*1024+1))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		return err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return errors.New("trailing manifest content")
	}
	return nil
}

// VerifyStaged checks only the NEW workspace's file bytes and pinned provenance.
// No Terraform, cluster or state operation is executed.
func VerifyStaged(localAppData, fresh, owned, freshHash string, env []string) (StageReport, error) {
	fail := StageReport{}
	if err := CheckOverrides(env); err != nil {
		return fail, err
	}
	if !filepath.IsAbs(localAppData) || filepath.Clean(localAppData) != localAppData {
		return fail, errors.New("bad LOCALAPPDATA")
	}
	private := filepath.Join(localAppData, "Firedoor")
	freshWant := filepath.Join(private, "live-recovery-fresh-plans", FixedFreshLeaf)
	root := filepath.Join(private, CandidateLeaf)
	if filepath.Clean(fresh) != freshWant || !filepath.IsAbs(owned) ||
		filepath.Dir(filepath.Clean(owned)) != root || !stageLeaf.MatchString(filepath.Base(owned)) {
		return fail, errors.New("foreign workspace path")
	}
	for _, p := range []string{localAppData, private, filepath.Dir(fresh), fresh, root, owned} {
		if err := stageDirPlain(p); err != nil {
			return fail, fmt.Errorf("unsafe staging path: %w", err)
		}
	}
	if err := stageHashEqual(filepath.Join(fresh, "manifest.json"), freshHash); err != nil {
		return fail, fmt.Errorf("source-stage provenance: %w", err)
	}
	var previous recoveryfreshplan.Manifest
	if err := stageReadJSON(filepath.Join(fresh, "manifest.json"), &previous); err != nil {
		return fail, err
	}
	if previous.SchemaVersion != recoveryfreshplan.Schema || previous.Status != recoveryfreshplan.Status ||
		previous.Cluster != Cluster || previous.Context != "kind-"+Cluster || previous.Profile != "hardened" ||
		previous.RepoCommit != recoveryfreshplan.MergedSourceCommit || previous.TerraformExecuted ||
		previous.StateVerified || previous.FreshPlanProduced || previous.ApplyAuthorized || previous.ApplyExecuted || previous.RecoveryProven ||
		previous.FreshPath != fresh {
		return fail, errors.New("source stage is not immutable planning-only evidence")
	}
	var m StageManifest
	manifestPath := filepath.Join(owned, "manifest.json")
	if err := stageReadJSON(manifestPath, &m); err != nil {
		return fail, err
	}
	if m.SchemaVersion != StageSchema || m.Status != StageStatus || m.Cluster != Cluster ||
		m.Context != "kind-"+Cluster || m.Profile != "hardened" || m.SourceStagePath != fresh || m.OwnedStagePath != owned ||
		m.SourceStageManifestSHA256 != freshHash || m.PreviousPlanSessionSHA256 != "7a6264d08998ff48aa329a59abe7b223fdc72079871ddb97503238983fc6532d" ||
		m.PreviousPlanSHA256 != "dd1e9e321484cb71236a6594f53f73041d2da00e0584582d3a05f382da59fb9e" ||
		m.StateBackendInitialized || m.StateOwnershipVerified || m.StateBoundPlanGenerated || m.PreviousPlanDeployable ||
		m.ApplyAuthorized || m.ApplyExecuted || m.RecoveryProven {
		return fail, errors.New("unsafe source-stage identity or ownership claim")
	}
	files := recoveryfreshplan.SourceFiles()
	if len(files) != 12 || len(m.SourceSHA256) != len(files) || len(previous.SourceSHA256) != len(files) ||
		m.TFVarsSHA256 != previous.TFVarsSHA256 {
		return fail, errors.New("invalid file fingerprints")
	}
	seen := map[string]bool{"manifest.json": true, "terraform/testbed/terraform.tfvars.json": true}
	for _, name := range files {
		pin := previous.SourceSHA256[name]
		if !sha256Hex.MatchString(pin) || m.SourceSHA256[name] != pin {
			return fail, fmt.Errorf("source pin differs: %s", name)
		}
		rel := strings.TrimPrefix(name, "infrastructure/")
		if rel == name {
			return fail, errors.New("source path lacks infrastructure prefix")
		}
		seen[rel] = true
		for _, dir := range []string{fresh, owned} {
			if err := stageHashEqual(filepath.Join(dir, filepath.FromSlash(rel)), pin); err != nil {
				return fail, fmt.Errorf("source %s: %w", name, err)
			}
		}
	}
	for _, dir := range []string{fresh, owned} {
		if err := stageHashEqual(filepath.Join(dir, "terraform", "testbed", "terraform.tfvars.json"), m.TFVarsSHA256); err != nil {
			return fail, fmt.Errorf("tfvars: %w", err)
		}
	}
	found := map[string]bool{}
	if err := filepath.WalkDir(owned, func(p string, d fs.DirEntry, readErr error) error {
		if readErr != nil {
			return readErr
		}
		rel, err := filepath.Rel(owned, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		path := filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked entry: %s", path)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || !seen[path] || found[path] {
			return fmt.Errorf("unapproved state/workspace artifact: %s", path)
		}
		found[path] = true
		return nil
	}); err != nil {
		return fail, err
	}
	if len(found) != len(seen) {
		return fail, errors.New("missing staged file")
	}
	mh, err := stageHash(manifestPath)
	if err != nil {
		return fail, err
	}
	return StageReport{SchemaVersion: StageSchema, Status: StageStatus, Cluster: Cluster, SourceFilesVerified: len(files), ManifestSHA256: mh,
		StageVerified: true, DeploymentBlocked: true}, nil
}
