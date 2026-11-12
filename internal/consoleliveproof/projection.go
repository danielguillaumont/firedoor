// Package consoleliveproof creates a minimized, historical-only console view
// from a privately preserved FD-004 F5 traffic receipt. It does no IO and
// never performs Terraform, Kubernetes, or network operations.
package consoleliveproof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const ReceiptSchema = "firedoor.v10.fd004-traffic-proof-receipt.v1"
const ProjectionSchema = "firedoor.v10.console-live-proof-projection.v1"
const expectedCluster = "firedoor-rh-3c138b60a075"
const expectedApplySHA = "1E390B0301308C9C646AFA10896E5282E185C1ADA8914559C7A25CAF4A37BD12"
const maxReceiptBytes = 32 * 1024

// The receipt contains local evidence identifiers, but the projection does not.
// Its field inventory is exact to prevent accidental future data exposure.
type receipt struct {
	SchemaVersion                     string `json:"schema_version"`
	Cluster                           string `json:"cluster"`
	Context                           string `json:"context"`
	ApplyResultSHA256                 string `json:"apply_result_sha256"`
	TerraformStateSHA256              string `json:"terraform_state_sha256"`
	ObservedUTC                       string `json:"observed_utc"`
	F4AuditPassed                     bool   `json:"f4_audit_passed"`
	NetworkTrafficEnforcementVerified bool   `json:"network_traffic_enforcement_verified"`
	AllowedVerified                   int    `json:"allowed_verified"`
	DeniedVerified                    int    `json:"denied_verified"`
	Tests                             []test `json:"tests"`
	KubernetesObjectsModified         bool   `json:"kubernetes_objects_modified"`
	TerraformExecuted                 bool   `json:"terraform_executed"`
	RecoveryProven                    bool   `json:"recovery_proven"`
}

type test struct {
	ID     string `json:"id"`
	Result string `json:"result"`
}

type PathObservation struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Observation string `json:"observation"`
}

type Projection struct {
	SchemaVersion                 string            `json:"schema_version"`
	Source                        string            `json:"source"`
	EvidenceStatus                string            `json:"evidence_status"`
	LabLabel                      string            `json:"lab_label"`
	ObservedUTC                   string            `json:"observed_utc"`
	AllowedCount                  int               `json:"allowed_count"`
	DeniedCount                   int               `json:"denied_count"`
	NetworkVerifiedAtCapture      bool              `json:"network_verified_at_capture"`
	CurrentClusterChecked         bool              `json:"current_cluster_checked"`
	CurrentEnforcementVerified    bool              `json:"current_enforcement_verified"`
	TerraformExecutedByProjection bool              `json:"terraform_executed_by_projection"`
	KubernetesChangedByProjection bool              `json:"kubernetes_changed_by_projection"`
	RecoveryProven                bool              `json:"recovery_proven"`
	Observations                  []PathObservation `json:"observations"`
	Limitations                   []string          `json:"limitations"`
}

var receiptKeys = []string{
	"schema_version", "cluster", "context", "apply_result_sha256", "terraform_state_sha256",
	"observed_utc", "f4_audit_passed", "network_traffic_enforcement_verified",
	"allowed_verified", "denied_verified", "tests", "kubernetes_objects_modified",
	"terraform_executed", "recovery_proven",
}
var expectedPaths = []struct{ id, from, to, result string }{
	{"edge-to-app", "edge", "app", "allowed"},
	{"app-to-data", "app", "data", "allowed"},
	{"data-to-edge-control", "data", "edge", "allowed"},
	{"edge-to-data", "edge", "data", "denied"},
	{"data-to-app", "data", "app", "denied"},
}

func exactObject(data json.RawMessage, keys ...string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return errors.New("expected JSON object")
	}
	if len(obj) != len(keys) {
		return errors.New("unexpected field inventory")
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			return errors.New("missing required field")
		}
	}
	return nil
}

// DecodeAndProject strictly checks a *locally supplied* receipt and returns
// only allowlisted display data. Callers must independently hash-pin bytes
// before invoking this function. No private paths, state hashes, or IPs leave.
func DecodeAndProject(input []byte) (Projection, error) {
	reject := func(reason string) (Projection, error) {
		return Projection{}, fmt.Errorf("historical proof projection rejected: %s", reason)
	}
	if len(input) == 0 || len(input) > maxReceiptBytes {
		return reject("receipt size")
	}
	input = bytes.TrimPrefix(input, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return reject("invalid JSON")
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return reject("trailing JSON")
	}
	if err := exactObject(raw, receiptKeys...); err != nil {
		return reject("top-level fields")
	}
	var r receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return reject("receipt field types")
	}
	if r.SchemaVersion != ReceiptSchema || r.Cluster != expectedCluster || r.Context != "kind-"+expectedCluster ||
		!strings.EqualFold(r.ApplyResultSHA256, expectedApplySHA) ||
		!isSHA256(r.TerraformStateSHA256) {
		return reject("unexpected source or evidence identity")
	}
	if !r.F4AuditPassed || !r.NetworkTrafficEnforcementVerified || r.AllowedVerified != 3 || r.DeniedVerified != 2 ||
		r.KubernetesObjectsModified || r.TerraformExecuted || r.RecoveryProven {
		return reject("invalid capture assurance")
	}
	if !strings.HasSuffix(r.ObservedUTC, "Z") {
		return reject("UTC observation time required")
	}
	stamp, err := time.Parse(time.RFC3339Nano, r.ObservedUTC)
	if err != nil || stamp.Year() < 2026 {
		return reject("observation timestamp")
	}
	if len(r.Tests) != len(expectedPaths) {
		return reject("five paths required")
	}
	var rawTests []json.RawMessage
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return reject("tests shape")
	}
	if err := json.Unmarshal(fields["tests"], &rawTests); err != nil || len(rawTests) != len(expectedPaths) {
		return reject("tests array")
	}
	byID := make(map[string]string, len(expectedPaths))
	for i, item := range r.Tests {
		if err := exactObject(rawTests[i], "id", "result"); err != nil {
			return reject("test fields")
		}
		if _, duplicate := byID[item.ID]; duplicate {
			return reject("duplicate test")
		}
		byID[item.ID] = item.Result
	}
	observations := make([]PathObservation, 0, len(expectedPaths))
	for _, expected := range expectedPaths {
		if byID[expected.id] != expected.result {
			return reject("missing or contradictory path")
		}
		observations = append(observations, PathObservation{
			ID: expected.id, From: expected.from, To: expected.to, Observation: expected.result,
		})
	}
	return Projection{
		SchemaVersion:  ProjectionSchema,
		Source:         "explicit_private_f5_receipt_projection",
		EvidenceStatus: "historical_verified_at_capture_not_live",
		LabLabel:       "Disposable KIND network-policy lab",
		ObservedUTC:    stamp.UTC().Format(time.RFC3339Nano),
		AllowedCount:   3, DeniedCount: 2,
		NetworkVerifiedAtCapture: true,
		CurrentClusterChecked:    false, CurrentEnforcementVerified: false,
		TerraformExecutedByProjection: false, KubernetesChangedByProjection: false,
		RecoveryProven: false, Observations: observations,
		Limitations: []string{
			"Historical F5 observations were verified at capture time; this projection does not check current cluster state.",
			"Five HTTP paths demonstrate bounded behavior in a disposable lab; they are not universal proof of isolation.",
			"A local SHA-256 match is an integrity check, not an independent or cryptographic attestation of origin.",
			"Recovery and production security posture are not established by this display projection.",
		},
	}, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
