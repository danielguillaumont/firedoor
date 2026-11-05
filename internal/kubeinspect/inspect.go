package kubeinspect

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"
)

// Runner is deliberately narrow so tests can substitute immutable API fixtures.
// The production runner runs only the explicitly constructed read-only commands.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type CLI struct{}

func (CLI) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("read-only command timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("read-only %s command failed: %w", name, err)
	}
	return out, nil
}

type Collector struct {
	Runner Runner
	Now    func() time.Time
}

func (c Collector) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if c.Runner == nil {
		return nil, fmt.Errorf("no command runner")
	}
	// Exact allowlist prevents accidental reads of Secrets or foreign resources.
	if name == "kind" && slices.Equal(args, []string{"get", "clusters"}) {
		return c.Runner.Run(ctx, name, args...)
	}
	if name == "kubectl" && slices.Equal(args, []string{"config", "current-context"}) {
		return c.Runner.Run(ctx, name, args...)
	}
	if name != "kubectl" {
		return nil, fmt.Errorf("read-only command rejected")
	}
	if slices.Equal(args, []string{"--context", Context, "get", "nodes", "-o", "json"}) {
		return c.Runner.Run(ctx, name, args...)
	}
	for _, zone := range zones {
		ns := "firedoor-" + zone
		if slices.Equal(args, []string{"--context", Context, "get", "namespace", ns, "-o", "json"}) {
			return c.Runner.Run(ctx, name, args...)
		}
		prefix := []string{"--context", Context, "-n", ns, "get"}
		for _, resource := range []string{"networkpolicies", "roles", "rolebindings"} {
			if slices.Equal(args, append(append([]string{}, prefix...), resource, "-o", "json")) {
				return c.Runner.Run(ctx, name, args...)
			}
		}
		if slices.Equal(args, append(append([]string{}, prefix...), "deployment", ns, "-o", "json")) ||
			slices.Equal(args, append(append([]string{}, prefix...), "serviceaccount", ns+"-sa", "-o", "json")) ||
			slices.Equal(args, append(append([]string{}, prefix...), "pods", "-l", "app.kubernetes.io/name="+ns, "-o", "json")) {
			return c.Runner.Run(ctx, name, args...)
		}
	}
	return nil, fmt.Errorf("Kubernetes read outside the explicit Firedoor allowlist rejected")
}

func (c Collector) get(ctx context.Context, dest any, args ...string) error {
	full := append([]string{"--context", Context}, args...)
	out, err := c.run(ctx, "kubectl", full...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, dest); err != nil {
		return fmt.Errorf("invalid Kubernetes JSON response: %w", err)
	}
	return nil
}

