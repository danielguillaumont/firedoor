package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

// The FD-004 bridge is pure and OFFLINE. It joins Terraform output claims,
// Kubernetes configuration snapshots and the existing v0.3 probe verifier.
// No field here authenticates Terraform state, cluster identity or policy
// causality. Live provenance and Terraform lifecycle are separate v1.0 gates.
const (
	FD004BridgeSchema  = "firedoor.v10.fd004-bridge.v1"
	FD004TerraformRoot = "infrastructure/terraform/testbed"
	maxTerraformOutput = 64 << 10
)

type FD004BridgeReport struct {
	SchemaVersion          string  `json:"schema_version"`
	Scenario               string  `json:"scenario"`
	Cluster                string  `json:"cluster"`
	Context                string  `json:"context"`
	Profile                Profile `json:"profile"`
	TerraformRoot          string  `json:"terraform_root"`
	PolicyStatus           string  `json:"policy_status"`
	BehaviorStatus         string  `json:"behavior_status"`
	Status                 string  `json:"status"`
	RoutesMatched          int     `json:"routes_matched"`
	RoutesMismatch         int     `json:"routes_mismatch"`
	RoutesInconclusive     int     `json:"routes_inconclusive"`
	TerraformAuthenticated bool    `json:"terraform_authenticated"`
	Limitations            string  `json:"limitations"`
}

type terraformOutputValue struct {
	Sensitive *bool           `json:"sensitive"`
	Type      json.RawMessage `json:"type"`
	Value     json.RawMessage `json:"value"`
}

