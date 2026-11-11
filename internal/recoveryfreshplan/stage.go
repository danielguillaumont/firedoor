// Package recoveryfreshplan independently verifies a source-only, isolated
// FD-004 Terraform planning workspace. It never executes Terraform or grants
// deployment permission.
package recoveryfreshplan

import (
	"bytes"
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
	"sort"
	"strings"
)

const (
	Cluster              = "firedoor-rh-3c138b60a075"
	Context              = "kind-" + Cluster
	StageLeaf            = "20261110-231253933-3c138b60a075"
	OriginalSourceCommit = "a22c36e"
	MergedSourceCommit   = "60a227d"
	Schema               = "firedoor.v10.fd004-fresh-plan-source-stage.v1"
	Status               = "isolated_hardened_plan_source_prepared_no_state_no_plan_no_apply"
)

var sourceFiles = []string{
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
}

const kindSource = "infrastructure/kind/v09-calico-kind.yaml"

var freshName = regexp.MustCompile(`^\d{8}-\d{9}-[a-f0-9]{12}$`)
var hashName = regexp.MustCompile(`^[a-f0-9]{64}$`)
var backendDeclaration = regexp.MustCompile(`(?m)\bbackend\s*(?:"|\{)|(?m)^\s*cloud\s*\{`)

// Pins identify the immutable, separate incidents. The CLI always uses
// DefaultPins; tests may use synthetic fixture fingerprints.
type Pins struct{ Bootstrap, Plan string }

var DefaultPins = Pins{
	Bootstrap: "b124bc70bc93aeaeb907f888fb799a28cead597af23f8fefbe6b7cd3ea5036c2",
	Plan:      "7213cfbd02b341704ab24af4ccd5a7bdb5154767509ea64feda3aa4a0b9e3f60",
}

