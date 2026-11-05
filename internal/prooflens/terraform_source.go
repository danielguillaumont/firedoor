package prooflens

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TerraformSourceSchema identifies an offline inventory of Terraform source
// files. It is NOT a Terraform plan, applied-state record, or certification.
const TerraformSourceSchema = "firedoor.v05.terraform-source.v1"

const (
	TerraformSourceOnly   = "SOURCE_ONLY_READ_ONLY"
	maxTerraformFileBytes = 256 * 1024
	maxTerraformFiles     = 40
	maxTerraformTotal     = 2 * 1024 * 1024
)

// Only the three Firedoor-owned source directories are inventoried. Files
// ending in .tfvars, .tfstate, JSON plans, or other extensions are never read.
var terraformSourceDirs = []string{
	"infrastructure/terraform/modules/network-boundary",
	"infrastructure/terraform/modules/workload",
	"infrastructure/terraform/testbed",
}

var terraformRequiredFiles = []string{
	"infrastructure/terraform/modules/network-boundary/main.tf",
	"infrastructure/terraform/modules/workload/main.tf",
	"infrastructure/terraform/testbed/main.tf",
}

// TerraformSourceFile contains a file-level digest, never raw source content.
type TerraformSourceFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// TerraformSourceManifest describes the source tree at collection time. The
// tree hash is independent of collection time, to support reproducibility.
type TerraformSourceManifest struct {
	SchemaVersion string                `json:"schema_version"`
	Mode          string                `json:"mode"`
	CapturedUTC   string                `json:"captured_utc"`
	Files         []TerraformSourceFile `json:"files"`
	FileCount     int                   `json:"file_count"`
	TreeSHA256    string                `json:"tree_sha256"`
	Limitations   []string              `json:"limitations"`
}

func terraformLimitations() []string {
	return []string{
		"Source hashes do not prove which Terraform profile was selected, planned, or applied.",
		"No Terraform state, plan JSON, variable values, or Kubernetes observations were inspected.",
		"SHA-256 provides consistency checks, not publisher authentication or trustworthy attestation.",
	}
}

func terraformPathAllowed(path string) bool {
	if path == "" || filepath.ToSlash(path) != path || strings.Contains(path, "\\") ||
		strings.HasPrefix(path, "/") || filepath.Clean(path) != filepath.FromSlash(path) {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(path), ".tf") {
		return false
	}
	for _, dir := range terraformSourceDirs {
		if strings.HasPrefix(path, dir+"/") && !strings.Contains(strings.TrimPrefix(path, dir+"/"), "/") {
			return true
		}
	}
	return false
}

func terraformTreeDigest(files []TerraformSourceFile) (string, error) {
	type input struct {
		Files []TerraformSourceFile `json:"files"`
	}
	b, err := json.Marshal(input{Files: files})
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = io.WriteString(h, TerraformSourceSchema+"\n")
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CollectTerraformSources reads only .tf files immediately under the three
// fixed Firedoor directories. It does not execute Terraform or read secrets
// from tfvars, state, or plan files. Symlinked .tf files/directories fail closed.
func CollectTerraformSources(repoRoot string) (TerraformSourceManifest, error) {
	var empty TerraformSourceManifest
	if repoRoot == "" {
		return empty, errors.New("repository root required")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return empty, fmt.Errorf("resolve repository root: %w", err)
	}
	var files []TerraformSourceFile
	var total int64
	for _, dir := range terraformSourceDirs {
		fullDir := root
		// Check every path component for symlinks to avoid escaping the root.
		for _, segment := range strings.Split(dir, "/") {
			fullDir = filepath.Join(fullDir, segment)
			info, e := os.Lstat(fullDir)
			if e != nil {
				return empty, fmt.Errorf("required Terraform directory %s: %w", dir, e)
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return empty, fmt.Errorf("Terraform directory is not a plain directory: %s", dir)
			}
		}
		entries, e := os.ReadDir(fullDir)
		if e != nil {
			return empty, fmt.Errorf("enumerate %s: %w", dir, e)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".tf") {
				continue
			}
			rel := dir + "/" + entry.Name()
			if !terraformPathAllowed(rel) || !entry.Type().IsRegular() {
				return empty, fmt.Errorf("unsafe Terraform source file: %s", rel)
			}
			if len(files) >= maxTerraformFiles {
				return empty, errors.New("too many Terraform files")
			}
			path := filepath.Join(fullDir, entry.Name())
			info, e := os.Lstat(path)
			if e != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxTerraformFileBytes {
				return empty, fmt.Errorf("Terraform file is non-regular or oversized: %s", rel)
			}
			total += info.Size()
			if total > maxTerraformTotal {
				return empty, errors.New("Terraform source tree exceeds size limit")
			}
			f, e := os.Open(path)
			if e != nil {
				return empty, fmt.Errorf("open %s: %w", rel, e)
			}
			// Hash content without ever retaining it in a manifest or printing it.
			h := sha256.New()
			n, copyErr := io.Copy(h, io.LimitReader(f, maxTerraformFileBytes+1))
			closeErr := f.Close()
			if copyErr != nil || closeErr != nil || n != info.Size() || n > maxTerraformFileBytes {
				return empty, fmt.Errorf("Terraform source changed or failed while reading: %s", rel)
			}
			files = append(files, TerraformSourceFile{Path: rel, Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil))})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	hash, e := terraformTreeDigest(files)
	if e != nil {
		return empty, e
	}
	m := TerraformSourceManifest{SchemaVersion: TerraformSourceSchema, Mode: TerraformSourceOnly,
		CapturedUTC: time.Now().UTC().Format(time.RFC3339Nano), Files: files,
		FileCount: len(files), TreeSHA256: hash, Limitations: terraformLimitations()}
	if e := VerifyTerraformSourceManifest(m); e != nil {
		return empty, e
	}
	return m, nil
}

