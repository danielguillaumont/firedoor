package attribution

// Read-only Kubernetes policy admission for the fixed Firedoor owned lab.
// No apply, create, delete, patch, exec, or network probes exist in this file.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

const (
	preflightDeadline = 100 * time.Second
	commandDeadline   = 10 * time.Second
	nodeName          = "firedoor-local-control-plane"
	applicationLabel  = "app.kubernetes.io/name"
)

// CommandReader is injected for deterministic tests. ReadOnlyCommands is the
// real adapter; the preflight itself constructs every argv from fixed values.
type CommandReader interface {
	Read(ctx context.Context, binary string, args ...string) ([]byte, error)
}

type ReadOnlyCommands struct{}

func (ReadOnlyCommands) Read(ctx context.Context, binary string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	data, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("read command timeout/cancel: %w", ctx.Err())
		}
		return nil, fmt.Errorf("read-only command %s failed: %w", binary, err)
	}
	return data, nil
}

type PreflightNamespace struct {
	Name            string `json:"name"`
	PodReady        bool   `json:"pod_ready"`
	PolicyCount     int    `json:"policy_count"`
	ExpectedPolicy  string `json:"expected_policy,omitempty"`
	SourceNamespace string `json:"allowed_source_namespace,omitempty"`
}

type PreflightReport struct {
	Schema             string               `json:"schema"`
	Cluster            string               `json:"cluster"`
	Context            string               `json:"context"`
	Profile            string               `json:"terraform_profile"`
	CapturedAtUTC      time.Time            `json:"captured_at_utc"`
	Status             string               `json:"status"`
	Namespaces         []PreflightNamespace `json:"namespaces"`
	PlannedExperiments []string             `json:"planned_experiments"`
	Limitations        []string             `json:"limitations"`
}

type objectMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	UID       string            `json:"uid"`
	Labels    map[string]string `json:"labels"`
}

type namespaceObject struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type podObject struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		PodIP      string `json:"podIP"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		ContainerStatuses []struct {
			Name  string `json:"name"`
			Ready bool   `json:"ready"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type podList struct {
	Items []podObject `json:"items"`
}
type nodeList struct {
	Items []struct {
		Metadata objectMeta `json:"metadata"`
		Status   struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type labelSelector struct {
	MatchLabels      map[string]string `json:"matchLabels"`
	MatchExpressions []json.RawMessage `json:"matchExpressions"`
}
type networkPeer struct {
	NamespaceSelector *labelSelector  `json:"namespaceSelector"`
	PodSelector       *labelSelector  `json:"podSelector"`
	IPBlock           json.RawMessage `json:"ipBlock"`
}
type networkPort struct {
	Protocol string          `json:"protocol"`
	Port     json.RawMessage `json:"port"`
	EndPort  *int            `json:"endPort"`
}
type networkRule struct {
	From  []networkPeer `json:"from"`
	Ports []networkPort `json:"ports"`
}
type networkPolicy struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		PodSelector labelSelector     `json:"podSelector"`
		PolicyTypes []string          `json:"policyTypes"`
		Ingress     []networkRule     `json:"ingress"`
		Egress      []json.RawMessage `json:"egress"`
	} `json:"spec"`
}
type policyList struct {
	Items []networkPolicy `json:"items"`
}

func exactLabels(actual map[string]string, expected map[string]string) bool {
	return reflect.DeepEqual(actual, expected)
}
func oneReady(conditions []struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}) bool {
	for _, c := range conditions {
		if c.Type == "Ready" && c.Status == "True" {
			return true
		}
	}
	return false
}

func ownedNamespace(raw []byte, name string) error {
	var obj namespaceObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("namespace %s JSON: %w", name, err)
	}
	zone := strings.TrimPrefix(name, "firedoor-")
	if obj.Metadata.Name != name || obj.Status.Phase != "Active" ||
		obj.Metadata.Labels["kubernetes.io/metadata.name"] != name ||
		obj.Metadata.Labels["firedoor.dev/managed-by"] != "terraform" ||
		obj.Metadata.Labels["firedoor.dev/zone"] != zone ||
		obj.Metadata.Labels["app.kubernetes.io/part-of"] != "firedoor" {
		return fmt.Errorf("namespace %s identity, readiness or required labels differ from approved lab", name)
	}
	return nil
}

