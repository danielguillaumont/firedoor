package deployreview

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const testCluster = "firedoor-rh-0ffb4d40a3c2"
const testKube = "C:\\Users\\Dev\\AppData\\Local\\Firedoor\\disposable-rehearsals\\20261110-063823384-0ffb4d40a3c2\\kubeconfig"

func fixture() map[string]any {
	changes := []any{}
	for address, typ := range expectedResources() {
		zone, role := zoneAndType(address)
		name := "firedoor-" + zone
		if role == "serviceaccount" {
			name += "-sa"
		}
		if role == "policy" {
			name += "-ingress"
		}
		labels := map[string]any{"app.kubernetes.io/part-of": "firedoor", "app.kubernetes.io/name": "firedoor-" + zone, "firedoor.dev/managed-by": "terraform", "firedoor.dev/zone": zone}
		if role == "namespace" {
			labels["pod-security.kubernetes.io/enforce"] = "baseline"
		}
		if role == "policy" {
			delete(labels, "firedoor.dev/zone")
			delete(labels, "app.kubernetes.io/name")
		}
		meta := map[string]any{"name": name, "labels": labels}
		if role != "namespace" {
			meta["namespace"] = "firedoor-" + zone
		}
		after := map[string]any{"metadata": []any{meta}}
		if role == "serviceaccount" {
			after["automount_service_account_token"] = false
		}
		if role == "policy" {
			src := "edge"
			if zone == "data" {
				src = "app"
			}
			sourceSelector := map[string]any{"match_labels": map[string]any{"firedoor.dev/zone": src}}
			from := map[string]any{"namespace_selector": []any{sourceSelector}}
			port := map[string]any{"port": "8080", "protocol": "TCP"}
			ingress := map[string]any{"from": []any{from}, "ports": []any{port}}
			podSelector := map[string]any{"match_labels": map[string]any{"app.kubernetes.io/name": "firedoor-" + zone}}
			spec := map[string]any{"policy_types": []any{"Ingress"}, "pod_selector": []any{podSelector}, "ingress": []any{ingress}}
			after["spec"] = []any{spec}
		}
		changes = append(changes, map[string]any{"address": address, "mode": "managed", "type": typ, "name": "this", "change": map[string]any{"actions": []any{"create"}, "before": nil, "after": after}})
	}
	outputs := map[string]any{}
	for _, name := range []string{"active_profile", "owned_namespaces", "services", "managed_ingress_policies"} {
		after := any(map[string]any{"placeholder": "output"})
		if name == "active_profile" {
			after = "hardened"
		}
		outputs[name] = map[string]any{"actions": []any{"create"}, "before": nil, "after": after}
	}
	return map[string]any{"format_version": "1.2", "terraform_version": "1.17.1", "variables": map[string]any{"security_profile": map[string]any{"value": "hardened"}, "kube_context": map[string]any{"value": "kind-" + testCluster}, "kubeconfig_path": map[string]any{"value": testKube}}, "resource_changes": changes, "output_changes": outputs}
}
func toJSON(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func changeFor(p map[string]any, needle string) map[string]any {
	for _, v := range p["resource_changes"].([]any) {
		r := v.(map[string]any)
		if strings.Contains(r["address"].(string), needle) {
			return r
		}
	}
	panic("missing test resource")
}
func policy(p map[string]any) map[string]any {
	return changeFor(p, `network_boundaries["app"]`)["change"].(map[string]any)["after"].(map[string]any)
}
func TestValidInitialHardenedPlan(t *testing.T) {
	p := fixture()
	r, e := ReviewJSON(toJSON(p), testCluster, testKube)
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != Status || r.ResourceCount != 14 || r.PolicyCount != 2 || r.ApplyAuthorized || r.ClusterChecked || r.RecoveryProven || r.TerraformAuthenticated || len(r.Addresses) != 14 {
		t.Fatalf("wrong safety contract: %+v", r)
	}
}
func TestRejectUnsafeInitialPlan(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"baseline_profile", func(p map[string]any) {
			p["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "baseline"
		}},
		{"foreign_context", func(p map[string]any) {
			p["variables"].(map[string]any)["kube_context"].(map[string]any)["value"] = "kind-production"
		}},
		{"foreign_kubeconfig", func(p map[string]any) {
			p["variables"].(map[string]any)["kubeconfig_path"].(map[string]any)["value"] = "~/.kube/config"
		}},
		{"unknown_variable", func(p map[string]any) { p["variables"].(map[string]any)["unsafe"] = "yes" }},
		{"delete_action", func(p map[string]any) {
			changeFor(p, "kubernetes_service_v1")["change"].(map[string]any)["actions"] = []any{"delete"}
		}},
		{"replacement", func(p map[string]any) {
			changeFor(p, "kubernetes_service_v1")["change"].(map[string]any)["actions"] = []any{"delete", "create"}
		}},
		{"replace_paths", func(p map[string]any) {
			changeFor(p, "kubernetes_service_v1")["change"].(map[string]any)["replace_paths"] = []any{[]any{"metadata"}}
		}},
		{"existing_resource", func(p map[string]any) {
			changeFor(p, "kubernetes_service_v1")["change"].(map[string]any)["before"] = map[string]any{"id": "bad"}
		}},
		{"foreign_resource", func(p map[string]any) {
			p["resource_changes"] = append(p["resource_changes"].([]any), map[string]any{"address": "foreign"})
		}},
		{"duplicate_address", func(p map[string]any) { a := p["resource_changes"].([]any); a[0] = a[1] }},
		{"missing_resource", func(p map[string]any) { a := p["resource_changes"].([]any); p["resource_changes"] = a[:len(a)-1] }},
		{"resource_type_changed", func(p map[string]any) { changeFor(p, "kubernetes_service_v1")["type"] = "kubernetes_namespace_v1" }},
		{"resource_name_changed", func(p map[string]any) { changeFor(p, "kubernetes_service_v1")["name"] = "other" }},
		{"foreign_namespace", func(p map[string]any) {
			after := changeFor(p, "kubernetes_service_account_v1")["change"].(map[string]any)["after"].(map[string]any)
			after["metadata"].([]any)[0].(map[string]any)["namespace"] = "kube-system"
		}},
		{"wrong_owner", func(p map[string]any) {
			after := changeFor(p, "kubernetes_service_v1")["change"].(map[string]any)["after"].(map[string]any)
			after["metadata"].([]any)[0].(map[string]any)["labels"].(map[string]any)["firedoor.dev/managed-by"] = "other"
		}},
		{"sa_token_enabled", func(p map[string]any) {
			a := changeFor(p, "kubernetes_service_account_v1")["change"].(map[string]any)["after"].(map[string]any)
			a["automount_service_account_token"] = true
		}},
		{"baseline_policy_selector", func(p map[string]any) {
			spec := policy(p)["spec"].([]any)[0].(map[string]any)
			ing := spec["ingress"].([]any)[0].(map[string]any)
			from := ing["from"].([]any)[0].(map[string]any)
			from["namespace_selector"].([]any)[0].(map[string]any)["match_labels"] = map[string]any{}
		}},
		{"wrong_policy_port", func(p map[string]any) {
			spec := policy(p)["spec"].([]any)[0].(map[string]any)
			ing := spec["ingress"].([]any)[0].(map[string]any)
			ing["ports"].([]any)[0].(map[string]any)["port"] = "9090"
		}},
		{"wrong_policy_type", func(p map[string]any) {
			spec := policy(p)["spec"].([]any)[0].(map[string]any)
			spec["policy_types"] = []any{"Egress"}
		}},
		{"missing_output", func(p map[string]any) { delete(p["output_changes"].(map[string]any), "owned_namespaces") }},
		{"wrong_profile_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["active_profile"].(map[string]any)["after"] = "baseline"
		}},
		{"drift", func(p map[string]any) { p["resource_drift"] = []any{map[string]any{"address": "unexpected"}} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := fixture()
			c.mutate(p)
			if r, e := ReviewJSON(toJSON(p), testCluster, testKube); e == nil {
				t.Fatalf("accepted unsafe plan: %+v", r)
			}
		})
	}
}
func TestRejectMalformed(t *testing.T) {
	for name, b := range map[string][]byte{"empty": nil, "trailing": append(toJSON(fixture()), []byte(` {}`)...), "duplicate": []byte(`{"format_version":"1.2","format_version":"1.2"}`), "oversize": bytes.Repeat([]byte("x"), Limit+1), "double_bom": append([]byte{0xef, 0xbb, 0xbf, 0xef, 0xbb, 0xbf}, toJSON(fixture())...), "unknown_format": []byte(`{"format_version":"2.0"}`)} {
		t.Run(name, func(t *testing.T) {
			if _, e := ReviewJSON(b, testCluster, testKube); e == nil {
				t.Fatal("malformed plan accepted")
			}
		})
	}
	if _, e := ReviewJSON(toJSON(fixture()), "firedoor-local", testKube); e == nil {
		t.Fatal("real cluster name accepted")
	}
}