func parseTerraformFD004Outputs(b []byte, profile Profile) error {
	if len(b) == 0 || len(b) > maxTerraformOutput {
		return errors.New("Terraform output JSON is empty or exceeds 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var outputs map[string]terraformOutputValue
	if err := decoder.Decode(&outputs); err != nil {
		return fmt.Errorf("invalid Terraform output JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("trailing Terraform JSON value")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing Terraform JSON: %w", err)
	}
	wanted := []string{"active_profile", "owned_namespaces", "services", "managed_ingress_policies"}
	if len(outputs) != len(wanted) {
		return errors.New("expected precisely four Terraform testbed outputs")
	}
	for _, name := range wanted {
		item, ok := outputs[name]
		if !ok || item.Sensitive == nil || *item.Sensitive || len(item.Type) == 0 || bytes.Equal(bytes.TrimSpace(item.Type), []byte("null")) || len(item.Value) == 0 {
			return fmt.Errorf("missing, sensitive, or untyped Terraform output %q", name)
		}
	}
	var actualProfile string
	if err := json.Unmarshal(outputs["active_profile"].Value, &actualProfile); err != nil || actualProfile != string(profile) {
		return errors.New("Terraform active_profile does not match requested plan")
	}
	if string(bytes.TrimSpace(outputs["active_profile"].Type)) != `"string"` {
		return errors.New("Terraform active_profile has an unexpected type")
	}
	for key, expected := range map[string]map[string]string{
		"owned_namespaces":         {"edge": "firedoor-edge", "app": "firedoor-app", "data": "firedoor-data"},
		"services":                 {"edge": "firedoor-edge.firedoor-edge.svc.cluster.local:8080", "app": "firedoor-app.firedoor-app.svc.cluster.local:8080", "data": "firedoor-data.firedoor-data.svc.cluster.local:8080"},
		"managed_ingress_policies": {"app": "firedoor-app-ingress", "data": "firedoor-data-ingress"},
	} {
		var value map[string]string
		if err := json.Unmarshal(outputs[key].Value, &value); err != nil || !reflect.DeepEqual(value, expected) {
			return fmt.Errorf("Terraform output %q differs from fixed owned testbed", key)
		}
		var tokens []json.RawMessage
		if err := json.Unmarshal(outputs[key].Type, &tokens); err != nil || len(tokens) != 2 || string(bytes.TrimSpace(tokens[0])) != `"object"` {
			return fmt.Errorf("Terraform output %q must describe an object type", key)
		}
		var fieldTypes map[string]string
		if err := json.Unmarshal(tokens[1], &fieldTypes); err != nil || len(fieldTypes) != len(expected) {
			return fmt.Errorf("Terraform output %q has an invalid object type", key)
		}
		for k := range expected {
			if fieldTypes[k] != "string" {
				return fmt.Errorf("Terraform output %q has an unexpected object field type", key)
			}
		}
	}
	return nil
}

func sameLabels(actual, wanted map[string]string) bool {
	if len(actual) == 0 && len(wanted) == 0 {
		return true
	}
	return reflect.DeepEqual(actual, wanted)
}

func isKnownFD004Policy(p kubeinspect.Policy, zone string, profile Profile) bool {
	if p.Name != "firedoor-"+zone+"-ingress" ||
		len(p.PolicyTypes) != 1 || p.PolicyTypes[0] != "Ingress" ||
		len(p.Egress) != 0 || len(p.Ingress) != 1 ||
		len(p.PodSelector.MatchExpressions) != 0 ||
		!sameLabels(p.PodSelector.MatchLabels, map[string]string{"app.kubernetes.io/name": "firedoor-" + zone}) {
		return false
	}
	rule := p.Ingress[0]
	if len(rule.Peers) != 1 || len(rule.Ports) != 1 {
		return false
	}
	peer := rule.Peers[0]
	if peer.NamespaceSelector == nil || peer.PodSelector != nil || peer.IPBlock != nil || len(peer.NamespaceSelector.MatchExpressions) != 0 {
		return false
	}
	expected := map[string]string{}
	if profile == ProfileHardened {
		source := "edge"
		if zone == "data" {
			source = "app"
		}
		expected = map[string]string{"firedoor.dev/zone": source}
	}
	if !sameLabels(peer.NamespaceSelector.MatchLabels, expected) {
		return false
	}
	port := rule.Ports[0]
	if port.EndPort != nil || port.Protocol == nil || *port.Protocol != "TCP" {
		return false
	}
	switch v := port.Port.(type) {
	case string:
		return v == "8080"
	case int:
		return v == 8080
	case float64:
		return v == 8080
	case json.Number:
		return v.String() == "8080"
	default:
		return false
	}
}

// fd004PolicyStatus checks exactly two owned policy snapshots. A missing or
// changed policy is drift, not a successful containment outcome. This is an
// offline *specification* comparison; other namespace policies and runtime CNI
// behavior require the next live gate.
func fd004PolicyStatus(snapshot kubeinspect.Snapshot, profile Profile) (string, error) {
	if snapshot.SchemaVersion != kubeinspect.Schema || snapshot.Cluster != kubeinspect.Cluster ||
		snapshot.Context != kubeinspect.Context || snapshot.CapturedAtUTC.IsZero() ||
		strings.TrimSpace(snapshot.Limitations) == "" || len(snapshot.Zones) != 3 {
		return "", errors.New("untrusted or incomplete Kubernetes snapshot metadata")
	}
	seen := map[string]bool{}
	status := "consistent"
	for _, z := range snapshot.Zones {
		if z.Name != "edge" && z.Name != "app" && z.Name != "data" {
			return "", fmt.Errorf("unowned zone %q in Kubernetes snapshot", z.Name)
		}
		if seen[z.Name] || z.Namespace != "firedoor-"+z.Name {
			return "", errors.New("duplicate or mislabeled Kubernetes zone")
		}
		seen[z.Name] = true
		if z.Name == "edge" {
			if len(z.NetworkPolicies) != 0 {
				status = "drift"
			}
			continue
		}
		if len(z.NetworkPolicies) != 1 || !isKnownFD004Policy(z.NetworkPolicies[0], z.Name, profile) {
			status = "drift"
		}
	}
	if len(seen) != 3 {
		return "", errors.New("missing Kubernetes testbed zone")
	}
	return status, nil
}

// AssembleFD004Bridge uses the existing liveverify Verify function and does
// NOT rerun or reinterpret HTTP probes. In particular, "consistent" is only
// an internal comparison of separately supplied artifacts, NOT proof that the
// Terraform outputs came from real state or that Calico caused a denial.
func AssembleFD004Bridge(plan Plan, terraformOutputJSON []byte, snapshot kubeinspect.Snapshot, suite SuiteObservation) (FD004BridgeReport, error) {
	if err := plan.Validate(); err != nil {
		return FD004BridgeReport{}, err
	}
	if err := parseTerraformFD004Outputs(terraformOutputJSON, plan.Profile); err != nil {
		return FD004BridgeReport{}, err
	}
	if snapshot.CapturedAtUTC.Before(suite.StartedAtUTC.Add(-2*time.Minute)) ||
		snapshot.CapturedAtUTC.After(suite.FinishedAtUTC.Add(2*time.Minute)) {
		return FD004BridgeReport{}, errors.New("Kubernetes snapshot not contemporaneous with the observation suite")
	}
	policyStatus, err := fd004PolicyStatus(snapshot, plan.Profile)
	if err != nil {
		return FD004BridgeReport{}, err
	}
	verified, err := Verify(plan, suite)
	if err != nil {
		return FD004BridgeReport{}, fmt.Errorf("invalid underlying network evidence: %w", err)
	}
	result := FD004BridgeReport{
		SchemaVersion: FD004BridgeSchema, Scenario: "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		Profile: plan.Profile, TerraformRoot: FD004TerraformRoot,
		PolicyStatus: policyStatus, BehaviorStatus: verified.Status,
		Status: "consistent", RoutesMatched: verified.Summary.Match,
		RoutesMismatch: verified.Summary.Mismatch, RoutesInconclusive: verified.Summary.Inconclusive,
		TerraformAuthenticated: false,
		Limitations:            "OFFLINE consistency assessment only. Terraform output JSON is an unauthenticated claim, Kubernetes inventory and saved probe JSON are not attested, and matching behavior does not establish NetworkPolicy causality. The guarded live Terraform state/plan and resource ownership checks are not yet connected.",
	}
	if verified.Status == "inconclusive" {
		result.Status = "inconclusive"
	}
	if verified.Status == "mismatched" {
		result.Status = "mismatch"
	}
	if policyStatus == "drift" {
		result.Status = "drift"
	}
	return result, nil
}