// Collect reads only the dedicated kind node and Firedoor-owned namespaces.
// It never evaluates controls or changes resources. Failures do not mean pass.
func (c Collector) Collect(ctx context.Context) (Snapshot, error) {
	s := Snapshot{SchemaVersion: Schema, Cluster: Cluster, Context: Context,
		Limitations: "Read-only desired-state/resource inventory, not effective RBAC authorization or CNI policy enforcement. ClusterRoleBindings, admission policies, and Terraform state are not inspected."}
	current, err := c.run(ctx, "kubectl", "config", "current-context")
	if err != nil {
		return Snapshot{}, fmt.Errorf("context check: %w", err)
	}
	if strings.TrimSpace(string(current)) != Context {
		return Snapshot{}, fmt.Errorf("current kubectl context is not %s; refusing inspection", Context)
	}
	clusters, err := c.run(ctx, "kind", "get", "clusters")
	if err != nil {
		return Snapshot{}, fmt.Errorf("kind ownership check: %w", err)
	}
	found := false
	for _, line := range strings.Split(string(clusters), "\n") {
		if strings.TrimSpace(line) == Cluster {
			found = true
		}
	}
	if !found {
		return Snapshot{}, fmt.Errorf("dedicated kind cluster %s was not found", Cluster)
	}
	var nodes nodeList
	if err := c.get(ctx, &nodes, "get", "nodes", "-o", "json"); err != nil {
		return Snapshot{}, fmt.Errorf("node inspection: %w", err)
	}
	if len(nodes.Items) != 1 || nodes.Items[0].Metadata.Name != "firedoor-local-control-plane" {
		return Snapshot{}, fmt.Errorf("unexpected control-plane inventory; refusing inspection")
	}
	ready := false
	for _, condition := range nodes.Items[0].Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			ready = true
		}
	}
	if !ready {
		return Snapshot{}, fmt.Errorf("dedicated node not Ready")
	}

	// Verify ALL namespace ownership before reading any tenant resources.
	for _, zone := range zones {
		var ns kubeNamespace
		name := "firedoor-" + zone
		if err := c.get(ctx, &ns, "get", "namespace", name, "-o", "json"); err != nil {
			return Snapshot{}, fmt.Errorf("namespace %s: %w", name, err)
		}
		if ns.Metadata.Name != name || !ownedLabels(ns.Metadata.Labels, zone) {
			return Snapshot{}, fmt.Errorf("namespace %s missing Firedoor Terraform ownership labels", name)
		}
	}
	for _, zone := range zones {
		slot, err := c.collectZone(ctx, zone)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%s: %w", zone, err)
		}
		s.Zones = append(s.Zones, slot)
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	s.CapturedAtUTC = now().UTC()
	return s, nil
}

func ownedLabels(labels map[string]string, zone string) bool {
	return labels["firedoor.dev/managed-by"] == "terraform" && labels["firedoor.dev/zone"] == zone && labels["app.kubernetes.io/part-of"] == "firedoor"
}

