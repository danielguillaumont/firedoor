package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

// This is an OFFLINE consistency contract for a FUTURE disposable testbed.
// It does not authenticate KIND, Docker, Kubernetes or Terraform. The CLI never
// executes external commands. A later independent live runner must collect and
// reauthenticate every claimed identity before using it for probe execution.
const FD004DisposableReadinessSchema = "firedoor.v10.fd004-disposable-readiness-input.v1"
const FD004DisposableReadinessReportSchema = "firedoor.v10.fd004-disposable-readiness-review.v1"
const FD004MaxDisposableReadinessJSON = 256 << 10

var fd004DisposableName = regexp.MustCompile(`^firedoor-rh-[a-f0-9]{12}$`)
var fd004DockerID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var fd004KubeUID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{7,79}$`)
var fd004HexSHA = regexp.MustCompile(`^[a-f0-9]{64}$`)

const fd004DeletedHistoricalCluster = "firedoor-rh-0ffb4d40a3c2"

type FD004DisposableNode struct {
	Name             string `json:"name"`
	DockerID         string `json:"docker_id"`
	KindClusterLabel string `json:"kind_cluster_label"`
	Running          bool   `json:"running"`
	APIHostPort      int    `json:"api_host_port"`
}

type FD004DisposablePod struct {
	Zone      string            `json:"zone"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	UID       string            `json:"uid"`
	IP        string            `json:"ip"`
	Ready     bool              `json:"ready"`
	Labels    map[string]string `json:"labels"`
}

type FD004DisposablePolicy struct {
	Zone          string             `json:"zone"`
	Namespace     string             `json:"namespace"`
	UID           string             `json:"uid"`
	ManagedBy     string             `json:"managed_by"`
	Configuration kubeinspect.Policy `json:"configuration"`
}

type FD004DisposableReadinessInput struct {
	SchemaVersion            string                  `json:"schema_version"`
	Scenario                 string                  `json:"scenario"`
	Cluster                  string                  `json:"cluster"`
	Context                  string                  `json:"context"`
	CapturedAtUTC            time.Time               `json:"captured_at_utc"`
	KindClusters             []string                `json:"kind_clusters"`
	DockerNodes              []FD004DisposableNode   `json:"docker_nodes"`
	KubeconfigCurrentContext string                  `json:"kubeconfig_current_context"`
	KubeconfigAPIServer      string                  `json:"kubeconfig_api_server"`
	KubernetesNodes          []string                `json:"kubernetes_nodes"`
	TerraformStateAddresses  []string                `json:"terraform_state_addresses"`
	TerraformStateSHA256     string                  `json:"terraform_state_sha256"`
	TerraformProfile         string                  `json:"terraform_profile"`
	TerraformKubeContext     string                  `json:"terraform_kube_context"`
	Pods                     []FD004DisposablePod    `json:"pods"`
	Policies                 []FD004DisposablePolicy `json:"policies"`
	ApplyAuthorized          bool                    `json:"apply_authorized"`
	RecoveryTested           bool                    `json:"recovery_tested"`
	TerraformAuthenticated   bool                    `json:"terraform_authenticated"`
	PolicyCausalityProven    bool                    `json:"policy_causality_proven"`
}

type FD004DisposableReadinessReport struct {
	SchemaVersion          string `json:"schema_version"`
	Scenario               string `json:"scenario"`
	Cluster                string `json:"cluster"`
	Context                string `json:"context"`
	Profile                string `json:"profile"`
	NodeCount              int    `json:"node_count"`
	PodCount               int    `json:"pod_count"`
	HardenedPolicyCount    int    `json:"hardened_policy_count"`
	StateResourceCount     int    `json:"state_resource_count"`
	ProbeRouteCount        int    `json:"probe_route_count"`
	Status                 string `json:"status"`
	ApplyAuthorized        bool   `json:"apply_authorized"`
	RecoveryTested         bool   `json:"recovery_tested"`
	TerraformAuthenticated bool   `json:"terraform_authenticated"`
	PolicyCausalityProven  bool   `json:"policy_causality_proven"`
	Limitations            string `json:"limitations"`
}