func readyOwnedPod(raw []byte, ns string) (podObject, error) {
	var pods podList
	if err := json.Unmarshal(raw, &pods); err != nil {
		return podObject{}, err
	}
	if len(pods.Items) != 1 {
		return podObject{}, fmt.Errorf("%s: expected one and only one owned Pod, got %d", ns, len(pods.Items))
	}
	p := pods.Items[0]
	ip := net.ParseIP(p.Status.PodIP)
	if p.Metadata.Namespace != ns || !strings.HasPrefix(p.Metadata.Name, ns+"-") || p.Metadata.UID == "" ||
		p.Metadata.Labels[applicationLabel] != ns || p.Metadata.Labels["firedoor.dev/managed-by"] != "terraform" ||
		p.Metadata.Labels["firedoor.dev/zone"] != strings.TrimPrefix(ns, "firedoor-") ||
		p.Metadata.Labels["app.kubernetes.io/part-of"] != "firedoor" ||
		p.Spec.NodeName != nodeName || p.Status.Phase != "Running" ||
		ip == nil || ip.To4() == nil || !ip.IsPrivate() || !oneReady(p.Status.Conditions) ||
		len(p.Status.ContainerStatuses) != 1 || !p.Status.ContainerStatuses[0].Ready || p.Status.ContainerStatuses[0].Name != "demo-http" {
		return podObject{}, fmt.Errorf("%s: Pod identity or readiness mismatch", ns)
	}
	return p, nil
}

func policyPortIs8080(raw json.RawMessage) bool {
	var asInt int
	if json.Unmarshal(raw, &asInt) == nil && asInt == 8080 {
		return true
	}
	var asString string
	return json.Unmarshal(raw, &asString) == nil && asString == "8080"
}

// ValidatePolicySnapshot rejects extra policies as well as widened/missing
// ingress rules. It never assumes a witness can override another policy.
func ValidatePolicySnapshot(raw []byte, namespace string) (PreflightNamespace, error) {
	zone := strings.TrimPrefix(namespace, "firedoor-")
	out := PreflightNamespace{Name: namespace, PodReady: true}
	if zone != "edge" && zone != "app" && zone != "data" || namespace != "firedoor-"+zone {
		return out, errors.New("foreign NetworkPolicy namespace")
	}
	var p policyList
	if err := json.Unmarshal(raw, &p); err != nil {
		return out, fmt.Errorf("policy list JSON: %w", err)
	}
	out.PolicyCount = len(p.Items)
	if zone == "edge" {
		if len(p.Items) != 0 {
			return out, errors.New("unexpected NetworkPolicy in edge namespace")
		}
		return out, nil
	}
	if len(p.Items) != 1 {
		return out, fmt.Errorf("expected exactly one managed ingress policy in %s, got %d", namespace, len(p.Items))
	}
	pol := p.Items[0]
	expectedSource := "edge"
	if zone == "data" {
		expectedSource = "app"
	}
	out.ExpectedPolicy = "firedoor-" + zone + "-ingress"
	out.SourceNamespace = "firedoor-" + expectedSource
	if pol.Metadata.Name != out.ExpectedPolicy || pol.Metadata.Namespace != namespace ||
		pol.Metadata.Labels["app.kubernetes.io/part-of"] != "firedoor" ||
		pol.Metadata.Labels["firedoor.dev/managed-by"] != "terraform" ||
		!exactLabels(pol.Spec.PodSelector.MatchLabels, map[string]string{applicationLabel: namespace}) ||
		len(pol.Spec.PodSelector.MatchExpressions) != 0 ||
		len(pol.Spec.PolicyTypes) != 1 || pol.Spec.PolicyTypes[0] != "Ingress" ||
		len(pol.Spec.Egress) != 0 || len(pol.Spec.Ingress) != 1 {
		return out, fmt.Errorf("%s: policy identity, type or selector does not match hardened Terraform design", namespace)
	}
	in := pol.Spec.Ingress[0]
	if len(in.From) != 1 || len(in.Ports) != 1 || in.From[0].NamespaceSelector == nil ||
		in.From[0].PodSelector != nil || len(in.From[0].IPBlock) != 0 ||
		len(in.From[0].NamespaceSelector.MatchExpressions) != 0 ||
		!exactLabels(in.From[0].NamespaceSelector.MatchLabels, map[string]string{"firedoor.dev/zone": expectedSource}) ||
		in.Ports[0].Protocol != "TCP" || !policyPortIs8080(in.Ports[0].Port) || in.Ports[0].EndPort != nil {
		return out, fmt.Errorf("%s: current allow rules differ from exact hardened ingress expectation", namespace)
	}
	return out, nil
}

func mustRead(ctx context.Context, r CommandReader, bin string, args ...string) ([]byte, error) {
	if r == nil {
		return nil, errors.New("nil command reader")
	}
	child, cancel := context.WithTimeout(ctx, commandDeadline)
	defer cancel()
	data, err := r.Read(child, bin, args...)
	if err != nil {
		return nil, fmt.Errorf("read-only %s inspection failed: %w", bin, err)
	}
	if len(data) == 0 || len(data) > 8*1024*1024 {
		return nil, errors.New("empty or oversized read-only inspection response")
	}
	return data, nil
}

