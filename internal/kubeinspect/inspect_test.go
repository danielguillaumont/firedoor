package kubeinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	responses map[string]string
	calls     []string
}

func cmdKey(name string, args ...string) string { return name + " " + strings.Join(args, " ") }
func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := cmdKey(name, args...)
	f.calls = append(f.calls, key)
	response, ok := f.responses[key]
	if !ok {
		return nil, errors.New("unexpected test command: " + key)
	}
	return []byte(response), nil
}
func fixtureRunner() *fakeRunner {
	f := &fakeRunner{responses: map[string]string{}}
	f.responses[cmdKey("kubectl", "config", "current-context")] = Context + "\n"
	f.responses[cmdKey("kind", "get", "clusters")] = "unrelated\n" + Cluster + "\n"
	f.responses[cmdKey("kubectl", "--context", Context, "get", "nodes", "-o", "json")] = `{"items":[{"metadata":{"name":"firedoor-local-control-plane"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`
	for _, z := range zones {
		ns := "firedoor-" + z
		meta := fmt.Sprintf(`{"name":%q,"namespace":%q,"labels":{"firedoor.dev/zone":%q,"firedoor.dev/managed-by":"terraform","app.kubernetes.io/part-of":"firedoor"}}`, ns, ns, z)
		f.responses[cmdKey("kubectl", "--context", Context, "get", "namespace", ns, "-o", "json")] = fmt.Sprintf(`{"metadata":%s}`, meta)
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "deployment", ns, "-o", "json")] = fmt.Sprintf(`{"metadata":%s,"spec":{"replicas":1,"template":{"spec":{"serviceAccountName":%q,"automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true},"containers":[{"name":"demo-http","image":"busybox:1.36.1","securityContext":{"allowPrivilegeEscalation":false,"runAsNonRoot":true,"runAsUser":10001}}]}}},"status":{"readyReplicas":1}}`, meta, ns+"-sa")
		accountMeta := fmt.Sprintf(`{"name":%q,"namespace":%q,"labels":{"firedoor.dev/zone":%q,"firedoor.dev/managed-by":"terraform","app.kubernetes.io/part-of":"firedoor"}}`, ns+"-sa", ns, z)
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "serviceaccount", ns+"-sa", "-o", "json")] = fmt.Sprintf(`{"metadata":%s,"automountServiceAccountToken":false}`, accountMeta)
		podMeta := fmt.Sprintf(`{"name":%q,"namespace":%q,"labels":{"firedoor.dev/zone":%q,"firedoor.dev/managed-by":"terraform","app.kubernetes.io/part-of":"firedoor","app.kubernetes.io/name":%q}}`, ns+"-pod", ns, z, ns)
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "pods", "-l", "app.kubernetes.io/name="+ns, "-o", "json")] = fmt.Sprintf(`{"items":[{"metadata":%s,"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`, podMeta)
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "networkpolicies", "-o", "json")] = `{"items":[]}`
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "roles", "-o", "json")] = `{"items":[]}`
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "rolebindings", "-o", "json")] = `{"items":[]}`
	}
	return f
}

