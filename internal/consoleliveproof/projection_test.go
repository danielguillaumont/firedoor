package consoleliveproof

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func goodReceipt(t *testing.T) []byte {
	t.Helper()
	r := receipt{
		SchemaVersion: ReceiptSchema, Cluster: expectedCluster, Context: "kind-" + expectedCluster,
		ApplyResultSHA256:    expectedApplySHA,
		TerraformStateSHA256: strings.Repeat("a", 64),
		ObservedUTC:          "2026-11-12T03:25:51.12Z",
		F4AuditPassed:        true, NetworkTrafficEnforcementVerified: true,
		AllowedVerified: 3, DeniedVerified: 2,
	}
	for _, e := range expectedPaths {
		r.Tests = append(r.Tests, test{ID: e.id, Result: e.result})
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProjectionPreservesOnlyBoundedHistoricalObservations(t *testing.T) {
	projection, err := DecodeAndProject(goodReceipt(t))
	if err != nil {
		t.Fatal(err)
	}
	if projection.AllowedCount != 3 || projection.DeniedCount != 2 ||
		!projection.NetworkVerifiedAtCapture || projection.CurrentClusterChecked ||
		projection.CurrentEnforcementVerified || projection.RecoveryProven ||
		projection.ObservedUTC != "2026-11-12T03:25:51.12Z" ||
		len(projection.Observations) != 5 {
		t.Fatalf("misleading projection: %+v", projection)
	}
	b, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		expectedCluster, expectedApplySHA, strings.Repeat("a", 64), "kind-", "cluster_ip",
		"terraform_state_sha256", "apply_result_sha256", "kubeconfig", "private_path",
	} {
		if bytes.Contains(bytes.ToLower(b), bytes.ToLower([]byte(forbidden))) {
			t.Fatalf("private/raw field leaked: %s", forbidden)
		}
	}
	withBOM := append([]byte{0xef, 0xbb, 0xbf}, goodReceipt(t)...)
	if _, err := DecodeAndProject(withBOM); err != nil {
		t.Fatal(err)
	}
}

func TestRejectBadPrivateReceipts(t *testing.T) {
	base := goodReceipt(t)
	changed := map[string]func(map[string]any){
		"wrong cluster":       func(r map[string]any) { r["cluster"] = "firedoor-local" },
		"wrong context":       func(r map[string]any) { r["context"] = "kind-firedoor-local" },
		"wrong apply hash":    func(r map[string]any) { r["apply_result_sha256"] = strings.Repeat("b", 64) },
		"bad state sha":       func(r map[string]any) { r["terraform_state_sha256"] = "bad" },
		"not audited":         func(r map[string]any) { r["f4_audit_passed"] = false },
		"not verified":        func(r map[string]any) { r["network_traffic_enforcement_verified"] = false },
		"wrong allow count":   func(r map[string]any) { r["allowed_verified"] = 4 },
		"wrong deny count":    func(r map[string]any) { r["denied_verified"] = 1 },
		"k8s changed":         func(r map[string]any) { r["kubernetes_objects_modified"] = true },
		"terraform ran":       func(r map[string]any) { r["terraform_executed"] = true },
		"recovery overstated": func(r map[string]any) { r["recovery_proven"] = true },
		"not utc":             func(r map[string]any) { r["observed_utc"] = "2026-11-12T03:25:51-05:00" },
		"missing field":       func(r map[string]any) { delete(r, "f4_audit_passed") },
		"extra raw field":     func(r map[string]any) { r["kubeconfig"] = "secret" },
		"missing path":        func(r map[string]any) { r["tests"] = r["tests"].([]any)[:4] },
		"duplicate path":      func(r map[string]any) { r["tests"].([]any)[4] = r["tests"].([]any)[0] },
		"bad result":          func(r map[string]any) { r["tests"].([]any)[3].(map[string]any)["result"] = "allowed" },
		"extra test field":    func(r map[string]any) { r["tests"].([]any)[0].(map[string]any)["service_ip"] = "10.96.0.1" },
		"wrong field type":    func(r map[string]any) { r["tests"] = "not array" },
	}
	for name, mutate := range changed {
		t.Run(name, func(t *testing.T) {
			var obj map[string]any
			if err := json.Unmarshal(base, &obj); err != nil {
				t.Fatal(err)
			}
			mutate(obj)
			b, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeAndProject(b); err == nil {
				t.Fatal("unsafe receipt accepted")
			}
		})
	}
	malformed := map[string][]byte{
		"empty": {}, "trailing": append(append([]byte{}, base...), []byte(` {}`)...),
		"double BOM": append([]byte{0xef, 0xbb, 0xbf, 0xef, 0xbb, 0xbf}, base...),
		"too large":  []byte(strings.Repeat("x", maxReceiptBytes+1)),
		"not object": []byte(`[]`),
	}
	for name, b := range malformed {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeAndProject(b); err == nil {
				t.Fatal("malformed accepted")
			}
		})
	}
}