func (c Collector) collectZone(ctx context.Context, zone string) (Zone, error) {
	ns := "firedoor-" + zone
	out := Zone{Name: zone, Namespace: ns, Pods: []Pod{}, NetworkPolicies: []Policy{}, Roles: []Role{}, RoleBindings: []RoleBinding{}}
	var deployment kubeDeployment
	if err := c.get(ctx, &deployment, "-n", ns, "get", "deployment", ns, "-o", "json"); err != nil {
		return Zone{}, err
	}
	if deployment.Metadata.Name != ns || deployment.Metadata.Namespace != ns || !ownedLabels(deployment.Metadata.Labels, zone) {
		return Zone{}, fmt.Errorf("unexpected Deployment identity/ownership")
	}
	out.Deployment = Deployment{Name: ns, ReadyReplicas: deployment.Status.ReadyReplicas,
		ServiceAccountName: deployment.Spec.Template.Spec.ServiceAccountName,
		AutomountToken:     deployment.Spec.Template.Spec.AutomountServiceAccountToken,
		PodSecurityContext: deployment.Spec.Template.Spec.SecurityContext.output()}
	if deployment.Spec.Replicas != nil {
		out.Deployment.DesiredReplicas = *deployment.Spec.Replicas
	} else {
		out.Deployment.DesiredReplicas = 1
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return Zone{}, fmt.Errorf("Deployment has no containers")
	}
	for _, ctr := range deployment.Spec.Template.Spec.Containers {
		if ctr.Name == "" {
			return Zone{}, fmt.Errorf("container missing name")
		}
		out.Deployment.Containers = append(out.Deployment.Containers, Container{Name: ctr.Name, Image: ctr.Image, Security: ctr.SecurityContext.output()})
	}

	var account kubeAccount
	if err := c.get(ctx, &account, "-n", ns, "get", "serviceaccount", ns+"-sa", "-o", "json"); err != nil {
		return Zone{}, err
	}
	if account.Metadata.Namespace != ns || account.Metadata.Name != ns+"-sa" || !ownedLabels(account.Metadata.Labels, zone) {
		return Zone{}, fmt.Errorf("unexpected dedicated ServiceAccount identity/ownership")
	}
	out.ServiceAccount = Account{Name: account.Metadata.Name, AutomountToken: account.AutomountServiceAccountToken}

	var pods kubePodList
	if err := c.get(ctx, &pods, "-n", ns, "get", "pods", "-l", "app.kubernetes.io/name="+ns, "-o", "json"); err != nil {
		return Zone{}, err
	}
	for _, pod := range pods.Items {
		if pod.Metadata.Namespace != ns || !ownedLabels(pod.Metadata.Labels, zone) {
			return Zone{}, fmt.Errorf("out-of-scope Pod discovered")
		}
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				ready = true
			}
		}
		out.Pods = append(out.Pods, Pod{Name: pod.Metadata.Name, Labels: pod.Metadata.Labels, Phase: pod.Status.Phase, Ready: ready})
	}
	sort.Slice(out.Pods, func(i, j int) bool { return out.Pods[i].Name < out.Pods[j].Name })

	var policies kubePolicyList
	if err := c.get(ctx, &policies, "-n", ns, "get", "networkpolicies", "-o", "json"); err != nil {
		return Zone{}, err
	}
	for _, p := range policies.Items {
		if p.Metadata.Namespace != ns || p.Metadata.Name == "" {
			return Zone{}, fmt.Errorf("unexpected NetworkPolicy identity")
		}
		out.NetworkPolicies = append(out.NetworkPolicies, Policy{Name: p.Metadata.Name, PodSelector: p.Spec.PodSelector.output(),
			PolicyTypes: nonNil(p.Spec.PolicyTypes), Ingress: fromRules(p.Spec.Ingress), Egress: fromEgressRules(p.Spec.Egress)})
	}
	sort.Slice(out.NetworkPolicies, func(i, j int) bool { return out.NetworkPolicies[i].Name < out.NetworkPolicies[j].Name })

	var roles kubeRoleList
	if err := c.get(ctx, &roles, "-n", ns, "get", "roles", "-o", "json"); err != nil {
		return Zone{}, err
	}
	for _, r := range roles.Items {
		if r.Metadata.Namespace != ns || r.Metadata.Name == "" {
			return Zone{}, fmt.Errorf("unexpected Role identity")
		}
		rules := make([]Rule, 0, len(r.Rules))
		for _, rule := range r.Rules {
			rules = append(rules, rule.output())
		}
		out.Roles = append(out.Roles, Role{Name: r.Metadata.Name, Rules: rules})
	}
	sort.Slice(out.Roles, func(i, j int) bool { return out.Roles[i].Name < out.Roles[j].Name })

	var bindings kubeBindingList
	if err := c.get(ctx, &bindings, "-n", ns, "get", "rolebindings", "-o", "json"); err != nil {
		return Zone{}, err
	}
	for _, b := range bindings.Items {
		if b.Metadata.Namespace != ns || b.Metadata.Name == "" {
			return Zone{}, fmt.Errorf("unexpected RoleBinding identity")
		}
		out.RoleBindings = append(out.RoleBindings, RoleBinding{Name: b.Metadata.Name, RoleRef: b.RoleRef.output(), Subjects: b.Subjects})
	}
	sort.Slice(out.RoleBindings, func(i, j int) bool { return out.RoleBindings[i].Name < out.RoleBindings[j].Name })
	return out, nil
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
func fromRules(input []kubePolicyRule) []PolicyRule       { return convertRules(input, false) }
func fromEgressRules(input []kubePolicyRule) []PolicyRule { return convertRules(input, true) }
func convertRules(input []kubePolicyRule, egress bool) []PolicyRule {
	out := make([]PolicyRule, 0, len(input))
	for _, rule := range input {
		from := rule.From
		if egress {
			from = rule.To
		}
		peers := make([]Peer, 0, len(from))
		for _, p := range from {
			peers = append(peers, p.output())
		}
		ports := make([]Port, 0, len(rule.Ports))
		for _, p := range rule.Ports {
			ports = append(ports, p.output())
		}
		out = append(out, PolicyRule{Peers: peers, Ports: ports})
	}
	return out
}