type Manifest struct {
	SchemaVersion                    string            `json:"schema_version"`
	Status                           string            `json:"status"`
	Cluster                          string            `json:"cluster"`
	Context                          string            `json:"context"`
	Profile                          string            `json:"profile"`
	SourceCommit                     string            `json:"source_commit"`
	RepoCommit                       string            `json:"repo_commit"`
	StagePath                        string            `json:"stage_path"`
	FreshPath                        string            `json:"fresh_path"`
	KubeconfigPath                   string            `json:"kubeconfig_path"`
	SourceSHA256                     map[string]string `json:"source_sha256"`
	TFVarsSHA256                     string            `json:"tfvars_sha256"`
	StageIntentSHA256                string            `json:"stage_intent_sha256"`
	BootstrapReconciliationSHA256    string            `json:"bootstrap_reconciliation_sha256"`
	RetainedPlanReconciliationSHA256 string            `json:"retained_plan_reconciliation_sha256"`
	TerraformExecuted                bool              `json:"terraform_executed"`
	StateVerified                    bool              `json:"terraform_state_verified"`
	FreshPlanProduced                bool              `json:"fresh_plan_produced"`
	ApplyAuthorized                  bool              `json:"apply_authorized"`
	ApplyExecuted                    bool              `json:"apply_executed"`
	RecoveryProven                   bool              `json:"recovery_proven"`
}
type intent struct {
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
type vars struct {
	SecurityProfile string `json:"security_profile"`
	KubeconfigPath  string `json:"kubeconfig_path"`
	KubeContext     string `json:"kube_context"`
}
type bootstrap struct {
	Cluster                       string `json:"cluster"`
	Context                       string `json:"context"`
	OriginalBootstrapRunSucceeded bool   `json:"original_bootstrap_run_succeeded"`
	InfrastructureObservedReady   bool   `json:"infrastructure_observed_ready"`
	PrivateKubeconfigSHA256       string `json:"private_kubeconfig_sha256"`
}
type planReconciliation struct {
	Cluster         string `json:"cluster"`
	ApplyAuthorized bool   `json:"apply_authorized"`
	ApplyExecuted   bool   `json:"apply_executed"`
}
type Report struct {
	SchemaVersion          string `json:"schema_version"`
	Status                 string `json:"status"`
	Cluster                string `json:"cluster"`
	FilesVerified          int    `json:"files_verified"`
	WorkspaceSHA256        string `json:"workspace_manifest_sha256"`
	TerraformStateVerified bool   `json:"terraform_state_verified"`
	FreshPlanProduced      bool   `json:"fresh_plan_produced"`
	ApplyAuthorized        bool   `json:"apply_authorized"`
	ApplyExecuted          bool   `json:"apply_executed"`
	DeploymentBlocked      bool   `json:"deployment_blocked"`
}

func plain(p string, isDir bool) error {
	st, err := os.Lstat(p)
	if err != nil {
		return fmt.Errorf("missing path: %w", err)
	}
	if st.Mode()&os.ModeSymlink != 0 || st.IsDir() != isDir {
		return fmt.Errorf("linked or incorrect path type: %s", p)
	}
	if !isDir && !st.Mode().IsRegular() {
		return fmt.Errorf("non-regular file: %s", p)
	}
	return nil
}
func fingerprint(p string) (string, error) {
	if err := plain(p, false); err != nil {
		return "", err
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
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Size() > 8*1024*1024 {
		return "", errors.New("oversized input")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func sameHash(p, want string) error {
	got, err := fingerprint(p)
	if err != nil {
		return err
	}
	if !hashName.MatchString(want) || got != want {
		return fmt.Errorf("fingerprint mismatch: %s", p)
	}
	return nil
}
func strictValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	x, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch x {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return errors.New("duplicate or invalid JSON key")
			}
			seen[s] = true
			if err = strictValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("malformed object")
		}
	case '[':
		for d.More() {
			if err = strictValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("malformed array")
		}
	default:
		return errors.New("bad JSON delimiter")
	}
	return nil
}
func readJSON(p string, target any) error {
	if err := plain(p, false); err != nil {
		return err
	}
	st, err := os.Stat(p)
	if err != nil {
		return err
	}
	if st.Size() < 2 || st.Size() > 256*1024 {
		return errors.New("JSON input size outside bounds")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return errors.New("NUL JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err = strictValue(d); err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("JSON trailing input")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return err
	}
	return nil
}
func checkInventory(root string, expected map[string]bool) error {
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in workspace: %s", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked entry: %s", rel)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file: %s", rel)
		}
		key := filepath.ToSlash(rel)
		if !expected[key] {
			return fmt.Errorf("unexpected workspace file: %s", key)
		}
		if found[key] {
			return fmt.Errorf("duplicate workspace file: %s", key)
		}
		found[key] = true
		return nil
	})
	if err != nil {
		return err
	}
	if len(found) != len(expected) {
		return fmt.Errorf("workspace inventory missing files: got %d, expected %d", len(found), len(expected))
	}
	return nil
}
func requirePath(p string, base string) error {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path escapes isolated private root")
	}
	return nil
}
func Verify(repo, stage, fresh string, pins Pins) (Report, error) {
	var failed Report
	if !hashName.MatchString(pins.Bootstrap) || !hashName.MatchString(pins.Plan) {
		return failed, errors.New("invalid evidence pins")
	}
	local := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(local) {
		return failed, errors.New("LOCALAPPDATA is not absolute")
	}
	base := filepath.Join(local, "Firedoor")
	stageWant := filepath.Join(base, "live-recovery-staging", StageLeaf)
	if filepath.Clean(stage) != stageWant {
		return failed, errors.New("wrong retained stage")
	}
	freshBase := filepath.Join(base, "live-recovery-fresh-plans")
	if err := requirePath(fresh, freshBase); err != nil {
		return failed, err
	}
	if filepath.Dir(filepath.Clean(fresh)) != freshBase || !freshName.MatchString(filepath.Base(fresh)) {
		return failed, errors.New("unexpected fresh workspace name")
	}
	if !filepath.IsAbs(repo) || !filepath.IsAbs(fresh) || fresh == stage {
		return failed, errors.New("workspace path must be absolute and separate")
	}
	for _, p := range []string{local, base, filepath.Join(base, "live-recovery-staging"), stage, filepath.Join(base, "live-recovery-bootstraps"), filepath.Join(base, "live-recovery-plans"), freshBase, fresh, repo} {
		if err := plain(p, true); err != nil {
			return failed, err
		}
	}
	manifestPath := filepath.Join(fresh, "manifest.json")
	var m Manifest
	if err := readJSON(manifestPath, &m); err != nil {
		return failed, fmt.Errorf("manifest: %w", err)
	}
	if m.SchemaVersion != Schema || m.Status != Status || m.Cluster != Cluster || m.Context != Context || m.Profile != "hardened" || m.SourceCommit != OriginalSourceCommit || m.RepoCommit != MergedSourceCommit || m.StagePath != stage || m.FreshPath != fresh || m.TerraformExecuted || m.StateVerified || m.FreshPlanProduced || m.ApplyAuthorized || m.ApplyExecuted || m.RecoveryProven {
		return failed, errors.New("workspace manifest identity or claims rejected")
	}
	if m.BootstrapReconciliationSHA256 != pins.Bootstrap || m.RetainedPlanReconciliationSHA256 != pins.Plan {
		return failed, errors.New("manifest incident pins differ")
	}
	intentPath := filepath.Join(stage, "intent.json")
	if err := sameHash(intentPath, m.StageIntentSHA256); err != nil {
		return failed, err
	}
	var i intent
	if err := readJSON(intentPath, &i); err != nil {
		return failed, err
	}
	if i.SchemaVersion != "firedoor.v10.fd004-live-recovery-stage.v1" || i.Status != "hardened_source_staged_no_infrastructure" || i.SourceCommit != OriginalSourceCommit || i.Cluster != Cluster || i.Context != Context || i.Profile != "hardened" || i.ClusterCreated || i.ApplyAuthorized || i.RecoveryTested || i.KubeconfigPath != m.KubeconfigPath {
		return failed, errors.New("original stage intent is unexpected")
	}
	if len(i.SourceSHA256) != 13 || len(m.SourceSHA256) != len(sourceFiles) {
		return failed, errors.New("wrong original or fresh source inventory")
	}
	kube := filepath.Join(stage, "kubeconfig")
	if m.KubeconfigPath != kube {
		return failed, errors.New("non-retained kubeconfig")
	}
	kcHash, err := fingerprint(kube)
	if err != nil {
		return failed, err
	}
	evidence := filepath.Join(base, "live-recovery-bootstraps", StageLeaf, "bootstrap-reconciliation.v1.json")
	if err := sameHash(evidence, pins.Bootstrap); err != nil {
		return failed, err
	}
	var b bootstrap
	// Historical record has additional fields; fingerprint is immutable and is
	// authoritative here. Narrow parsing only extracts identity and failure.
	if err := readHistorical(evidence, &b); err != nil {
		return failed, err
	}
	if b.Cluster != Cluster || b.Context != Context || b.OriginalBootstrapRunSucceeded || !b.InfrastructureObservedReady || strings.ToLower(b.PrivateKubeconfigSHA256) != kcHash {
		return failed, errors.New("historical bootstrap evidence inconsistent")
	}
	historicalRun := filepath.Join(base, "live-recovery-plans", "20261111-040926850-3c138b60a075", "retained-plan-reconciliation.v1.json")
	if err := sameHash(historicalRun, pins.Plan); err != nil {
		return failed, err
	}
	var pr planReconciliation
	if err := readHistorical(historicalRun, &pr); err != nil {
		return failed, err
	}
	if pr.Cluster != Cluster || pr.ApplyAuthorized || pr.ApplyExecuted {
		return failed, errors.New("historical plan approval claim")
	}
	expected := map[string]bool{"manifest.json": true, "terraform/testbed/terraform.tfvars.json": true}
	expectedStage := map[string]bool{"intent.json": true, "kubeconfig": true, "kind/cluster.yaml": true, "terraform/testbed/terraform.tfvars.json": true}
	for _, f := range sourceFiles {
		hash, ok := i.SourceSHA256[f]
		if !ok || !hashName.MatchString(hash) || m.SourceSHA256[f] != hash {
			return failed, fmt.Errorf("invalid source hash: %s", f)
		}
		relative := strings.TrimPrefix(f, "infrastructure/")
		expected[relative] = true
		expectedStage[relative] = true
		for _, root := range []struct {
			root string
			name string
		}{{repo, f}, {stage, relative}, {fresh, relative}} {
			if err := sameHash(filepath.Join(root.root, filepath.FromSlash(root.name)), hash); err != nil {
				return failed, err
			}
		}
		if strings.HasSuffix(f, ".tf") && strings.HasPrefix(f, "infrastructure/terraform/testbed/") {
			raw, err := os.ReadFile(filepath.Join(fresh, filepath.FromSlash(relative)))
			if err != nil {
				return failed, err
			}
			if backendDeclaration.Match(raw) {
				return failed, errors.New("explicit backend/cloud declaration is not allowed")
			}
		}
	}
	if len(i.SourceSHA256) != 13 || !hashName.MatchString(i.SourceSHA256[kindSource]) {
		return failed, errors.New("missing original KIND pin")
	}
	if err := sameHash(filepath.Join(stage, "kind", "cluster.yaml"), i.SourceSHA256[kindSource]); err != nil {
		return failed, err
	}
	if err := sameHash(filepath.Join(repo, filepath.FromSlash(kindSource)), i.SourceSHA256[kindSource]); err != nil {
		return failed, err
	}
	if err := checkInventory(stage, expectedStage); err != nil {
		return failed, fmt.Errorf("original stage: %w", err)
	}
	if err := checkInventory(fresh, expected); err != nil {
		return failed, fmt.Errorf("fresh stage: %w", err)
	}
	tfvars := filepath.Join(stage, "terraform", "testbed", "terraform.tfvars.json")
	if err := sameHash(tfvars, m.TFVarsSHA256); err != nil {
		return failed, err
	}
	if err := sameHash(filepath.Join(fresh, "terraform", "testbed", "terraform.tfvars.json"), m.TFVarsSHA256); err != nil {
		return failed, err
	}
	var v vars
	if err := readJSON(tfvars, &v); err != nil {
		return failed, err
	}
	if v.SecurityProfile != "hardened" || v.KubeContext != Context || v.KubeconfigPath != kube {
		return failed, errors.New("staged hardened variables rejected")
	}
	mh, err := fingerprint(manifestPath)
	if err != nil {
		return failed, err
	}
	return Report{SchemaVersion: Schema, Status: Status, Cluster: Cluster, FilesVerified: 12, WorkspaceSHA256: mh, DeploymentBlocked: true}, nil
}

// Historical evidence can have additional schema fields; only the pinned hash
// allows this bounded identity read to ignore those extra fields.
func readHistorical(path string, target any) error {
	if err := plain(path, false); err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Size() < 2 || st.Size() > 256*1024 {
		return errors.New("historical JSON outside size limit")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err = strictValue(d); err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("historical JSON trailing input")
	}
	return json.Unmarshal(raw, target)
}
func SourceFiles() []string {
	out := append([]string(nil), sourceFiles...)
	sort.Strings(out)
	return out
}