// ReviewFD004DisposableReadinessJSON fails on duplicate keys, unknown fields,
// bad/partial identities, unexpected policies and stale snapshot claims. The
// separate confirmedCluster is an explicit operator selection; it MUST NOT be
// populated from the submitted JSON by a caller.
func ReviewFD004DisposableReadinessJSON(raw []byte, confirmedCluster string, now time.Time) (FD004DisposableReadinessReport, error) {
	var none FD004DisposableReadinessReport
	if len(raw) == 0 || len(raw) > FD004MaxDisposableReadinessJSON || bytes.IndexByte(raw, 0) >= 0 {
		return none, errors.New("readiness JSON empty, oversized, or contains NUL")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := fd004ReadJSONValue(d) // existing duplicate-key-resistant JSON parser
	if err != nil {
		return none, fmt.Errorf("readiness JSON rejected: %w", err)
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return none, errors.New("readiness root must be an object")
	}
	// A missing false-valued security flag must not be accepted as an explicit
	// negative assertion. Require every top-level field, including all four
	// non-authoritative security flags, to appear exactly once.
	required := []string{
		"schema_version", "scenario", "cluster", "context", "captured_at_utc",
		"kind_clusters", "docker_nodes", "kubeconfig_current_context",
		"kubeconfig_api_server", "kubernetes_nodes", "terraform_state_addresses",
		"terraform_state_sha256", "terraform_profile", "terraform_kube_context",
		"pods", "policies", "apply_authorized", "recovery_tested",
		"terraform_authenticated", "policy_causality_proven",
	}
	if len(fields) != len(required) {
		return none, errors.New("unexpected or missing readiness fields")
	}
	for _, name := range required {
		if _, exists := fields[name]; !exists {
			return none, fmt.Errorf("missing required readiness field %s", name)
		}
	}
	if _, err := d.Token(); err != io.EOF {
		return none, errors.New("trailing or malformed readiness JSON")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return none, err
	}
	strict := json.NewDecoder(bytes.NewReader(normalized))
	strict.DisallowUnknownFields()
	var input FD004DisposableReadinessInput
	if err := strict.Decode(&input); err != nil {
		return none, fmt.Errorf("readiness shape rejected: %w", err)
	}
	return ReviewFD004DisposableReadiness(input, confirmedCluster, now)
}

func ReviewFD004DisposableReadiness(in FD004DisposableReadinessInput, confirmedCluster string, now time.Time) (FD004DisposableReadinessReport, error) {
	var none FD004DisposableReadinessReport
	if in.SchemaVersion != FD004DisposableReadinessSchema || in.Scenario != "FD-004" ||
		!fd004DisposableName.MatchString(in.Cluster) || in.Cluster == fd004DeletedHistoricalCluster ||
		confirmedCluster == "" || confirmedCluster != in.Cluster || in.Context != "kind-"+in.Cluster ||
		in.KubeconfigCurrentContext != in.Context || in.TerraformKubeContext != in.Context ||
		in.TerraformProfile != "hardened" || in.ApplyAuthorized || in.RecoveryTested ||
		in.TerraformAuthenticated || in.PolicyCausalityProven {
		return none, errors.New("disposable identity, operator selection or unsupported security claim")
	}
	now = now.UTC()
	when := in.CapturedAtUTC.UTC()
	if now.IsZero() || when.IsZero() || when.Before(now.Add(-10*time.Minute)) || when.After(now.Add(30*time.Second)) {
		return none, errors.New("readiness inventory is stale or future-dated")
	}
	kindSeen := map[string]bool{}
	for _, name := range in.KindClusters {
		if name == "" || name == fd004DeletedHistoricalCluster || kindSeen[name] || len(in.KindClusters) > 32 {
			return none, errors.New("invalid KIND cluster inventory")
		}
		kindSeen[name] = true
	}
	if !kindSeen[in.Cluster] || !kindSeen[OwnedCluster] {
		return none, errors.New("disposable and protected KIND cluster inventories must both be present")
	}

	control, worker := in.Cluster+"-control-plane", in.Cluster+"-worker"
	if len(in.DockerNodes) != 2 || len(in.KubernetesNodes) != 2 {
		return none, errors.New("expected exactly two disposable nodes")
	}
	expected := map[string]bool{control: true, worker: true}
	dockerNames, dockerIDs := map[string]bool{}, map[string]bool{}
	apiPort := 0
	for _, node := range in.DockerNodes {
		if !expected[node.Name] || dockerNames[node.Name] || !fd004DockerID.MatchString(node.DockerID) || dockerIDs[node.DockerID] ||
			node.KindClusterLabel != in.Cluster || !node.Running {
			return none, errors.New("Docker node identity, ownership or running state rejected")
		}
		dockerNames[node.Name], dockerIDs[node.DockerID] = true, true
		if node.Name == control {
			if node.APIHostPort < 1024 || node.APIHostPort > 65535 {
				return none, errors.New("invalid control-plane API host port")
			}
			apiPort = node.APIHostPort
		} else if node.APIHostPort != 0 {
			return none, errors.New("worker must not advertise an API host port")
		}
	}
	kubeNames := map[string]bool{}
	for _, node := range in.KubernetesNodes {
		if !expected[node] || kubeNames[node] {
			return none, errors.New("unexpected Kubernetes node")
		}
		kubeNames[node] = true
	}
	if in.KubeconfigAPIServer != fmt.Sprintf("https://127.0.0.1:%d", apiPort) {
		return none, errors.New("Kubeconfig API endpoint differs from owned Docker control-plane")
	}

	addresses, err := parseFD004StateAddresses([]byte(strings.Join(in.TerraformStateAddresses, "\n")))
	if err != nil || len(addresses) != 14 || !fd004HexSHA.MatchString(in.TerraformStateSHA256) {
		return none, errors.New("Terraform state address inventory or state digest rejected")
	}
	if len(in.Pods) != 3 || len(in.Policies) != 2 {
		return none, errors.New("expected three workloads and two hardened policies")
	}
	seenPods, seenIP, seenUID := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, pod := range in.Pods {
		if pod.Zone != "edge" && pod.Zone != "app" && pod.Zone != "data" {
			return none, errors.New("unknown Pod zone")
		}
		expectedName := "firedoor-" + pod.Zone
		ip := net.ParseIP(pod.IP)
		if seenPods[pod.Zone] || seenIP[pod.IP] || seenUID[pod.UID] || !pod.Ready || pod.Namespace != expectedName ||
			!strings.HasPrefix(pod.Name, expectedName+"-") || len(pod.Name) > 253 || !fd004KubeUID.MatchString(pod.UID) ||
			ip == nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() ||
			pod.Labels["app.kubernetes.io/name"] != expectedName || pod.Labels["firedoor.dev/managed-by"] != "terraform" {
			return none, fmt.Errorf("owned Pod evidence rejected in %s", pod.Zone)
		}
		seenPods[pod.Zone], seenIP[pod.IP], seenUID[pod.UID] = true, true, true
	}
	seenPolicies := map[string]bool{}
	for _, policy := range in.Policies {
		if policy.Zone != "app" && policy.Zone != "data" {
			return none, errors.New("unknown hardened policy zone")
		}
		if seenPolicies[policy.Zone] || policy.Namespace != "firedoor-"+policy.Zone ||
			!fd004KubeUID.MatchString(policy.UID) || seenUID[policy.UID] || policy.ManagedBy != "terraform" ||
			!isKnownFD004Policy(policy.Configuration, policy.Zone, ProfileHardened) {
			return none, fmt.Errorf("hardened policy identity or selector rejected: %s", policy.Zone)
		}
		seenPolicies[policy.Zone] = true
		seenUID[policy.UID] = true
	}
	// The source-side six-route plan is used only as a non-executable expected
	// matrix. It remains scoped to firedoor-local and MUST NOT be run against
	// the disposable cluster or massaged into a false live proof.
	probe, err := NewPlan(ProfileHardened)
	if err != nil {
		return none, err
	}
	ids := make([]string, 0, len(probe.Probes))
	for _, route := range probe.Probes {
		ids = append(ids, route.ID)
	}
	sort.Strings(ids)
	if len(ids) != 6 {
		return none, errors.New("invalid six-route contract")
	}
	return FD004DisposableReadinessReport{
		SchemaVersion: FD004DisposableReadinessReportSchema, Scenario: "FD-004", Cluster: in.Cluster, Context: in.Context,
		Profile: "hardened", NodeCount: 2, PodCount: 3, HardenedPolicyCount: 2, StateResourceCount: 14,
		ProbeRouteCount: 6, Status: "offline_identity_consistent_live_authentication_pending",
		ApplyAuthorized: false, RecoveryTested: false, TerraformAuthenticated: false, PolicyCausalityProven: false,
		Limitations: "UNAUTHENTICATED OFFLINE INVENTORY. Matching supplied JSON values, Docker IDs, loopback port, Terraform address names and policy selectors does NOT establish their origin, state validity, CNI enforcement, HTTP behavior, or approval to change resources. A separately reviewed READ-ONLY collector must independently inspect the real disposable testbed, bind the six-route probes to its exact Pod UIDs/IPs, and reauthenticate the scope before any live step. Never run the firedoor-local-pinned probe executor on this reported scope.",
	}, nil
}
