package controls

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

// EvaluateSnapshot is a pure, deterministic configuration evaluator. It makes
// no Kubernetes, Terraform, filesystem or network requests. A verdict is NOT
// evidence that the CNI enforced a policy or that RBAC permissions are absent.
func EvaluateSnapshot(s kubeinspect.Snapshot, profile string) ([]Finding, error) {
	if s.SchemaVersion != kubeinspect.Schema || s.Cluster != kubeinspect.Cluster || s.Context != kubeinspect.Context || s.CapturedAtUTC.IsZero() {
		return nil, fmt.Errorf("invalid or out-of-scope Kubernetes inventory metadata")
	}
	if profile != "baseline" && profile != "hardened" {
		return nil, fmt.Errorf("expected explicitly selected baseline or hardened profile")
	}
	if len(s.Zones) != 3 {
		return nil, fmt.Errorf("expected exactly three owned zones; got %d", len(s.Zones))
	}
	byName := make(map[string]kubeinspect.Zone, 3)
	for _, z := range s.Zones {
		if z.Name != "edge" && z.Name != "app" && z.Name != "data" {
			return nil, fmt.Errorf("unexpected zone %q", z.Name)
		}
		if z.Namespace != "firedoor-"+z.Name || z.Deployment.Name != z.Namespace {
			return nil, fmt.Errorf("invalid namespace or deployment identity in %q", z.Name)
		}
		if _, duplicate := byName[z.Name]; duplicate {
			return nil, fmt.Errorf("duplicate zone %q", z.Name)
		}
		byName[z.Name] = z
	}

	definitions := FoundationDefinitions()
	findings := make([]Finding, 0, 18)
	for _, name := range []string{"edge", "app", "data"} {
		z, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("missing owned zone %q", name)
		}
		scope := InspectionScope{
			Cluster: s.Cluster, Context: s.Context, Profile: profile,
			Target: ObjectReference{Namespace: z.Namespace, Kind: "Deployment", Name: z.Deployment.Name},
		}
		for _, d := range definitions {
			f := Finding{Control: d, Scope: scope, Applicability: Applicable}
			e := func(kind, name, path, observed, expected string) Evidence {
				return Evidence{Object: ObjectReference{Namespace: z.Namespace, Kind: kind, Name: name},
					FieldPath: path, Observed: observed, Expected: expected, CapturedAtUTC: s.CapturedAtUTC}
			}
			switch d.ID {
			case "CTRL-ID-001":
				expected := z.Namespace + "-sa"
				f.Evidence = []Evidence{
					e("Deployment", z.Deployment.Name, "spec.template.spec.serviceAccountName", present(z.Deployment.ServiceAccountName), expected),
					e("ServiceAccount", z.ServiceAccount.Name, "metadata.name", present(z.ServiceAccount.Name), expected),
				}
				if z.Deployment.ServiceAccountName == expected && z.ServiceAccount.Name == expected {
					f.Verdict = Pass
				} else {
					f.Verdict, f.Reason = Fail, "workload does not reference its expected dedicated ServiceAccount"
				}
			case "CTRL-ID-002":
				f.Evidence = []Evidence{
					e("Deployment", z.Deployment.Name, "spec.template.spec.automountServiceAccountToken", boolVal(z.Deployment.AutomountToken), "false (effective)"),
					e("ServiceAccount", z.ServiceAccount.Name, "automountServiceAccountToken", boolVal(z.ServiceAccount.AutomountToken), "false (effective)"),
				}
				effective := true // API default when Pod and ServiceAccount both omit the field.
				if z.ServiceAccount.AutomountToken != nil {
					effective = *z.ServiceAccount.AutomountToken
				}
				if z.Deployment.AutomountToken != nil {
					effective = *z.Deployment.AutomountToken // Pod takes precedence.
				}
				if !effective {
					f.Verdict = Pass
				} else {
					f.Verdict, f.Reason = Fail, "effective token automount is enabled (or Kubernetes defaults to enabled)"
				}
			case "CTRL-ID-003":
				f = evaluateRBAC(f, z, e)
			case "CTRL-NET-001":
				f = evaluateIngress(f, z, e)
			case "CTRL-WL-001":
				f = evaluateNonRoot(f, z, e)
			case "CTRL-WL-002":
				f = evaluateEscalation(f, z, e)
			default:
				return nil, fmt.Errorf("unknown control definition %q", d.ID)
			}
			if err := f.Validate(); err != nil {
				return nil, fmt.Errorf("invalid finding %s in %s: %w", d.ID, z.Namespace, err)
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

func present(s string) string {
	if s == "" {
		return "<unset>"
	}
	return s
}

func boolVal(b *bool) string {
	if b == nil {
		return "<unset>"
	}
	if *b {
		return "true"
	}
	return "false"
}

func evaluateRBAC(f Finding, z kubeinspect.Zone, e func(string, string, string, string, string) Evidence) Finding {
	// Namespaced RoleBindings are only a subset of effective RBAC. In particular,
	// this snapshot does NOT include ClusterRoleBindings and cannot prove a pass.
	f.Verdict, f.Reason = Unknown, "effective authorization cannot be determined: ClusterRoleBindings and SubjectAccessReview are not inspected"
	f.Evidence = []Evidence{e("ServiceAccount", z.ServiceAccount.Name, "namespacedRoleBindings", fmt.Sprintf("%d inspected; ClusterRoleBindings not inspected", len(z.RoleBindings)), "least privilege across all effective bindings")}
	for _, b := range z.RoleBindings {
		bound := false
		for _, sub := range b.Subjects {
			if sub.Kind == "ServiceAccount" && sub.Name == z.Deployment.ServiceAccountName && (sub.Namespace == "" || sub.Namespace == z.Namespace) {
				bound = true
			}
		}
		if !bound {
			continue
		}
		f.Evidence = append(f.Evidence, e("RoleBinding", b.Name, "roleRef", b.RoleRef.Kind+"/"+b.RoleRef.Name, "bounded, scoped RBAC grant"))
		if b.RoleRef.Kind == "ClusterRole" && slices.Contains([]string{"cluster-admin", "admin", "edit"}, b.RoleRef.Name) {
			f.Verdict, f.Reason = Fail, "workload ServiceAccount has a broad, named built-in ClusterRole binding"
			return f
		}
		if b.RoleRef.Kind != "Role" {
			continue // Other ClusterRoles cannot be evaluated without reading their rules.
		}
		for _, role := range z.Roles {
			if role.Name != b.RoleRef.Name {
				continue
			}
			for _, rule := range role.Rules {
				if slices.Contains(rule.Verbs, "*") || slices.Contains(rule.Resources, "*") || slices.Contains(rule.NonResourceURLs, "*") {
					f.Evidence = append(f.Evidence, e("Role", role.Name, "rules", "wildcard verb/resource/nonResourceURL", "no wildcard privileges"))
					f.Verdict, f.Reason = Fail, "a namespaced binding grants wildcard privileges to the workload ServiceAccount"
					return f
				}
			}
		}
	}
	return f
}

func evaluateIngress(f Finding, z kubeinspect.Zone, e func(string, string, string, string, string) Evidence) Finding {
	// This control checks declared *ingress* isolation only, not egress or CNI
	// enforcement. Kubernetes combines selected policies by union of allows.
	if len(z.Pods) == 0 {
		f.Verdict, f.Reason = Unknown, "no observed Pod labels available to establish NetworkPolicy selection"
		return f
	}
	for _, pod := range z.Pods {
		if pod.Name == "" || len(pod.Labels) == 0 {
			f.Verdict, f.Reason = Unknown, "Pod name/labels missing; policy selection cannot be established"
			return f
		}
		selected := 0
		for _, p := range z.NetworkPolicies {
			match, known := selectorMatches(p.PodSelector, pod.Labels)
			if !known {
				f.Verdict, f.Reason = Unknown, "unsupported or malformed policy selector; refusing a passing verdict"
				return f
			}
			if !match || !slices.Contains(p.PolicyTypes, "Ingress") {
				continue
			}
			selected++
			f.Evidence = append(f.Evidence, e("NetworkPolicy", p.Name, "spec.podSelector+policyTypes", "selects "+pod.Name+"; types="+strings.Join(p.PolicyTypes, ","), "Ingress isolation selecting observed pod"))
			for _, rule := range p.Ingress {
				if broadIngressRule(rule) {
					f.Verdict, f.Reason = Fail, "selected NetworkPolicy includes an unrestricted or globally broad ingress peer"
					f.Evidence = append(f.Evidence, e("NetworkPolicy", p.Name, "spec.ingress.from", "unrestricted peer detected", "bounded ingress peers"))
					return f
				}
			}
		}
		if selected == 0 {
			f.Verdict, f.Reason = Fail, "no ingress-isolating NetworkPolicy selects an observed workload Pod"
			f.Evidence = append(f.Evidence, e("Pod", pod.Name, "metadata.labels+selectedNetworkPolicies", "0 ingress-isolating policies", ">=1 selecting Ingress policy"))
			return f
		}
	}
	f.Verdict, f.Reason = Pass, "observed Pods are ingress-isolated by selecting policies with no unbounded ingress peer; no egress or enforcement claim"
	return f
}

func broadIngressRule(r kubeinspect.PolicyRule) bool {
	if len(r.Peers) == 0 { // Empty 'from' allows all sources, possibly port-limited.
		return true
	}
	for _, peer := range r.Peers {
		if peer.NamespaceSelector == nil && peer.PodSelector == nil && peer.IPBlock == nil {
			return true // Empty peer {} allows every source.
		}
		if peer.NamespaceSelector != nil && peer.IPBlock == nil && emptySelector(*peer.NamespaceSelector) &&
			(peer.PodSelector == nil || emptySelector(*peer.PodSelector)) {
			return true // All namespaces and all pods.
		}
		if peer.IPBlock != nil && (peer.IPBlock.CIDR == "0.0.0.0/0" || peer.IPBlock.CIDR == "::/0") {
			return true // Extremely broad address range, even with exceptions.
		}
	}
	return false
}

func emptySelector(s kubeinspect.LabelSelector) bool {
	return len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

func selectorMatches(s kubeinspect.LabelSelector, labels map[string]string) (bool, bool) {
	for k, v := range s.MatchLabels {
		if labels[k] != v {
			return false, true
		}
	}
	for _, expr := range s.MatchExpressions {
		if expr.Key == "" {
			return false, false
		}
		v, ok := labels[expr.Key]
		switch expr.Operator {
		case "In":
			if len(expr.Values) == 0 {
				return false, false
			}
			if !ok || !slices.Contains(expr.Values, v) {
				return false, true
			}
		case "NotIn":
			if len(expr.Values) == 0 {
				return false, false
			}
			if ok && slices.Contains(expr.Values, v) {
				return false, true
			}
		case "Exists":
			if len(expr.Values) != 0 {
				return false, false
			}
			if !ok {
				return false, true
			}
		case "DoesNotExist":
			if len(expr.Values) != 0 {
				return false, false
			}
			if ok {
				return false, true
			}
		default:
			return false, false
		}
	}
	return true, true
}

func evaluateNonRoot(f Finding, z kubeinspect.Zone, e func(string, string, string, string, string) Evidence) Finding {
	if len(z.Deployment.Containers) == 0 {
		f.Verdict, f.Reason = Unknown, "no container security contexts were observed"
		return f
	}
	unknown := false
	for _, c := range z.Deployment.Containers {
		if c.Name == "" {
			f.Verdict, f.Reason = Unknown, "unnamed container"
			return f
		}
		runAs := c.Security.RunAsNonRoot
		if runAs == nil {
			runAs = z.Deployment.PodSecurityContext.RunAsNonRoot
		}
		uid := c.Security.RunAsUser
		if uid == nil {
			uid = z.Deployment.PodSecurityContext.RunAsUser
		}
		f.Evidence = append(f.Evidence, e("Deployment", z.Deployment.Name,
			"spec.template.spec.containers["+c.Name+"].securityContext.runAsNonRoot",
			boolVal(runAs)+" (effective), runAsUser="+intVal(uid), "runAsNonRoot=true, uid != 0"))
		if (runAs != nil && !*runAs) || (uid != nil && *uid == 0) {
			f.Verdict, f.Reason = Fail, "container permits root or explicitly selects UID 0"
			return f
		}
		if runAs == nil {
			unknown = true
		}
	}
	if unknown {
		f.Verdict, f.Reason = Unknown, "runAsNonRoot is unset; cannot establish explicit non-root enforcement"
	} else {
		f.Verdict = Pass
	}
	return f
}

func intVal(v *int64) string {
	if v == nil {
		return "<unset>"
	}
	return fmt.Sprintf("%d", *v)
}

func evaluateEscalation(f Finding, z kubeinspect.Zone, e func(string, string, string, string, string) Evidence) Finding {
	if len(z.Deployment.Containers) == 0 {
		f.Verdict, f.Reason = Unknown, "no container security contexts were observed"
		return f
	}
	unknown := false
	for _, c := range z.Deployment.Containers {
		if c.Name == "" {
			f.Verdict, f.Reason = Unknown, "unnamed container"
			return f
		}
		f.Evidence = append(f.Evidence, e("Deployment", z.Deployment.Name,
			"spec.template.spec.containers["+c.Name+"].securityContext.allowPrivilegeEscalation",
			boolVal(c.Security.AllowPrivilegeEscalation)+", privileged="+boolVal(c.Security.Privileged),
			"allowPrivilegeEscalation=false, privileged!=true"))
		if (c.Security.AllowPrivilegeEscalation != nil && *c.Security.AllowPrivilegeEscalation) ||
			(c.Security.Privileged != nil && *c.Security.Privileged) {
			f.Verdict, f.Reason = Fail, "container has privilege escalation or privileged mode enabled"
			return f
		}
		if c.Security.AllowPrivilegeEscalation == nil {
			unknown = true
		}
	}
	if unknown {
		f.Verdict, f.Reason = Unknown, "allowPrivilegeEscalation not explicitly set on every container"
	} else {
		f.Verdict = Pass
	}
	return f
}
