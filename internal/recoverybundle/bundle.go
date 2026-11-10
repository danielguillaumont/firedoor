// Package recoverybundle validates private FD-004 recovery-preflight artifacts.
// It performs only local reads and never authorizes execution or proves recovery.
package recoverybundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	Schema              = "firedoor.v10.fd004-recovery-bundle-review.v1"
	sourceSchema        = "firedoor.v10.fd004-recovery-preflight.v1"
	maxStateBytes int64 = 10 << 20
	maxJSONBytes  int64 = 1 << 20
)

var fileNames = []string{"terraform.tfstate.backup", "firedoor-app-ingress.json", "firedoor-data-ingress.json", "recovery-preflight-report.json"}
var hex64 = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type Review struct {
	SchemaVersion          string    `json:"schema_version"`
	Scenario               string    `json:"scenario"`
	Status                 string    `json:"status"`
	Profile                string    `json:"profile"`
	Cluster                string    `json:"cluster"`
	Context                string    `json:"context"`
	CapturedAtUTC          time.Time `json:"captured_at_utc"`
	StateSHA256            string    `json:"state_sha256"`
	TerraformOutputsSHA256 string    `json:"terraform_outputs_sha256"`
	CheckedFiles           int       `json:"checked_files"`
	ApplyAuthorized        bool      `json:"apply_authorized"`
	RecoveryTested         bool      `json:"recovery_tested"`
	PolicyCausalityProven  bool      `json:"policy_causality_proven"`
	Limitations            string    `json:"limitations"`
}

type preflightReport struct {
	SchemaVersion          string            `json:"schema_version"`
	Scenario               string            `json:"scenario"`
	Cluster                string            `json:"cluster"`
	Context                string            `json:"context"`
	Profile                string            `json:"profile"`
	CapturedAtUTC          time.Time         `json:"captured_at_utc"`
	StateAddressCount      int               `json:"state_address_count"`
	TerraformOutputsSHA256 string            `json:"terraform_outputs_sha256"`
	RecoveryManifests      []json.RawMessage `json:"recovery_manifests"`
	Status                 string            `json:"status"`
	ApplyAuthorized        bool              `json:"apply_authorized"`
	RecoveryTested         bool              `json:"recovery_tested"`
	TerraformAuthenticated bool              `json:"terraform_authenticated"`
	PolicyCausalityProven  bool              `json:"policy_causality_proven"`
	Limitations            string            `json:"limitations"`
}

func regularFile(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > max {
		return nil, fmt.Errorf("not a bounded regular file: %s", filepath.Base(path))
	}
	return os.ReadFile(path)
}
func hexSum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func jsonValue(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, errors.New("JSON object required")
	}
	return v, nil
}

