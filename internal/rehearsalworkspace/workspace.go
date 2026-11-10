// Package rehearsalworkspace validates a staged Terraform/KIND rehearsal workspace.
// It performs file reads ONLY: no Docker, Terraform, kubectl or kind execution.
package rehearsalworkspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Schema = "firedoor.v10.fd004-disposable-workspace.v1"
const Status = "isolated_workspace_prepared_not_executed"

// SourcePaths is the only allowlist of source files eligible to enter a staged
// disposable-lab workspace. All modules are local; remote module sources are forbidden.
var SourcePaths = []string{
	"infrastructure/terraform/testbed/main.tf",
	"infrastructure/terraform/testbed/outputs.tf",
	"infrastructure/terraform/testbed/providers.tf",
	"infrastructure/terraform/testbed/variables.tf",
	"infrastructure/terraform/testbed/versions.tf",
	"infrastructure/terraform/testbed/.terraform.lock.hcl",
	"infrastructure/terraform/modules/workload/main.tf",
	"infrastructure/terraform/modules/workload/outputs.tf",
	"infrastructure/terraform/modules/workload/variables.tf",
	"infrastructure/terraform/modules/network-boundary/main.tf",
	"infrastructure/terraform/modules/network-boundary/outputs.tf",
	"infrastructure/terraform/modules/network-boundary/variables.tf",
	"infrastructure/kind/v09-calico-kind.yaml",
}

type Manifest struct {
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

type Report struct {
	SchemaVersion         string `json:"schema_version"`
	Status                string `json:"status"`
	Cluster               string `json:"cluster"`
	Context               string `json:"context"`
	FilesCompared         int    `json:"files_compared"`
	TerraformStatePresent bool   `json:"terraform_state_present"`
	KubeconfigPresent     bool   `json:"kubeconfig_present"`
	ApplyAuthorized       bool   `json:"apply_authorized"`
	RecoveryTested        bool   `json:"recovery_tested"`
}

var nameRE = regexp.MustCompile(`^firedoor-rh-[a-f0-9]{12}$`)
var shaRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

func stageRelative(src string) (string, error) {
	if src == "infrastructure/kind/v09-calico-kind.yaml" {
		return "kind/cluster.yaml", nil
	}
	if !strings.HasPrefix(src, "infrastructure/terraform/") {
		return "", errors.New("invalid source prefix")
	}
	return strings.TrimPrefix(src, "infrastructure/"), nil
}

func StageRelative(src string) string { result, _ := stageRelative(src); return result }

func secureFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	if info.Size() > 2*1024*1024 {
		return nil, fmt.Errorf("unexpected file size: %s", path)
	}
	return os.ReadFile(path)
}

func isInside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Verify refuses altered files, unexpected workspace contents, state, generated
// kubeconfigs, external modules or any claim that infrastructure was executed.
func Verify(repo, workspace string) (Report, error) {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return Report{}, err
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return Report{}, err
	}
	if isInside(workspace, repo) || isInside(repo, workspace) || repo == workspace {
		return Report{}, errors.New("workspace must be outside source repository")
	}
	ri, err := os.Lstat(workspace)
	if err != nil {
		return Report{}, err
	}
	if !ri.IsDir() || ri.Mode()&os.ModeSymlink != 0 {
		return Report{}, errors.New("workspace not a real directory")
	}
	raw, err := secureFile(filepath.Join(workspace, "intent.json"))
	if err != nil {
		return Report{}, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Report{}, fmt.Errorf("manifest invalid: %w", err)
	}
	if m.SchemaVersion != Schema || m.Status != Status || m.SourceCommit != "8f25ec8" || !nameRE.MatchString(m.Cluster) || m.Context != "kind-"+m.Cluster || m.Profile != "baseline" || m.ClusterCreated || m.ApplyAuthorized || m.RecoveryTested {
		return Report{}, errors.New("manifest identity or safety claim invalid")
	}
	config := filepath.Join(workspace, "kubeconfig")
	if filepath.Clean(m.KubeconfigPath) != config {
		return Report{}, errors.New("private kubeconfig path invalid")
	}
	if len(m.SourceSHA256) != len(SourcePaths) {
		return Report{}, errors.New("source file allowlist cardinality mismatch")
	}
	expected := map[string]bool{"intent.json": true, "terraform/testbed/terraform.tfvars.json": true}
	for _, path := range SourcePaths {
		rel, _ := stageRelative(path)
		expected[rel] = true
		original, err := secureFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			return Report{}, fmt.Errorf("original %s: %w", path, err)
		}
		staged, err := secureFile(filepath.Join(workspace, filepath.FromSlash(rel)))
		if err != nil {
			return Report{}, fmt.Errorf("staged %s: %w", rel, err)
		}
		a := sha256.Sum256(original)
		b := sha256.Sum256(staged)
		hash := hex.EncodeToString(a[:])
		if a != b || !shaRE.MatchString(m.SourceSHA256[path]) || m.SourceSHA256[path] != hash {
			return Report{}, fmt.Errorf("source mismatch: %s", path)
		}
	}
	var tfvars struct {
		Profile string `json:"security_profile"`
		Path    string `json:"kubeconfig_path"`
		Context string `json:"kube_context"`
	}
	data, err := secureFile(filepath.Join(workspace, "terraform", "testbed", "terraform.tfvars.json"))
	if err != nil {
		return Report{}, err
	}
	if err := json.Unmarshal(data, &tfvars); err != nil {
		return Report{}, err
	}
	if tfvars.Profile != "baseline" || filepath.Clean(tfvars.Path) != config || tfvars.Context != m.Context {
		return Report{}, errors.New("terraform variables point outside disposable lab")
	}
	if strings.Contains(string(data), "firedoor-local") {
		return Report{}, errors.New("local cluster reference forbidden")
	}
	allowedDirectories := map[string]bool{}
	for name := range expected {
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "." && dir != ""; dir = filepath.ToSlash(filepath.Dir(dir)) {
			allowedDirectories[dir] = true
		}
	}
	found := map[string]bool{}
	err = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink forbidden: %s", rel)
		}
		if info.IsDir() {
			if !allowedDirectories[filepath.ToSlash(rel)] {
				return fmt.Errorf("unexpected directory in workspace: %s", rel)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not regular file: %s", rel)
		}
		rel = filepath.ToSlash(rel)
		if !expected[rel] {
			return fmt.Errorf("unexpected file in workspace: %s", rel)
		}
		if found[rel] {
			return fmt.Errorf("duplicate file: %s", rel)
		}
		found[rel] = true
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	if len(found) != len(expected) {
		return Report{}, fmt.Errorf("missing staged files: found %d of %d", len(found), len(expected))
	}
	return Report{SchemaVersion: Schema, Status: Status, Cluster: m.Cluster, Context: m.Context, FilesCompared: len(SourcePaths), ApplyAuthorized: false, RecoveryTested: false}, nil
}
