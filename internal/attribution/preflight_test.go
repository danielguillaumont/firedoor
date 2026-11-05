package attribution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fixtureReader struct {
	context  string
	profile  string
	extras   map[string]string
	calls    []string
	drift    bool
	podReads map[string]int
}

func rawJSON(x any) []byte { b, _ := json.Marshal(x); return b }
func (f *fixtureReader) Read(_ context.Context, binary string, args ...string) ([]byte, error) {
	cmd := binary + " " + strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	for _, bad := range []string{" apply ", " delete ", " patch ", " create ", " exec ", " replace ", " rollout ", " scale ", " edit ", " annotate ", " label "} {
		if strings.Contains(" "+cmd+" ", bad) {
			return nil, errors.New("mutation command detected")
		}
	}
	if x, ok := f.extras[cmd]; ok {
		return []byte(x), nil
	}
	if cmd == "kubectl config current-context" {
		if f.context == "" {
			return []byte(OwnedContext), nil
		}
		return []byte(f.context), nil
	}
	if cmd == "kind get clusters" {
		return []byte("firedoor-local\n"), nil
	}
	if cmd == "terraform -chdir=infrastructure/terraform/testbed output -raw active_profile" {
		if f.profile == "" {
			return []byte("hardened"), nil
		}
		return []byte(f.profile), nil
	}
	if cmd == "kubectl --context kind-firedoor-local get nodes -o json" {
		return rawJSON(map[string]any{"items": []any{map[string]any{"metadata": map[string]any{"name": nodeName}, "status": map[string]any{"conditions": []any{map[string]string{"type": "Ready", "status": "True"}}}}}}), nil
	}
	for _, zone := range []string{"edge", "app", "data"} {
		ns := "firedoor-" + zone
		labels := map[string]string{"firedoor.dev/zone": zone, "firedoor.dev/managed-by": "terraform", "app.kubernetes.io/part-of": "firedoor", applicationLabel: ns, "kubernetes.io/metadata.name": ns}
		if cmd == "kubectl --context kind-firedoor-local get namespace "+ns+" -o json" {
			return rawJSON(map[string]any{"metadata": map[string]any{"name": ns, "labels": labels}, "status": map[string]string{"phase": "Active"}}), nil
		}
		if cmd == "kubectl --context kind-firedoor-local -n "+ns+" get pods -l "+applicationLabel+"="+ns+" -o json" {
			if f.podReads == nil {
				f.podReads = make(map[string]int)
			}
			f.podReads[ns]++
			uid := "uid-" + ns
			if f.drift && f.podReads[ns] > 1 {
				uid = "replacement-uid"
			}
			return rawJSON(map[string]any{"items": []any{map[string]any{
				"metadata": map[string]any{"name": ns + "-abcde-12345", "namespace": ns, "uid": uid, "labels": labels},
				"spec":     map[string]string{"nodeName": nodeName},
				"status":   map[string]any{"phase": "Running", "podIP": map[string]string{"edge": "192.168.0.10", "app": "192.168.0.11", "data": "192.168.0.12"}[zone], "conditions": []any{map[string]string{"type": "Ready", "status": "True"}}, "containerStatuses": []any{map[string]any{"name": "demo-http", "ready": true}}},
			}}}), nil
		}
		if cmd == "kubectl --context kind-firedoor-local -n "+ns+" get networkpolicies -o json" {
			return validPolicies(zone), nil
		}
	}
	return nil, fmt.Errorf("unexpected read: %s", cmd)
}

func validPolicies(zone string) []byte {
	if zone == "edge" {
		return rawJSON(map[string]any{"items": []any{}})
	}
	source := "edge"
	if zone == "data" {
		source = "app"
	}
	policy := map[string]any{
		"metadata": map[string]any{"name": "firedoor-" + zone + "-ingress", "namespace": "firedoor-" + zone, "labels": map[string]string{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}},
		"spec": map[string]any{
			"podSelector": map[string]any{"matchLabels": map[string]string{applicationLabel: "firedoor-" + zone}},
			"policyTypes": []string{"Ingress"},
			"ingress":     []any{map[string]any{"from": []any{map[string]any{"namespaceSelector": map[string]any{"matchLabels": map[string]string{"firedoor.dev/zone": source}}}}, "ports": []any{map[string]any{"protocol": "TCP", "port": 8080}}}},
		},
	}
	return rawJSON(map[string]any{"items": []any{policy}})
}

func TestReadOnlyPreflightAdmitsFixedOwnedHardenedLab(t *testing.T) {
	f := &fixtureReader{}
	got, err := RunPreflight(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "admissible_for_later_review" || len(got.Namespaces) != 3 || got.Namespaces[0].PolicyCount != 0 ||
		got.Namespaces[1].SourceNamespace != "firedoor-edge" || got.Namespaces[2].SourceNamespace != "firedoor-app" ||
		!reflect.DeepEqual(got.PlannedExperiments, []string{"witness-edge-to-data", "witness-data-to-app"}) {
		t.Fatalf("bad preflight: %+v", got)
	}
	if len(f.calls) != 16 {
		t.Fatalf("unexpected command count %d: %v", len(f.calls), f.calls)
	}
}
func TestReadOnlyPreflightFailsClosed(t *testing.T) {
	key := "kubectl --context kind-firedoor-local -n firedoor-data get networkpolicies -o json"
	foreign := string(validPolicies("data"))
	foreign = strings.Replace(foreign, "firedoor-data-ingress", "firedoor-v04-witness-edge-data", 1)
	extra := string(rawJSON(map[string]any{"items": []any{map[string]string{"invalid": "unexpected"}}}))
	cases := []struct {
		name  string
		setup func(*fixtureReader)
	}{
		{"wrong_context", func(f *fixtureReader) { f.context = "production-cluster" }},
		{"baseline_profile", func(f *fixtureReader) { f.profile = "baseline" }},
		{"wrong_policy_name", func(f *fixtureReader) { f.extras = map[string]string{key: foreign} }},
		{"extra_policy", func(f *fixtureReader) { f.extras = map[string]string{key: extra} }},
		{"pod_replacement", func(f *fixtureReader) { f.drift = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fixtureReader{}
			tc.setup(f)
			if _, err := RunPreflight(context.Background(), f); err == nil {
				t.Fatal("unsafe lab admitted")
			}
		})
	}
}
func TestPolicyDriftExamplesRejected(t *testing.T) {
	baseline := string(validPolicies("app"))
	for _, tc := range []struct{ name, from, to string }{
		{"permit_every_zone", "firedoor.dev/zone", "foreign-zone"},
		{"different_source", "\"edge\"", "\"data\""},
		{"wrong_port", "8080", "8081"},
		{"wrong_protocol", "TCP", "UDP"},
		{"wrong_policy_type", "Ingress", "Egress"},
		{"widen_target_label", "firedoor-app\"", "other-app\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(baseline, tc.from, tc.to, 1)
			if _, err := ValidatePolicySnapshot([]byte(bad), "firedoor-app"); err == nil {
				t.Fatal("policy drift accepted")
			}
		})
	}
	if _, err := ValidatePolicySnapshot(validPolicies("app"), "production"); err == nil {
		t.Fatal("foreign namespace admitted")
	}
}