// VerifyTerraformSourceManifest checks the structure and internal checksums of
// a saved manifest. Without a trusted external digest it does not authenticate
// the person/process which produced it.
func VerifyTerraformSourceManifest(m TerraformSourceManifest) error {
	if m.SchemaVersion != TerraformSourceSchema || m.Mode != TerraformSourceOnly {
		return errors.New("unexpected Terraform manifest schema or mode")
	}
	if !validUTC(m.CapturedUTC) || m.FileCount != len(m.Files) || len(m.Files) > maxTerraformFiles {
		return errors.New("invalid timestamp or file count")
	}
	if len(m.Limitations) != 3 || !checkLimitations(m.Limitations) {
		return errors.New("missing provenance limitations")
	}
	var total int64
	seen := make(map[string]bool, len(m.Files))
	previous := ""
	for _, file := range m.Files {
		if !terraformPathAllowed(file.Path) || seen[file.Path] || (previous != "" && file.Path <= previous) ||
			!validHex(file.SHA256, 64) || file.Bytes < 0 || file.Bytes > maxTerraformFileBytes {
			return errors.New("invalid, unsorted, duplicate, or oversized Terraform file record")
		}
		total += file.Bytes
		seen[file.Path] = true
		previous = file.Path
	}
	if total > maxTerraformTotal {
		return errors.New("Terraform source tree is oversized")
	}
	for _, required := range terraformRequiredFiles {
		if !seen[required] {
			return fmt.Errorf("missing required Terraform source: %s", required)
		}
	}
	if !validHex(m.TreeSHA256, 64) {
		return errors.New("invalid Terraform source tree digest")
	}
	hash, err := terraformTreeDigest(m.Files)
	if err != nil {
		return err
	}
	if hash != m.TreeSHA256 {
		return errors.New("Terraform source tree digest mismatch")
	}
	return nil
}

// RecheckTerraformSources compares the saved digest against a fresh read of
// the same fixed source directories, without calling Terraform.
func RecheckTerraformSources(repoRoot string, saved TerraformSourceManifest) error {
	if err := VerifyTerraformSourceManifest(saved); err != nil {
		return err
	}
	current, err := CollectTerraformSources(repoRoot)
	if err != nil {
		return err
	}
	if current.TreeSHA256 != saved.TreeSHA256 {
		return errors.New("Terraform source tree drift detected")
	}
	return nil
}

// TerraformSourceSummary can be linked from a declared Proof Lens evidence
// record. It describes source files only, not an authenticated applied plan.
func TerraformSourceSummary(m TerraformSourceManifest) (SourceSummary, error) {
	if err := VerifyTerraformSourceManifest(m); err != nil {
		return SourceSummary{}, err
	}
	return SourceSummary{Kind: "terraform_source_tree", Reference: "infrastructure/terraform", SHA256: m.TreeSHA256}, nil
}
