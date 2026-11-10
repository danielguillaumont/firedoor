package recoverybundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testBundle(t *testing.T) (string, string, time.Time) {
	t.Helper()
	dir := t.TempDir()
	now := time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)
	state := []byte(`{"version":4,"terraform_version":"1.17.1","serial":7,"lineage":"local-test"}`)
	statePath := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(statePath, state, 0600); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"terraform.tfstate.backup": state}
	manifests := make([]any, 0, 2)
	for i, pair := range [][2]string{{"app", "edge"}, {"data", "app"}} {
		m := expectedManifest(pair[0], pair[1])
		b, e := json.MarshalIndent(m, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		files[fileNames[i+1]] = b
		manifests = append(manifests, m)
	}
	report := map[string]any{
		"schema_version": sourceSchema, "scenario": "FD-004", "cluster": "firedoor-local", "context": "kind-firedoor-local", "profile": "hardened",
		"captured_at_utc": now.Add(-5 * time.Minute), "state_address_count": 14, "terraform_outputs_sha256": strings.Repeat("a", 64),
		"recovery_manifests": manifests, "status": "hardened_preflight_consistent_recovery_not_exercised", "apply_authorized": false,
		"recovery_tested": false, "terraform_authenticated": false, "policy_causality_proven": false, "limitations": "Recovery not exercised",
	}
	b, e := json.Marshal(report)
	if e != nil {
		t.Fatal(e)
	}
	files["recovery-preflight-report.json"] = b
	var lines []string
	for _, n := range fileNames {
		if e = os.WriteFile(filepath.Join(dir, n), files[n], 0600); e != nil {
			t.Fatal(e)
		}
		lines = append(lines, fmt.Sprintf("%s SHA256 %s", n, hexSum(files[n])))
	}
	if e = os.WriteFile(filepath.Join(dir, "SHA256.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	return dir, statePath, now
}
func TestRecoveryBundleValidIsNeverAuthorization(t *testing.T) {
	b, s, now := testBundle(t)
	r, e := Verify(b, s, now)
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "bundle_integrity_checked_recovery_not_exercised" || r.ApplyAuthorized || r.RecoveryTested || r.PolicyCausalityProven || r.CheckedFiles != 4 || r.StateSHA256 == "" {
		t.Fatalf("unsafe success: %+v", r)
	}
}
func TestRecoveryBundleRejectsTampering(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, b, s string)
	}{
		{"state changed", func(t *testing.T, b, s string) { os.WriteFile(s, []byte(`{"serial":9}`), 0600) }},
		{"missing artifact", func(t *testing.T, b, s string) { os.Remove(filepath.Join(b, "firedoor-app-ingress.json")) }},
		{"foreign artifact", func(t *testing.T, b, s string) { os.WriteFile(filepath.Join(b, "extra.txt"), []byte("x"), 0600) }},
		{"manifest tampering", func(t *testing.T, b, s string) {
			os.WriteFile(filepath.Join(b, "firedoor-data-ingress.json"), []byte(`{"kind":"NetworkPolicy"}`), 0600)
		}},
		{"hash list tampering", func(t *testing.T, b, s string) { os.WriteFile(filepath.Join(b, "SHA256.txt"), []byte("garbage"), 0600) }},
		{"duplicate hash line", func(t *testing.T, b, s string) {
			p := filepath.Join(b, "SHA256.txt")
			raw, _ := os.ReadFile(p)
			l := strings.Split(string(raw), "\n")
			l[1] = l[0]
			os.WriteFile(p, []byte(strings.Join(l, "\n")), 0600)
		}},
		{"symlink manifest", func(t *testing.T, b, s string) {
			p := filepath.Join(b, "firedoor-app-ingress.json")
			os.Remove(p)
			if e := os.Symlink(s, p); e != nil {
				t.Skipf("symlinks unavailable: %v", e)
			}
		}},
		{"malformed report", func(t *testing.T, b, s string) {
			os.WriteFile(filepath.Join(b, "recovery-preflight-report.json"), []byte("{"), 0600)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, s, now := testBundle(t)
			tc.mutate(t, b, s)
			if _, e := Verify(b, s, now); e == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
}
func rewrite(t *testing.T, b, n string, data []byte) {
	t.Helper()
	p := filepath.Join(b, n)
	if e := os.WriteFile(p, data, 0600); e != nil {
		t.Fatal(e)
	}
	hashPath := filepath.Join(b, "SHA256.txt")
	raw, e := os.ReadFile(hashPath)
	if e != nil {
		t.Fatal(e)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		cols := strings.Fields(l)
		if cols[0] == n {
			cols[2] = hexSum(data)
		}
		lines = append(lines, strings.Join(cols, " "))
	}
	if e := os.WriteFile(hashPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestRecoveryBundleRejectsRehashedUnsafeContent(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing authorization field", func(r map[string]any) { delete(r, "apply_authorized") }},
		{"unknown security field", func(r map[string]any) { r["recovery_authorized"] = true }},
		{"false authorization", func(r map[string]any) { r["apply_authorized"] = true }},
		{"false recovery", func(r map[string]any) { r["recovery_tested"] = true }},
		{"foreign cluster", func(r map[string]any) { r["cluster"] = "production" }},
		{"baseline", func(r map[string]any) { r["profile"] = "baseline" }},
		{"invalid state inventory", func(r map[string]any) { r["state_address_count"] = float64(13) }},
		{"stale capture", func(r map[string]any) { r["captured_at_utc"] = "2026-11-01T01:00:00Z" }},
		{"future capture", func(r map[string]any) { r["captured_at_utc"] = "2026-11-12T01:00:00Z" }},
		{"altered output digest", func(r map[string]any) { r["terraform_outputs_sha256"] = "bad" }},
		{"embedded weak policy", func(r map[string]any) {
			ms := r["recovery_manifests"].([]any)
			m := ms[0].(map[string]any)
			spec := m["spec"].(map[string]any)
			spec["ingress"] = []any{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, s, now := testBundle(t)
			raw, _ := os.ReadFile(filepath.Join(b, "recovery-preflight-report.json"))
			var r map[string]any
			json.Unmarshal(raw, &r)
			tc.mutate(r)
			newRaw, _ := json.Marshal(r)
			rewrite(t, b, "recovery-preflight-report.json", newRaw)
			if _, e := Verify(b, s, now); e == nil {
				t.Fatal("rehashed unsafe artifact accepted")
			}
		})
	}
}
func TestRecoveryBundleRejectsRehashedWeakManifest(t *testing.T) {
	b, s, now := testBundle(t)
	raw, _ := os.ReadFile(filepath.Join(b, "firedoor-app-ingress.json"))
	var m map[string]any
	json.Unmarshal(raw, &m)
	spec := m["spec"].(map[string]any)
	spec["podSelector"] = map[string]any{}
	data, _ := json.Marshal(m)
	rewrite(t, b, "firedoor-app-ingress.json", data)
	if _, e := Verify(b, s, now); e == nil {
		t.Fatal("rehashed weak manifest accepted")
	}
}
