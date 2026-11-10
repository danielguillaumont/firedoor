package destroyreview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const cluster = "firedoor-rh-012345abcdef"
const kc = `C:\Users\Test\AppData\Local\Firedoor\disposable-rehearsals\a\kubeconfig`

func validPlan() map[string]any {
	vars := map[string]any{
		"security_profile": map[string]any{"value": "hardened"},
		"kube_context":     map[string]any{"value": "kind-" + cluster},
		"kubeconfig_path":  map[string]any{"value": kc},
	}
	changes := []any{}
	for address, kind := range expected() {
		zone := ""
		for _, z := range []string{"edge", "app", "data"} {
			if strings.Contains(address, fmt.Sprintf(`[%q]`, z)) {
				zone = z
				break
			}
		}
		name := "firedoor-" + zone
		if kind == "kubernetes_service_account_v1" {
			name += "-sa"
		}
		if kind == "kubernetes_network_policy_v1" {
			name += "-ingress"
		}
		labels := map[string]any{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}
		if kind != "kubernetes_network_policy_v1" {
			labels["app.kubernetes.io/name"] = "firedoor-" + zone
			labels["firedoor.dev/zone"] = zone
		}
		if kind == "kubernetes_namespace_v1" {
			labels["pod-security.kubernetes.io/enforce"] = "baseline"
		}
		metadata := map[string]any{"name": name, "labels": labels}
		if kind != "kubernetes_namespace_v1" {
			metadata["namespace"] = "firedoor-" + zone
		}
		before := map[string]any{"metadata": []any{metadata}}
		if kind == "kubernetes_network_policy_v1" {
			src := "edge"
			if zone == "data" {
				src = "app"
			}
			before["spec"] = []any{map[string]any{
				"policy_types": []any{"Ingress"},
				"pod_selector": []any{map[string]any{"match_labels": map[string]any{"app.kubernetes.io/name": "firedoor-" + zone}}},
				"ingress": []any{map[string]any{
					"from":  []any{map[string]any{"namespace_selector": []any{map[string]any{"match_labels": map[string]any{"firedoor.dev/zone": src}}}}},
					"ports": []any{map[string]any{"port": "8080", "protocol": "TCP"}},
				}},
			}}
		}
		changes = append(changes, map[string]any{
			"address": address, "mode": "managed", "type": kind, "name": "this",
			"change": map[string]any{"actions": []any{"delete"}, "before": before, "after": nil},
		})
	}
	return map[string]any{"format_version": "1.2", "variables": vars, "resource_changes": changes}
}
func check(t *testing.T, doc map[string]any, pass bool) {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReviewJSON(b, cluster, kc)
	if pass && (err != nil || r.ResourceCount != 14 || r.PolicyCount != 2 || r.DestroyAuthorized || r.ClusterAuthenticated || r.StateAuthenticated || r.RecoveryProven) {
		t.Fatalf("expected valid review, got %+v: %v", r, err)
	}
	if !pass && err == nil {
		t.Fatalf("unexpected acceptance: %+v", r)
	}
}
func TestValidDeleteOnlyAndNoAuthorization(t *testing.T) { check(t, validPlan(), true) }
func TestFailClosedMutations(t *testing.T) {
	cases := map[string]func(map[string]any){
		"wrong-profile": func(m map[string]any) {
			m["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "baseline"
		},
		"wrong-context": func(m map[string]any) {
			m["variables"].(map[string]any)["kube_context"].(map[string]any)["value"] = "kind-firedoor-local"
		},
		"wrong-kubeconfig": func(m map[string]any) {
			m["variables"].(map[string]any)["kubeconfig_path"].(map[string]any)["value"] = "~/.kube/config"
		},
		"missing-resource": func(m map[string]any) { a := m["resource_changes"].([]any); m["resource_changes"] = a[:13] },
		"extra-resource":   func(m map[string]any) { a := m["resource_changes"].([]any); m["resource_changes"] = append(a, a[0]) },
		"foreign-resource": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			a[0].(map[string]any)["address"] = "kubernetes_namespace_v1.production"
		},
		"create-action": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			a[0].(map[string]any)["change"].(map[string]any)["actions"] = []any{"create"}
		},
		"replace-action": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			a[0].(map[string]any)["change"].(map[string]any)["actions"] = []any{"delete", "create"}
		},
		"unexpected-after": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			a[0].(map[string]any)["change"].(map[string]any)["after"] = map[string]any{}
		},
		"missing-before": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			a[0].(map[string]any)["change"].(map[string]any)["before"] = nil
		},
		"wrong-owner": func(m map[string]any) {
			a := m["resource_changes"].([]any)
			b := a[0].(map[string]any)["change"].(map[string]any)["before"].(map[string]any)
			b["metadata"].([]any)[0].(map[string]any)["labels"].(map[string]any)["firedoor.dev/managed-by"] = "kubectl"
		},
		"resource-drift": func(m map[string]any) { m["resource_drift"] = []any{map[string]any{"address": "bad"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) { m := validPlan(); mutate(m); check(t, m, false) })
	}
}
func TestPolicyDowngradeRejected(t *testing.T) {
	m := validPlan()
	for _, entry := range m["resource_changes"].([]any) {
		r := entry.(map[string]any)
		if r["type"] == "kubernetes_network_policy_v1" {
			b := r["change"].(map[string]any)["before"].(map[string]any)
			sp := b["spec"].([]any)[0].(map[string]any)
			from := sp["ingress"].([]any)[0].(map[string]any)["from"].([]any)[0].(map[string]any)
			from["namespace_selector"].([]any)[0].(map[string]any)["match_labels"] = map[string]any{}
			break
		}
	}
	check(t, m, false)
}
func TestDuplicateKeysAndTrailingDataRejected(t *testing.T) {
	for _, raw := range []string{
		`{"format_version":"1.2","format_version":"1.2"}`,
		`{"format_version":"1.2"} {"format_version":"1.2"}`,
		`{"format_version":"1.2","resource_changes":[{"x":1,"x":2}]}`,
	} {
		if _, err := ReviewJSON([]byte(raw), cluster, kc); err == nil {
			t.Fatal("accepted malformed document")
		}
	}
}
func TestClusterNotNormalLab(t *testing.T) {
	b, _ := json.Marshal(validPlan())
	if _, err := ReviewJSON(b, "firedoor-local", kc); err == nil {
		t.Fatal("accepted original cluster")
	}
}
func TestOversizedAndNULRejected(t *testing.T) {
	if _, err := ReviewJSON(make([]byte, Limit+1), cluster, kc); err == nil {
		t.Fatal("accepted oversized")
	}
	if _, err := ReviewJSON([]byte{0}, cluster, kc); err == nil {
		t.Fatal("accepted NUL")
	}
}
