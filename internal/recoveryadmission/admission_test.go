package recoveryadmission

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/recoveryworkspace"
	"github.com/danielguillaumont/firedoor/internal/rehearsalworkspace"
)

const stageFixture = `C:\Users\Dev\AppData\Local\Firedoor\live-recovery-staging\20261110-231253933-3c138b60a075`
const kubeFixture = stageFixture + `\kubeconfig`

func encode(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func hash(v []byte) string { s := sha256.Sum256(v); return hex.EncodeToString(s[:]) }
func evidenceFixture() (map[string]any, map[string]any) {
	source := map[string]string{}
	for _, p := range rehearsalworkspace.SourcePaths {
		source[p] = strings.Repeat("a", 64)
	}
	intent := map[string]any{
		"schema_version": recoveryworkspace.Schema, "status": recoveryworkspace.Status,
		"source_commit": recoveryworkspace.BaseCommit, "cluster": TargetCluster,
		"context": "kind-" + TargetCluster, "kubeconfig_path": kubeFixture,
		"profile": "hardened", "cluster_created": false, "apply_authorized": false,
		"recovery_tested": false, "source_sha256": source,
	}
	receipt := map[string]any{
		"schema_version": "firedoor.v10.fd004-bootstrap-reconciliation.v1",
		"status":         "retained_infrastructure_verified_original_receipt_finalization_failed",
		"cluster":        TargetCluster, "context": "kind-" + TargetCluster,
		"source_commit": recoveryworkspace.BaseCommit, "stage_path": stageFixture,
		"reconciliation_utc":               "2026-11-11T03:00:00Z",
		"original_bootstrap_run_succeeded": false, "infrastructure_observed_ready": true,
		"original_bootstrap_receipt_status": "creation_started_retain_on_failure",
		"pending_bootstrap_receipt_status":  "bootstrap_ready_retained_no_terraform",
		"original_receipt_sha256":           OriginalSHA256, "pending_receipt_sha256": PendingSHA256,
		"private_kubeconfig_sha256": strings.Repeat("b", 64),
		"calico_manifest_sha256":    CalicoSHA256, "kubernetes_version": "v1.36.4",
		"node_names":              []string{TargetCluster + "-control-plane", TargetCluster + "-worker"},
		"calico_node_ready_count": 2, "calico_controllers_ready_count": 1,
		"coredns_ready_count": 2, "terraform_executed": false,
		"network_recovery_tested": false,
		"original_failure":        "System.ArgumentException on File.Replace(temp, receipt, null) in Windows PowerShell 5.1",
		"note":                    "Independent later read-only reconciliation. Does not retroactively mark original bootstrap execution successful.",
	}
	return intent, receipt
}

func newPlan() map[string]any {
	changes := []any{}
	for _, zone := range []string{"edge", "app", "data"} {
		for _, kind := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			labels := map[string]any{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform", "app.kubernetes.io/name": "firedoor-" + zone, "firedoor.dev/zone": zone}
			if kind == "kubernetes_namespace_v1" {
				labels["pod-security.kubernetes.io/enforce"] = "baseline"
			}
			name := "firedoor-" + zone
			if kind == "kubernetes_service_account_v1" {
				name += "-sa"
			}
			meta := map[string]any{"name": name, "labels": labels}
			if kind != "kubernetes_namespace_v1" {
				meta["namespace"] = "firedoor-" + zone
			}
			after := map[string]any{"metadata": []any{meta}}
			if kind == "kubernetes_service_account_v1" {
				after["automount_service_account_token"] = false
			}
			address := `module.workloads["` + zone + `"].` + kind + `.this`
			changes = append(changes, map[string]any{"address": address, "mode": "managed", "type": kind, "name": "this", "change": map[string]any{"actions": []any{"create"}, "before": nil, "after": after}})
		}
	}
	for _, zone := range []string{"app", "data"} {
		source := "edge"
		if zone == "data" {
			source = "app"
		}
		after := map[string]any{
			"metadata": []any{map[string]any{"name": "firedoor-" + zone + "-ingress", "namespace": "firedoor-" + zone, "labels": map[string]any{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}}},
			"spec": []any{map[string]any{
				"policy_types": []any{"Ingress"},
				"pod_selector": []any{map[string]any{"match_labels": map[string]any{"app.kubernetes.io/name": "firedoor-" + zone}}},
				"ingress": []any{map[string]any{
					"from":  []any{map[string]any{"namespace_selector": []any{map[string]any{"match_labels": map[string]any{"firedoor.dev/zone": source}}}}},
					"ports": []any{map[string]any{"port": "8080", "protocol": "TCP"}},
				}},
			}},
		}
		changes = append(changes, map[string]any{"address": `module.network_boundaries["` + zone + `"].kubernetes_network_policy_v1.this`, "mode": "managed", "type": "kubernetes_network_policy_v1", "name": "this", "change": map[string]any{"actions": []any{"create"}, "before": nil, "after": after}})
	}
	outputs := map[string]any{}
	for _, key := range []string{"active_profile", "owned_namespaces", "services", "managed_ingress_policies"} {
		after := any(map[string]any{"placeholder": "value"})
		if key == "active_profile" {
			after = "hardened"
		}
		outputs[key] = map[string]any{"actions": []any{"create"}, "before": nil, "after": after}
	}
	return map[string]any{
		"format_version": "1.2", "terraform_version": "1.17.1",
		"variables": map[string]any{
			"security_profile": map[string]any{"value": "hardened"},
			"kube_context":     map[string]any{"value": "kind-" + TargetCluster},
			"kubeconfig_path":  map[string]any{"value": kubeFixture},
		},
		"resource_changes": changes, "output_changes": outputs,
	}
}

func run(plan map[string]any, intent, rec map[string]any) (Report, error) {
	rawR := encode(rec)
	return Review(encode(plan), encode(intent), rawR, stageFixture, TargetCluster, hash(rawR))
}

func TestAcceptHardenedInitialPlanAsOfflineEvidenceOnly(t *testing.T) {
	i, r := evidenceFixture()
	p := newPlan()
	review, err := run(p, i, r)
	if err != nil {
		t.Fatal(err)
	}
	if review.SchemaVersion != Schema || review.Status != Status || review.ResourcesReviewed != 14 || review.HardenedPoliciesReviewed != 2 ||
		review.ApplyAuthorized || review.TerraformExecuted || review.TerraformAuthenticated || review.ClusterChecked || review.RecoveryProven || !review.HistoricalEvidenceHashMatched ||
		review.PlanSHA256 != hash(encode(p)) || review.Cluster != TargetCluster {
		t.Fatalf("unsafe review %+v", review)
	}
}

func TestRejectUnsafeAdmission(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]any, map[string]any, map[string]any)
	}{
		{"wrong-stage-identity", func(_ map[string]any, i, r map[string]any) { i["cluster"] = "firedoor-local" }},
		{"baseline-intent", func(_ map[string]any, i, r map[string]any) { i["profile"] = "baseline" }},
		{"intent-created-claim", func(_ map[string]any, i, r map[string]any) { i["cluster_created"] = true }},
		{"intent-apply-claim", func(_ map[string]any, i, r map[string]any) { i["apply_authorized"] = true }},
		{"lost-source-sha", func(_ map[string]any, i, r map[string]any) {
			delete(i["source_sha256"].(map[string]string), rehearsalworkspace.SourcePaths[0])
		}},
		{"wrong-source-sha", func(_ map[string]any, i, r map[string]any) {
			i["source_sha256"].(map[string]string)[rehearsalworkspace.SourcePaths[0]] = "abc"
		}},
		{"unknown-stage-field", func(_ map[string]any, i, r map[string]any) { i["untrusted"] = "yes" }},
		{"recon-no-ready", func(_ map[string]any, i, r map[string]any) { r["infrastructure_observed_ready"] = false }},
		{"recon-bad-status", func(_ map[string]any, i, r map[string]any) { r["status"] = "success" }},
		{"recon-original-success-lie", func(_ map[string]any, i, r map[string]any) { r["original_bootstrap_run_succeeded"] = true }},
		{"recon-terraform-executed", func(_ map[string]any, i, r map[string]any) { r["terraform_executed"] = true }},
		{"recon-false-node", func(_ map[string]any, i, r map[string]any) { r["node_names"] = []string{"firedoor-local-worker"} }},
		{"recon-unknown-field", func(_ map[string]any, i, r map[string]any) { r["extra"] = "attack" }},
		{"wrong-historical-sha", func(_ map[string]any, i, r map[string]any) { r["original_receipt_sha256"] = strings.Repeat("f", 64) }},
		{"wrong-recon-path", func(_ map[string]any, i, r map[string]any) { r["stage_path"] = `C:\other\place` }},
		{"baseline-plan", func(p, i, r map[string]any) {
			p["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "baseline"
		}},
		{"wrong-kubeconfig-plan", func(p, i, r map[string]any) {
			p["variables"].(map[string]any)["kubeconfig_path"].(map[string]any)["value"] = "C:\\Users\\Dev\\.kube\\config"
		}},
		{"foreign-resource", func(p, i, r map[string]any) {
			p["resource_changes"] = append(p["resource_changes"].([]any), map[string]any{"address": "other"})
		}},
		{"delete-action", func(p, i, r map[string]any) {
			p["resource_changes"].([]any)[0].(map[string]any)["change"].(map[string]any)["actions"] = []any{"delete"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, r := evidenceFixture()
			p := newPlan()
			tt.change(p, i, r)
			if got, e := run(p, i, r); e == nil {
				t.Fatalf("accepted altered evidence/plan %+v", got)
			}
		})
	}
}

func TestRejectTamperingAndMalformedEvidence(t *testing.T) {
	i, r := evidenceFixture()
	p := encode(newPlan())
	bi := encode(i)
	br := encode(r)
	want := hash(br)
	for name, v := range map[string][]byte{
		"trailing":  append(append([]byte{}, bi...), []byte(` {}`)...),
		"duplicate": []byte(`{"schema_version":"one","schema_version":"two"}`),
		"null":      append(append([]byte{}, bi...), 0),
		"oversize":  bytes.Repeat([]byte("x"), 128*1024+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := Review(p, v, br, stageFixture, TargetCluster, want); e == nil {
				t.Fatal("accepted malformed evidence")
			}
		})
	}
	if _, e := Review(p, bi, append(append([]byte{}, br...), byte('x')), stageFixture, TargetCluster, want); e == nil {
		t.Fatal("accepted modified historical reconciliation bytes")
	}
	for _, stage := range []string{`C:\Users\Dev\AppData\Local\Firedoor\live-recovery-staging\..\firedoor-local`, "/tmp/" + StageLeaf, stageFixture + `\..\other`} {
		if _, e := Review(p, bi, br, stage, TargetCluster, want); e == nil {
			t.Fatalf("accepted wrong stage %q", stage)
		}
	}
	if _, e := Review(p, bi, br, stageFixture, TargetCluster, strings.Repeat("0", 64)); e == nil {
		t.Fatal("accepted wrong checksum")
	}
	if _, e := Review(p, bi, br, stageFixture, "firedoor-local", want); e == nil {
		t.Fatal("accepted protected cluster")
	}
}
