package controls

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

func pbool(v bool) *bool  { return &v }
func pint(v int64) *int64 { return &v }

func ownedSnapshot() kubeinspect.Snapshot {
	s := kubeinspect.Snapshot{
		SchemaVersion: kubeinspect.Schema, Cluster: kubeinspect.Cluster, Context: kubeinspect.Context,
		CapturedAtUTC: time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC),
	}
	for _, name := range []string{"edge", "app", "data"} {
		ns := "firedoor-" + name
		z := kubeinspect.Zone{
			Name: name, Namespace: ns,
			Deployment: kubeinspect.Deployment{Name: ns, DesiredReplicas: 1, ReadyReplicas: 1, ServiceAccountName: ns + "-sa", AutomountToken: pbool(false),
				Containers: []kubeinspect.Container{{Name: "demo-http", Security: kubeinspect.Security{
					RunAsNonRoot: pbool(true), RunAsUser: pint(10001), AllowPrivilegeEscalation: pbool(false), Privileged: pbool(false),
				}}}},
			ServiceAccount: kubeinspect.Account{Name: ns + "-sa", AutomountToken: pbool(false)},
			Pods: []kubeinspect.Pod{{Name: ns + "-pod", Ready: true, Phase: "Running", Labels: map[string]string{
				"app.kubernetes.io/name": ns, "firedoor.dev/zone": name,
			}}},
		}
		if name != "edge" {
			from := "edge"
			if name == "data" {
				from = "app"
			}
			z.NetworkPolicies = []kubeinspect.Policy{{Name: ns + "-ingress", PolicyTypes: []string{"Ingress"},
				PodSelector: kubeinspect.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": ns}},
				Ingress: []kubeinspect.PolicyRule{{Peers: []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{
					MatchLabels: map[string]string{"firedoor.dev/zone": from},
				}}}}},
			}}
		}
		s.Zones = append(s.Zones, z)
	}
	return s
}

func finding(t *testing.T, f []Finding, zone, id string) Finding {
	t.Helper()
	for _, x := range f {
		if x.Scope.Target.Namespace == "firedoor-"+zone && x.Control.ID == id {
			return x
		}
	}
	t.Fatalf("missing %s in %s", id, zone)
	return Finding{}
}

func mustEvaluate(t *testing.T, s kubeinspect.Snapshot) []Finding {
	t.Helper()
	f, err := EvaluateSnapshot(s, "hardened")
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range f {
		if err := x.Validate(); err != nil {
			t.Fatalf("finding %d invalid: %v", i, err)
		}
	}
	return f
}

func TestFoundationExpectedObservedMatrix(t *testing.T) {
	f := mustEvaluate(t, ownedSnapshot())
	if len(f) != 18 {
		t.Fatalf("expected 18 findings, got %d", len(f))
	}
	counts := map[Verdict]int{}
	for _, x := range f {
		counts[x.Verdict]++
	}
	if counts[Pass] != 14 || counts[Fail] != 1 || counts[Unknown] != 3 {
		t.Fatalf("unsafe verdict distribution: %+v", counts)
	}
	t.Logf("Synthetic fixture (not live audit): 18 findings — pass=%d fail=%d unknown=%d", counts[Pass], counts[Fail], counts[Unknown])
	if finding(t, f, "edge", "CTRL-NET-001").Verdict != Fail {
		t.Fatal("edge without policy must fail")
	}
	if finding(t, f, "app", "CTRL-NET-001").Verdict != Pass || finding(t, f, "data", "CTRL-NET-001").Verdict != Pass {
		t.Fatal("app/data should be ingress-isolated")
	}
	for _, zone := range []string{"edge", "app", "data"} {
		if finding(t, f, zone, "CTRL-ID-003").Verdict != Unknown {
			t.Fatal("RBAC cannot pass without cluster-level evidence")
		}
	}
}

func TestSnapshotIdentityRejectsBadInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*kubeinspect.Snapshot)
	}{
		{"schema", func(s *kubeinspect.Snapshot) { s.SchemaVersion = "other" }},
		{"cluster", func(s *kubeinspect.Snapshot) { s.Cluster = "production" }},
		{"context", func(s *kubeinspect.Snapshot) { s.Context = "production" }},
		{"timestamp", func(s *kubeinspect.Snapshot) { s.CapturedAtUTC = time.Time{} }},
		{"missing zone", func(s *kubeinspect.Snapshot) { s.Zones = s.Zones[:2] }},
		{"duplicate zone", func(s *kubeinspect.Snapshot) { s.Zones[1] = s.Zones[0] }},
		{"out of scope namespace", func(s *kubeinspect.Snapshot) { s.Zones[1].Namespace = "kube-system" }},
		{"wrong deployment", func(s *kubeinspect.Snapshot) { s.Zones[1].Deployment.Name = "foreign" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ownedSnapshot()
			tc.change(&s)
			if _, err := EvaluateSnapshot(s, "hardened"); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
	if _, err := EvaluateSnapshot(ownedSnapshot(), "unknown"); err == nil {
		t.Fatal("unsupported profile accepted")
	}
}

func TestServiceAccountAndAutomountInheritance(t *testing.T) {
	s := ownedSnapshot()
	s.Zones[0].Deployment.ServiceAccountName = "default"
	s.Zones[1].Deployment.AutomountToken = nil         // SA=false is inherited
	s.Zones[2].Deployment.AutomountToken = pbool(true) // overrides SA=false
	f := mustEvaluate(t, s)
	if finding(t, f, "edge", "CTRL-ID-001").Verdict != Fail {
		t.Fatal("default SA must fail")
	}
	if finding(t, f, "app", "CTRL-ID-002").Verdict != Pass {
		t.Fatal("inherited SA=false should pass")
	}
	if finding(t, f, "data", "CTRL-ID-002").Verdict != Fail {
		t.Fatal("Pod=true override must fail")
	}

	s = ownedSnapshot()
	s.Zones[1].Deployment.AutomountToken = nil
	s.Zones[1].ServiceAccount.AutomountToken = nil
	if finding(t, mustEvaluate(t, s), "app", "CTRL-ID-002").Verdict != Fail {
		t.Fatal("implicit Kubernetes default=true must fail")
	}
}

func TestNetworkPolicySelectionAndUnionRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*kubeinspect.Zone)
		want   Verdict
	}{
		{"wrong pod selector", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].PodSelector.MatchLabels["app.kubernetes.io/name"] = "other"
		}, Fail},
		{"missing pod labels", func(z *kubeinspect.Zone) { z.Pods[0].Labels = nil }, Unknown},
		{"no pods", func(z *kubeinspect.Zone) { z.Pods = nil }, Unknown},
		{"wrong policy type", func(z *kubeinspect.Zone) { z.NetworkPolicies[0].PolicyTypes = []string{"Egress"} }, Fail},
		{"open all peer sources", func(z *kubeinspect.Zone) { z.NetworkPolicies[0].Ingress = []kubeinspect.PolicyRule{{}} }, Fail},
		{"open peer object", func(z *kubeinspect.Zone) { z.NetworkPolicies[0].Ingress[0].Peers = []kubeinspect.Peer{{}} }, Fail},
		{"all namespaces", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].Ingress[0].Peers = []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{}}}
		}, Fail},
		{"all namespaces and all pods", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].Ingress[0].Peers = []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{}, PodSelector: &kubeinspect.LabelSelector{}}}
		}, Fail},
		{"all IPv6 addresses", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].Ingress[0].Peers = []kubeinspect.Peer{{IPBlock: &kubeinspect.IPBlock{CIDR: "::/0"}}}
		}, Fail},
		{"all ipv4", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].Ingress[0].Peers = []kubeinspect.Peer{{IPBlock: &kubeinspect.IPBlock{CIDR: "0.0.0.0/0"}}}
		}, Fail},
		{"extra allow all policy", func(z *kubeinspect.Zone) {
			z.NetworkPolicies = append(z.NetworkPolicies, kubeinspect.Policy{Name: "backdoor", PodSelector: kubeinspect.LabelSelector{}, PolicyTypes: []string{"Ingress"}, Ingress: []kubeinspect.PolicyRule{{}}})
		}, Fail},
		{"no ingress rules is true default deny", func(z *kubeinspect.Zone) { z.NetworkPolicies[0].Ingress = nil }, Pass},
		{"unsupported selector", func(z *kubeinspect.Zone) {
			z.NetworkPolicies[0].PodSelector.MatchExpressions = []kubeinspect.SelectorExpr{{Key: "a", Operator: "Invalid"}}
		}, Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ownedSnapshot()
			tc.change(&s.Zones[1])
			f := mustEvaluate(t, s)
			if got := finding(t, f, "app", "CTRL-NET-001"); got.Verdict != tc.want {
				t.Fatalf("expected %s got %s: %s", tc.want, got.Verdict, got.Reason)
			}
		})
	}
}