// expectedManifest is deliberately independent from the generator. This detects
// accidental weakening of the recovery manifest contract.
func expectedManifest(zone, source string) any {
	return map[string]any{
		"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy",
		"metadata": map[string]any{
			"name": "firedoor-" + zone + "-ingress", "namespace": "firedoor-" + zone,
			"labels": map[string]any{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"},
		},
		"spec": map[string]any{
			"podSelector": map[string]any{"matchLabels": map[string]any{"app.kubernetes.io/name": "firedoor-" + zone}},
			"policyTypes": []any{"Ingress"},
			"ingress": []any{map[string]any{
				"from":  []any{map[string]any{"namespaceSelector": map[string]any{"matchLabels": map[string]any{"firedoor.dev/zone": source}}}},
				"ports": []any{map[string]any{"port": float64(8080), "protocol": "TCP"}},
			}},
		},
	}
}

// Verify reads an exact five-file bundle and checks independent exact-policy
// semantics, cryptographic checksums and equality to the *current* local state.
// It does not authenticate the bundle's author, execute plans, or prove recovery.
func Verify(bundleDir, statePath string, now time.Time) (Review, error) {
	var none Review
	if bundleDir == "" || statePath == "" {
		return none, errors.New("bundle and state paths required")
	}
	dir, err := os.Lstat(bundleDir)
	if err != nil {
		return none, err
	}
	if !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
		return none, errors.New("bundle must be a real directory")
	}
	entries, err := os.ReadDir(bundleDir)
	if err != nil {
		return none, err
	}
	if len(entries) != 5 {
		return none, fmt.Errorf("unexpected bundle entry count %d", len(entries))
	}
	expectedSet := map[string]bool{"SHA256.txt": true}
	for _, n := range fileNames {
		expectedSet[n] = true
	}
	for _, e := range entries {
		if !expectedSet[e.Name()] || !e.Type().IsRegular() {
			return none, fmt.Errorf("unexpected bundle entry %q", e.Name())
		}
	}
	lines, err := regularFile(filepath.Join(bundleDir, "SHA256.txt"), 2048)
	if err != nil {
		return none, err
	}
	lineList := strings.Split(strings.TrimSpace(string(lines)), "\n")
	if len(lineList) != len(fileNames) {
		return none, errors.New("expected exactly four SHA256 entries")
	}
	sums := make(map[string]string, 4)
	for _, l := range lineList {
		cols := strings.Fields(strings.TrimSpace(l))
		if len(cols) != 3 || cols[1] != "SHA256" || !hex64.MatchString(cols[2]) || !expectedSet[cols[0]] || cols[0] == "SHA256.txt" || sums[cols[0]] != "" {
			return none, errors.New("invalid or duplicate checksum record")
		}
		sums[cols[0]] = strings.ToLower(cols[2])
	}
	data := make(map[string][]byte, 4)
	for _, n := range fileNames {
		max := maxJSONBytes
		if n == "terraform.tfstate.backup" {
			max = maxStateBytes
		}
		b, e := regularFile(filepath.Join(bundleDir, n), max)
		if e != nil {
			return none, e
		}
		if hexSum(b) != sums[n] {
			return none, fmt.Errorf("checksum mismatch: %s", n)
		}
		data[n] = b
	}
	current, e := regularFile(statePath, maxStateBytes)
	if e != nil {
		return none, e
	}
	if hexSum(current) != sums["terraform.tfstate.backup"] {
		return none, errors.New("current state hash differs from backed-up state; abort")
	}
	var tfState map[string]any
	if e = json.Unmarshal(data["terraform.tfstate.backup"], &tfState); e != nil || tfState == nil {
		return none, errors.New("state backup is not Terraform JSON")
	}
	// Terraform local state serial must exist. Do not pretend this authenticates its origin.
	if _, ok := tfState["serial"].(float64); !ok {
		return none, errors.New("state JSON has no Terraform serial")
	}
	for i, pair := range [][2]string{{"app", "edge"}, {"data", "app"}} {
		actual, e := jsonValue(data[fileNames[i+1]])
		if e != nil {
			return none, fmt.Errorf("invalid recovery manifest: %w", e)
		}
		if !reflect.DeepEqual(actual, expectedManifest(pair[0], pair[1])) {
			return none, fmt.Errorf("unexpected hardened %s policy manifest", pair[0])
		}
	}
	// Preserve the exact source-report contract. Missing booleans must not
	// silently default to false, and unknown security claims are rejected.
	var reportFields map[string]json.RawMessage
	if e = json.Unmarshal(data["recovery-preflight-report.json"], &reportFields); e != nil {
		return none, fmt.Errorf("invalid preflight report fields: %w", e)
	}
	requiredFields := []string{"schema_version", "scenario", "cluster", "context", "profile", "captured_at_utc", "state_address_count", "terraform_outputs_sha256", "recovery_manifests", "status", "apply_authorized", "recovery_tested", "terraform_authenticated", "policy_causality_proven", "limitations"}
	if len(reportFields) != len(requiredFields) {
		return none, errors.New("unexpected preflight report fields")
	}
	for _, field := range requiredFields {
		if _, ok := reportFields[field]; !ok {
			return none, fmt.Errorf("missing preflight report field %s", field)
		}
	}
	var report preflightReport
	if e = json.Unmarshal(data["recovery-preflight-report.json"], &report); e != nil {
		return none, fmt.Errorf("invalid report: %w", e)
	}
	if report.SchemaVersion != sourceSchema || report.Scenario != "FD-004" || report.Cluster != "firedoor-local" || report.Context != "kind-firedoor-local" || report.Profile != "hardened" || report.StateAddressCount != 14 || report.Status != "hardened_preflight_consistent_recovery_not_exercised" || report.ApplyAuthorized || report.RecoveryTested || report.TerraformAuthenticated || report.PolicyCausalityProven || !hex64.MatchString(report.TerraformOutputsSHA256) || len(report.RecoveryManifests) != 2 || report.Limitations == "" {
		return none, errors.New("preflight report makes an invalid security claim or has invalid ownership")
	}
	if report.CapturedAtUTC.IsZero() || now.Sub(report.CapturedAtUTC) > 24*time.Hour || report.CapturedAtUTC.After(now.Add(5*time.Minute)) {
		return none, errors.New("recovery preflight is stale or future-dated; generate a fresh preflight")
	}
	for i, pair := range [][2]string{{"app", "edge"}, {"data", "app"}} {
		actual, e := jsonValue(report.RecoveryManifests[i])
		if e != nil || !reflect.DeepEqual(actual, expectedManifest(pair[0], pair[1])) {
			return none, fmt.Errorf("report embedded recovery manifest %s differs", pair[0])
		}
	}
	return Review{
		SchemaVersion: Schema, Scenario: "FD-004", Status: "bundle_integrity_checked_recovery_not_exercised", Profile: "hardened", Cluster: "firedoor-local", Context: "kind-firedoor-local",
		CapturedAtUTC: report.CapturedAtUTC, StateSHA256: sums["terraform.tfstate.backup"], TerraformOutputsSHA256: report.TerraformOutputsSHA256, CheckedFiles: 4,
		ApplyAuthorized: false, RecoveryTested: false, PolicyCausalityProven: false,
		Limitations: "Integrity and state equality are not authentication, recovery testing or authorization. No Terraform plan/apply, Kubernetes apply, cluster modification or live probe was executed. Live policy consistency requires an independent preflight.",
	}, nil
}
