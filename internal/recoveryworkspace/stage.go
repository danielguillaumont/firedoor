// Package recoveryworkspace independently verifies the next FD-004 disposable
// staging contract. It only reads files; it cannot provision infrastructure.
package recoveryworkspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/rehearsalworkspace"
)

const (
	Schema            = "firedoor.v10.fd004-live-recovery-stage.v1"
	Status            = "hardened_source_staged_no_infrastructure"
	BaseCommit        = "a22c36e"
	HistoricalCluster = "firedoor-rh-0ffb4d40a3c2"
)

var clusterRE = regexp.MustCompile(`^firedoor-rh-[0-9a-f]{12}$`)
var hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Intent struct {
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

type variables struct {
	Profile        string `json:"security_profile"`
	KubeconfigPath string `json:"kubeconfig_path"`
	Context        string `json:"kube_context"`
}

type Report struct {
	SchemaVersion         string `json:"schema_version"`
	Status                string `json:"status"`
	SourceCommit          string `json:"source_commit"`
	Cluster               string `json:"cluster"`
	Context               string `json:"context"`
	Profile               string `json:"profile"`
	FilesCompared         int    `json:"files_compared"`
	KubeconfigPresent     bool   `json:"kubeconfig_present"`
	TerraformStatePresent bool   `json:"terraform_state_present"`
	ApplyAuthorized       bool   `json:"apply_authorized"`
	RecoveryTested        bool   `json:"recovery_tested"`
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func safeFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
		return nil, fmt.Errorf("nonregular or oversized file: %s", path)
	}
	return os.ReadFile(path)
}

// validateKeys rejects duplicate keys at every JSON depth (the standard JSON
// unmarshaller would otherwise silently overwrite them).
func validateKeys(d *json.Decoder) error {
	first, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := first.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key not a string")
			}
			if keys[key] {
				return fmt.Errorf("duplicate JSON key: %s", key)
			}
			keys[key] = true
			if err := validateKeys(d); err != nil {
				return err
			}
		}
		_, err := d.Token()
		return err
	case '[':
		for d.More() {
			if err := validateKeys(d); err != nil {
				return err
			}
		}
		_, err := d.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func strictDecode(data []byte, out any) error {
	if len(data) > 2*1024*1024 {
		return errors.New("JSON too large")
	}
	tokenDecoder := json.NewDecoder(bytes.NewReader(data))
	if err := validateKeys(tokenDecoder); err != nil {
		return err
	}
	if _, err := tokenDecoder.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

// Verify checks a pristine stage located strictly underneath an independently
// supplied private root. It compares source content against the current repo
// and the intent hashes, and refuses all execution claims.
func Verify(repo, privateRoot, stage string) (Report, error) {
	var report Report
	var err error
	repo, err = filepath.Abs(repo)
	if err != nil {
		return report, err
	}
	privateRoot, err = filepath.Abs(privateRoot)
	if err != nil {
		return report, err
	}
	stage, err = filepath.Abs(stage)
	if err != nil {
		return report, err
	}
	if !within(stage, privateRoot) || within(stage, repo) || within(repo, stage) || repo == stage {
		return report, errors.New("staging root outside private boundary or inside repository")
	}
	for _, path := range []string{privateRoot, stage} {
		info, err := os.Lstat(path)
		if err != nil {
			return report, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return report, errors.New("invalid or linked directory")
		}
	}
	raw, err := safeFile(filepath.Join(stage, "intent.json"))
	if err != nil {
		return report, err
	}
	var intent Intent
	if err := strictDecode(raw, &intent); err != nil {
		return report, fmt.Errorf("invalid intent: %w", err)
	}
	if intent.SchemaVersion != Schema || intent.Status != Status || intent.SourceCommit != BaseCommit ||
		!clusterRE.MatchString(intent.Cluster) || intent.Cluster == HistoricalCluster ||
		intent.Context != "kind-"+intent.Cluster || intent.Profile != "hardened" ||
		intent.ClusterCreated || intent.ApplyAuthorized || intent.RecoveryTested {
		return report, errors.New("invalid identity, security profile, or unsupported execution claim")
	}
	kubeconfig := filepath.Join(stage, "kubeconfig")
	if filepath.Clean(intent.KubeconfigPath) != kubeconfig {
		return report, errors.New("invalid private kubeconfig target")
	}
	if len(intent.SourceSHA256) != len(rehearsalworkspace.SourcePaths) {
		return report, errors.New("source allowlist count mismatch")
	}
	expected := map[string]bool{"intent.json": true, "terraform/testbed/terraform.tfvars.json": true}
	for _, src := range rehearsalworkspace.SourcePaths {
		relative := rehearsalworkspace.StageRelative(src)
		if relative == "" {
			return report, fmt.Errorf("unsupported source: %s", src)
		}
		expected[relative] = true
		sourceData, err := safeFile(filepath.Join(repo, filepath.FromSlash(src)))
		if err != nil {
			return report, fmt.Errorf("source %s: %w", src, err)
		}
		stagedData, err := safeFile(filepath.Join(stage, filepath.FromSlash(relative)))
		if err != nil {
			return report, fmt.Errorf("staged %s: %w", relative, err)
		}
		digest := sha256.Sum256(sourceData)
		wanted := hex.EncodeToString(digest[:])
		if !bytes.Equal(sourceData, stagedData) || !hashRE.MatchString(intent.SourceSHA256[src]) ||
			intent.SourceSHA256[src] != wanted {
			return report, fmt.Errorf("source hash mismatch: %s", src)
		}
	}
	varsRaw, err := safeFile(filepath.Join(stage, "terraform", "testbed", "terraform.tfvars.json"))
	if err != nil {
		return report, err
	}
	var vars variables
	if err := strictDecode(varsRaw, &vars); err != nil {
		return report, fmt.Errorf("invalid tfvars: %w", err)
	}
	if vars.Profile != "hardened" || vars.Context != intent.Context || filepath.Clean(vars.KubeconfigPath) != kubeconfig {
		return report, errors.New("terraform target or initial profile mismatch")
	}
	if strings.Contains(string(varsRaw), "firedoor-local") {
		return report, errors.New("protected cluster in tfvars")
	}
	expectedDirs := map[string]bool{}
	for rel := range expected {
		for dir := filepath.ToSlash(filepath.Dir(rel)); dir != "." && dir != ""; dir = filepath.ToSlash(filepath.Dir(dir)) {
			expectedDirs[dir] = true
		}
	}
	found := map[string]bool{}
	err = filepath.Walk(stage, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked path forbidden: %s", rel)
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			if !expectedDirs[name] {
				return fmt.Errorf("unexpected directory: %s", name)
			}
			return nil
		}
		if !info.Mode().IsRegular() || !expected[name] || found[name] {
			return fmt.Errorf("unexpected or nonregular file: %s", name)
		}
		found[name] = true
		return nil
	})
	if err != nil {
		return report, err
	}
	if len(found) != len(expected) {
		return report, errors.New("missing staged file")
	}
	return Report{SchemaVersion: Schema, Status: Status, SourceCommit: BaseCommit,
		Cluster: intent.Cluster, Context: intent.Context, Profile: "hardened",
		FilesCompared: len(rehearsalworkspace.SourcePaths)}, nil
}