func TestCollectOwnedSnapshotWithObservedFields(t *testing.T) {
	f := fixtureRunner()
	for _, z := range []string{"app", "data"} {
		ns := "firedoor-" + z
		f.responses[cmdKey("kubectl", "--context", Context, "-n", ns, "get", "networkpolicies", "-o", "json")] = fmt.Sprintf(`{"items":[{"metadata":{"name":%q,"namespace":%q},"spec":{"podSelector":{"matchLabels":{"app.kubernetes.io/name":%q}},"policyTypes":["Ingress"],"ingress":[{"from":[{"namespaceSelector":{"matchLabels":{"firedoor.dev/zone":"edge"}}}],"ports":[{"protocol":"TCP","port":8080}]}]}}]}`, ns+"-ingress", ns, ns)
	}
	f.responses[cmdKey("kubectl", "--context", Context, "-n", "firedoor-edge", "get", "roles", "-o", "json")] = `{"items":[{"metadata":{"name":"reader","namespace":"firedoor-edge"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get"]}]}]}`
	f.responses[cmdKey("kubectl", "--context", Context, "-n", "firedoor-edge", "get", "rolebindings", "-o", "json")] = `{"items":[{"metadata":{"name":"reader-binding","namespace":"firedoor-edge"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"reader"},"subjects":[{"kind":"ServiceAccount","name":"firedoor-edge-sa","namespace":"firedoor-edge"}]}]}`
	at := time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)
	s, err := (Collector{Runner: f, Now: func() time.Time { return at }}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Cluster != Cluster || s.Context != Context || s.SchemaVersion != Schema || len(s.Zones) != 3 || !s.CapturedAtUTC.Equal(at) {
		t.Fatalf("bad identity/metadata: %+v", s)
	}
	for i, z := range zones {
		got := s.Zones[i]
		if got.Name != z || got.Deployment.ReadyReplicas != 1 || len(got.Pods) != 1 || !got.Pods[0].Ready || got.Pods[0].Labels["app.kubernetes.io/name"] != "firedoor-"+z {
			t.Errorf("missing zone details: %+v", got)
		}
		if got.ServiceAccount.AutomountToken == nil || *got.ServiceAccount.AutomountToken || got.Deployment.AutomountToken == nil || *got.Deployment.AutomountToken {
			t.Error("automount token was not decoded")
		}
		if got.Deployment.Containers[0].Security.AllowPrivilegeEscalation == nil || *got.Deployment.Containers[0].Security.AllowPrivilegeEscalation {
			t.Error("container security context was not decoded")
		}
	}
	if len(s.Zones[1].NetworkPolicies) != 1 || s.Zones[1].NetworkPolicies[0].Ingress[0].Peers[0].NamespaceSelector.MatchLabels["firedoor.dev/zone"] != "edge" {
		t.Fatal("policy selector was not decoded")
	}
	if len(s.Zones[0].Roles) != 1 || s.Zones[0].Roles[0].Rules[0].Verbs[0] != "get" || s.Zones[0].RoleBindings[0].RoleRef.Name != "reader" {
		t.Fatal("RBAC role/binding was not decoded")
	}
	jsonBytes, err := json.Marshal(s)
	if err != nil || !strings.Contains(string(jsonBytes), `"run_as_non_root":true`) {
		t.Fatalf("bad stable JSON representation: %s %v", jsonBytes, err)
	}
	if strings.Contains(string(jsonBytes), "evidence_consistent") || strings.Contains(string(jsonBytes), `"verdict":"pass"`) {
		t.Fatal("inventory must not assert security verdicts")
	}
	for _, call := range f.calls {
		if strings.Contains(call, " apply ") || strings.Contains(call, " exec ") || strings.Contains(call, " delete ") || strings.Contains(call, " patch ") {
			t.Fatalf("unsafe command: %s", call)
		}
	}
}

func TestFailClosedOnWrongContextAndNamespaceOwnership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fakeRunner)
	}{
		{"wrong context", func(f *fakeRunner) { f.responses[cmdKey("kubectl", "config", "current-context")] = "production" }},
		{"missing cluster", func(f *fakeRunner) { f.responses[cmdKey("kind", "get", "clusters")] = "other" }},
		{"wrong node", func(f *fakeRunner) {
			f.responses[cmdKey("kubectl", "--context", Context, "get", "nodes", "-o", "json")] = `{"items":[]}`
		}},
		{"unowned namespace", func(f *fakeRunner) {
			f.responses[cmdKey("kubectl", "--context", Context, "get", "namespace", "firedoor-app", "-o", "json")] = `{"metadata":{"name":"firedoor-app","labels":{"firedoor.dev/managed-by":"someone-else"}}}`
		}},
		{"unowned pod", func(f *fakeRunner) {
			f.responses[cmdKey("kubectl", "--context", Context, "-n", "firedoor-edge", "get", "pods", "-l", "app.kubernetes.io/name=firedoor-edge", "-o", "json")] = `{"items":[{"metadata":{"namespace":"production","name":"oops"}}]}`
		}},
		{"malformed API response", func(f *fakeRunner) {
			f.responses[cmdKey("kubectl", "--context", Context, "-n", "firedoor-data", "get", "roles", "-o", "json")] = `not-json`
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtureRunner()
			tc.mutate(f)
			if _, err := (Collector{Runner: f}).Collect(context.Background()); err == nil {
				t.Fatal("unsafe/invalid inventory accepted")
			}
		})
	}
}

func TestRunnerWhitelistRejectsWritesAndForeignNamespace(t *testing.T) {
	f := fixtureRunner()
	c := Collector{Runner: f}
	for _, args := range [][]string{
		{"--context", Context, "delete", "pod", "x"},
		{"--context", Context, "-n", "production", "get", "pods"},
		{"--context", "production", "get", "pods"},
		{"--context", Context, "exec", "foo"},
	} {
		if _, err := c.run(context.Background(), "kubectl", args...); err == nil {
			t.Fatalf("unsafe command accepted: %v", args)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("rejected commands reached runner: %v", f.calls)
	}
}

func TestEgressPeerAndPolicyPortAreTyped(t *testing.T) {
	in := []kubePolicyRule{{To: []kubePeer{{IPBlock: &kubeIPBlock{CIDR: "10.0.0.0/8"}}}, Ports: []kubePort{{Port: "dns"}}}}
	out := fromEgressRules(in)
	if out[0].Peers[0].IPBlock.CIDR != "10.0.0.0/8" || out[0].Ports[0].Port != "dns" {
		t.Fatalf("egress conversion failed: %+v", out)
	}
}