// RunPreflight is a strict read-only admission assessment, not a traffic test.
// Every Kubernetes object is read with the pinned kind context.
func RunPreflight(parent context.Context, r CommandReader) (PreflightReport, error) {
	ctx, cancel := context.WithTimeout(parent, preflightDeadline)
	defer cancel()
	plan := ApprovedPlan()
	if err := Validate(plan); err != nil {
		return PreflightReport{}, err
	}
	current, err := mustRead(ctx, r, "kubectl", "config", "current-context")
	if err != nil {
		return PreflightReport{}, err
	}
	if strings.TrimSpace(string(current)) != OwnedContext {
		return PreflightReport{}, errors.New("active Kubernetes context is NOT the dedicated Firedoor kind context")
	}
	clusters, err := mustRead(ctx, r, "kind", "get", "clusters")
	if err != nil {
		return PreflightReport{}, err
	}
	found := false
	for _, line := range strings.Split(string(clusters), "\n") {
		if strings.TrimSpace(line) == OwnedCluster {
			found = true
		}
	}
	if !found {
		return PreflightReport{}, errors.New("owned kind cluster not found")
	}
	var nodes nodeList
	raw, err := mustRead(ctx, r, "kubectl", "--context", OwnedContext, "get", "nodes", "-o", "json")
	if err != nil {
		return PreflightReport{}, err
	}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return PreflightReport{}, err
	}
	if len(nodes.Items) != 1 || nodes.Items[0].Metadata.Name != nodeName || !oneReady(nodes.Items[0].Status.Conditions) {
		return PreflightReport{}, errors.New("unexpected or unready kind node inventory")
	}
	profile, err := mustRead(ctx, r, "terraform", "-chdir=infrastructure/terraform/testbed", "output", "-raw", "active_profile")
	if err != nil {
		return PreflightReport{}, err
	}
	if strings.TrimSpace(string(profile)) != RequiredProfile {
		return PreflightReport{}, errors.New("Terraform active_profile is not hardened")
	}
	report := PreflightReport{Schema: "firedoor.v04.policy-preflight.v1", Cluster: OwnedCluster, Context: OwnedContext, Profile: RequiredProfile, CapturedAtUTC: time.Now().UTC(), Status: "admissible_for_later_review", PlannedExperiments: []string{plan.Experiments[0].ID, plan.Experiments[1].ID}, Limitations: []string{
		"Read-only point-in-time snapshot; no witness has been created and no A/B/A traffic probes have run.",
		"Current ingress-only policy layout supports the proposed witness hypothesis; future runtime must recheck all state and safely clean up.",
		"Terraform output confirms the local state profile, not absence of independent cluster drift; policy objects are checked separately.",
		"Policy attribution remains unproven until the controlled experiment and restored-state checks complete.",
	}}
	names := []string{"firedoor-edge", "firedoor-app", "firedoor-data"}
	pods := map[string]podObject{}
	for _, ns := range names {
		raw, err := mustRead(ctx, r, "kubectl", "--context", OwnedContext, "get", "namespace", ns, "-o", "json")
		if err != nil {
			return PreflightReport{}, err
		}
		if err := ownedNamespace(raw, ns); err != nil {
			return PreflightReport{}, err
		}
		raw, err = mustRead(ctx, r, "kubectl", "--context", OwnedContext, "-n", ns, "get", "pods", "-l", applicationLabel+"="+ns, "-o", "json")
		if err != nil {
			return PreflightReport{}, err
		}
		pod, err := readyOwnedPod(raw, ns)
		if err != nil {
			return PreflightReport{}, err
		}
		pods[ns] = pod
		raw, err = mustRead(ctx, r, "kubectl", "--context", OwnedContext, "-n", ns, "get", "networkpolicies", "-o", "json")
		if err != nil {
			return PreflightReport{}, err
		}
		check, err := ValidatePolicySnapshot(raw, ns)
		if err != nil {
			return PreflightReport{}, err
		}
		report.Namespaces = append(report.Namespaces, check)
	}
	// Detect Pod replacement before accepting any point-in-time inventory.
	for _, ns := range names {
		pod := pods[ns]
		raw, err := mustRead(ctx, r, "kubectl", "--context", OwnedContext, "-n", ns, "get", "pods", "-l", applicationLabel+"="+ns, "-o", "json")
		if err != nil {
			return PreflightReport{}, err
		}
		fresh, err := readyOwnedPod(raw, ns)
		if err != nil {
			return PreflightReport{}, err
		}
		if fresh.Metadata.UID != pod.Metadata.UID || fresh.Status.PodIP != pod.Status.PodIP || fresh.Metadata.Name != pod.Metadata.Name {
			return PreflightReport{}, errors.New("Pod identity changed during preflight")
		}
	}
	return report, nil
}
