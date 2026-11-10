package liveverify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func fd004ReviewFakePolicy(zone string, hardened bool) map[string]any {
	src := "edge"
	if zone == "data" {
		src = "app"
	}
	labels := map[string]any{}
	if hardened {
		labels["firedoor.dev/zone"] = src
	}
	return map[string]any{
		"metadata": []any{map[string]any{"name": "firedoor-" + zone + "-ingress", "namespace": "firedoor-" + zone, "generation": json.Number("7"), "labels": map[string]any{"firedoor.dev/managed-by": "terraform", "app.kubernetes.io/part-of": "firedoor"}}},
		"spec": []any{map[string]any{
			"pod_selector": []any{map[string]any{"match_labels": map[string]any{"app.kubernetes.io/name": "firedoor-" + zone}}},
			"policy_types": []any{"Ingress"},
			"ingress": []any{map[string]any{
				"from":  []any{map[string]any{"namespace_selector": []any{map[string]any{"match_labels": labels}}}},
				"ports": []any{map[string]any{"port": "8080", "protocol": "TCP"}},
			}},
		}},
	}
}
func fd004ReviewFakePlan() map[string]any {
	variables := map[string]any{}
	for k, v := range map[string]string{"security_profile": "baseline", "kube_context": "kind-firedoor-local", "kubeconfig_path": "~/.kube/config"} {
		variables[k] = map[string]any{"value": v}
	}
	changes := []any{}
	for _, zone := range []string{"edge", "app", "data"} {
		for _, kind := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			changes = append(changes, map[string]any{"address": fmt.Sprintf(`module.workloads[%q].%s.this`, zone, kind), "type": kind, "mode": "managed", "change": map[string]any{"actions": []any{"no-op"}, "before": map[string]any{"id": "constant"}, "after": map[string]any{"id": "constant"}}})
		}
	}
	for _, zone := range []string{"app", "data"} {
		changes = append(changes, map[string]any{
			"address": fmt.Sprintf(`module.network_boundaries[%q].kubernetes_network_policy_v1.this`, zone), "type": "kubernetes_network_policy_v1", "mode": "managed",
			"change": map[string]any{"actions": []any{"update"}, "before": fd004ReviewFakePolicy(zone, true), "after": fd004ReviewFakePolicy(zone, false)},
		})
	}
	return map[string]any{
		"format_version": "1.2", "terraform_version": "1.17.1", "variables": variables, "resource_changes": changes,
		"output_changes": map[string]any{"active_profile": map[string]any{"actions": []any{"update"}, "before": "hardened", "after": "baseline"}},
	}
}
func fd004ReviewJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func fd004ReviewPolicyChange(p map[string]any, zone string) map[string]any {
	for _, x := range p["resource_changes"].([]any) {
		r := x.(map[string]any)
		if strings.Contains(r["address"].(string), `network_boundaries["`+zone+`"]`) {
			return r["change"].(map[string]any)
		}
	}
	panic("missing test policy")
}
func TestFD004BaselinePlanReviewValid(t *testing.T) {
	p := fd004ReviewFakePlan()
	report, err := ReviewFD004BaselinePlanJSON(fd004ReviewJSON(p))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "plan_shape_reviewed_no_apply_authorization" || report.ApplyAuthorized || report.PolicyCausalityProven || report.InPlacePolicyUpdates != 2 || len(report.ReviewedAddresses) != 2 {
		t.Fatalf("invalid review: %+v", report)
	}
	// JSON input can contain a legitimate numeric generation value; clone must
	// preserve number equivalence when comparing unmodified attributes.
}
func TestFD004BaselinePlanReviewRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"wrong_profile", func(p map[string]any) {
			p["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "hardened"
		}},
		{"foreign_context", func(p map[string]any) {
			p["variables"].(map[string]any)["kube_context"].(map[string]any)["value"] = "production"
		}},
		{"extra_variable", func(p map[string]any) { p["variables"].(map[string]any)["creds"] = map[string]any{"value": "no"} }},
		{"wrong_profile_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["active_profile"].(map[string]any)["before"] = "baseline"
		}},
		{"changed_unrelated_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["services"] = map[string]any{"actions": []any{"update"}}
		}},
		{"delete_action", func(p map[string]any) { fd004ReviewPolicyChange(p, "app")["actions"] = []any{"delete"} }},
		{"replace_action", func(p map[string]any) { fd004ReviewPolicyChange(p, "data")["actions"] = []any{"delete", "create"} }},
		{"replacement_paths", func(p map[string]any) { fd004ReviewPolicyChange(p, "app")["replace_paths"] = []any{[]any{"metadata"}} }},
		{"extra_resource", func(p map[string]any) {
			p["resource_changes"] = append(p["resource_changes"].([]any), map[string]any{"address": "other"})
		}},
		{"changed_workload", func(p map[string]any) {
			p["resource_changes"].([]any)[0].(map[string]any)["change"].(map[string]any)["actions"] = []any{"update"}
		}},
		{"data_before_not_hardened", func(p map[string]any) {
			c := fd004ReviewPolicyChange(p, "data")
			s, _ := fd004Selector(c["before"].(map[string]any))
			s["match_labels"] = map[string]any{}
		}},
		{"app_after_still_hardened", func(p map[string]any) {
			c := fd004ReviewPolicyChange(p, "app")
			s, _ := fd004Selector(c["after"].(map[string]any))
			s["match_labels"] = map[string]any{"firedoor.dev/zone": "edge"}
		}},
		{"unexpected_port", func(p map[string]any) {
			c := fd004ReviewPolicyChange(p, "data")
			a := c["after"].(map[string]any)
			sp, _ := fd004PlanFirst(a, "spec")
			in, _ := fd004PlanFirst(sp, "ingress")
			po, _ := fd004PlanFirst(in, "ports")
			po["port"] = "9090"
		}},
		{"changed_policy_name", func(p map[string]any) {
			c := fd004ReviewPolicyChange(p, "data")
			a := c["after"].(map[string]any)
			m, _ := fd004PlanFirst(a, "metadata")
			m["name"] = "foreign"
		}},
		{"missing_output", func(p map[string]any) { delete(p, "output_changes") }},
		{"resource_drift", func(p map[string]any) { p["resource_drift"] = []any{map[string]any{"address": "foreign"}} }},
		{"type_mismatch", func(p map[string]any) {
			fd004ReviewPolicyChange(p, "data")
			p["resource_changes"].([]any)[13].(map[string]any)["type"] = "kubernetes_namespace_v1"
		}},
		{"missing_noop_before", func(p map[string]any) {
			delete(p["resource_changes"].([]any)[0].(map[string]any)["change"].(map[string]any), "before")
		}},
		{"missing_change", func(p map[string]any) { delete(fd004ReviewPolicyChange(p, "app"), "after") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fd004ReviewFakePlan()
			tc.edit(p)
			if _, err := ReviewFD004BaselinePlanJSON(fd004ReviewJSON(p)); err == nil {
				t.Fatal("unsafe plan accepted")
			}
		})
	}
}
func TestFD004BaselinePlanReviewMalformed(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty": nil, "trailing": append(fd004ReviewJSON(fd004ReviewFakePlan()), []byte(`{}`)...),
		"duplicate":      []byte(`{"format_version":"1.2","format_version":"1.2"}`),
		"oversized":      bytes.Repeat([]byte(" "), FD004MaxPlanJSONBytes+1),
		"unknown_format": []byte(`{"format_version":"2.0"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReviewFD004BaselinePlanJSON(b); err == nil {
				t.Fatal("malformed plan accepted")
			}
		})
	}
}