func TestSelectorExpressionSemantics(t *testing.T) {
	labels := map[string]string{"app": "web", "tier": "api"}
	for _, tc := range []struct {
		op, key     string
		vals        []string
		want, known bool
	}{
		{"In", "app", []string{"web"}, true, true},
		{"NotIn", "app", []string{"db"}, true, true},
		{"NotIn", "missing", []string{"db"}, true, true},
		{"Exists", "tier", nil, true, true},
		{"DoesNotExist", "missing", nil, true, true},
		{"In", "missing", []string{"web"}, false, true},
		{"Exists", "missing", nil, false, true},
		{"Bad", "tier", nil, false, false},
	} {
		t.Run(fmt.Sprintf("%s-%s", tc.op, tc.key), func(t *testing.T) {
			got, known := selectorMatches(kubeinspect.LabelSelector{MatchExpressions: []kubeinspect.SelectorExpr{{Operator: tc.op, Key: tc.key, Values: tc.vals}}}, labels)
			if got != tc.want || known != tc.known {
				t.Fatalf("want %v/%v got %v/%v", tc.want, tc.known, got, known)
			}
		})
	}
}

func TestEffectiveContainerSecurityContexts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*kubeinspect.Zone)
		id     string
		want   Verdict
	}{
		{"uid root", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.RunAsUser = pint(0) }, "CTRL-WL-001", Fail},
		{"runAsNonRoot false", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.RunAsNonRoot = pbool(false) }, "CTRL-WL-001", Fail},
		{"missing nonroot", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.RunAsNonRoot = nil }, "CTRL-WL-001", Unknown},
		{"pod inherited nonroot", func(z *kubeinspect.Zone) {
			z.Deployment.Containers[0].Security.RunAsNonRoot = nil
			z.Deployment.PodSecurityContext.RunAsNonRoot = pbool(true)
		}, "CTRL-WL-001", Pass},
		{"allow escalation true", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.AllowPrivilegeEscalation = pbool(true) }, "CTRL-WL-002", Fail},
		{"privileged true", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.Privileged = pbool(true) }, "CTRL-WL-002", Fail},
		{"escalation nil", func(z *kubeinspect.Zone) { z.Deployment.Containers[0].Security.AllowPrivilegeEscalation = nil }, "CTRL-WL-002", Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ownedSnapshot()
			tc.change(&s.Zones[0])
			f := mustEvaluate(t, s)
			if got := finding(t, f, "edge", tc.id); got.Verdict != tc.want {
				t.Fatalf("wanted %s, got %s (%s)", tc.want, got.Verdict, got.Reason)
			}
		})
	}
}

func TestRBACNeverInfersEmptyBindingsAreSafe(t *testing.T) {
	s := ownedSnapshot()
	if finding(t, mustEvaluate(t, s), "edge", "CTRL-ID-003").Verdict != Unknown {
		t.Fatal("empty namespace binding list is not proof of no permissions")
	}
	z := &s.Zones[0]
	z.RoleBindings = []kubeinspect.RoleBinding{{Name: "root", RoleRef: kubeinspect.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"}, Subjects: []kubeinspect.Subject{{Kind: "ServiceAccount", Name: "firedoor-edge-sa", Namespace: "firedoor-edge"}}}}
	if finding(t, mustEvaluate(t, s), "edge", "CTRL-ID-003").Verdict != Fail {
		t.Fatal("broad ClusterRoleBinding via namespaced RoleBinding should fail")
	}

	s = ownedSnapshot()
	z = &s.Zones[0]
	z.RoleBindings = []kubeinspect.RoleBinding{{Name: "wild", RoleRef: kubeinspect.RoleRef{Kind: "Role", Name: "wildrole"}, Subjects: []kubeinspect.Subject{{Kind: "ServiceAccount", Name: "firedoor-edge-sa"}}}}
	z.Roles = []kubeinspect.Role{{Name: "wildrole", Rules: []kubeinspect.Rule{{Resources: []string{"secrets"}, Verbs: []string{"*"}}}}}
	if finding(t, mustEvaluate(t, s), "edge", "CTRL-ID-003").Verdict != Fail {
		t.Fatal("wildcard role should fail")
	}

	s = ownedSnapshot()
	z = &s.Zones[0]
	z.RoleBindings = []kubeinspect.RoleBinding{{Name: "reader", RoleRef: kubeinspect.RoleRef{Kind: "Role", Name: "reader"}, Subjects: []kubeinspect.Subject{{Kind: "ServiceAccount", Name: "firedoor-edge-sa"}}}}
	z.Roles = []kubeinspect.Role{{Name: "reader", Rules: []kubeinspect.Rule{{Resources: []string{"pods"}, Verbs: []string{"get"}}}}}
	f := finding(t, mustEvaluate(t, s), "edge", "CTRL-ID-003")
	if f.Verdict != Unknown || !strings.Contains(f.Reason, "ClusterRoleBindings") {
		t.Fatal("namespaced least privilege alone cannot prove effective RBAC")
	}
}
