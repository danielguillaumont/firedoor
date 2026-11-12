// Package recoverytrafficproof evaluates network traffic observations from an
// already-deployed, independently audited, disposable Firedoor KIND cluster.
// It does not execute Kubernetes or Terraform operations.
package recoverytrafficproof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
)

const Cluster = "firedoor-rh-3c138b60a075"
const ApplyResultSHA = "1E390B0301308C9C646AFA10896E5282E185C1ADA8914559C7A25CAF4A37BD12"

const Schema = "firedoor.v10.fd004-traffic-observation.v1"

// Probe is a network request executed *inside an existing Firedoor pod*.
// The URL uses a verified ClusterIP:8080 (not DNS), to isolate network routing.
type Probe struct {
	ID                      string `json:"id"`
	SourceZone              string `json:"source_zone"`
	DestinationZone         string `json:"destination_zone"`
	Expected                string `json:"expected"`
	ServiceIP               string `json:"service_ip"`
	SourcePodReady          bool   `json:"source_pod_ready"`
	DestinationServiceReady bool   `json:"destination_service_ready"`
	TransportPrecheckOK     bool   `json:"transport_precheck_ok"`
	KubectlExitCode         int    `json:"kubectl_exit_code"`
	WgetExitCode            int    `json:"wget_exit_code"`
	ResponseBody            string `json:"response_body"`
}

type Observation struct {
	SchemaVersion              string  `json:"schema_version"`
	Cluster                    string  `json:"cluster"`
	Context                    string  `json:"context"`
	F4AuditPassed              bool    `json:"f4_audit_passed"`
	ApplyResultSHA256          string  `json:"apply_result_sha256"`
	TerraformStateHashVerified bool    `json:"terraform_state_hash_verified"`
	IngressPolicyCount         int     `json:"ingress_policy_count"`
	NoKubernetesObjectWrites   bool    `json:"no_kubernetes_object_writes"`
	NoTerraformExecution       bool    `json:"no_terraform_execution"`
	Probes                     []Probe `json:"probes"`
}

type Result struct {
	SchemaVersion                      string `json:"schema_version"`
	Status                             string `json:"status"`
	Cluster                            string `json:"cluster"`
	AllowedVerified                    int    `json:"allowed_verified"`
	DeniedVerified                     int    `json:"denied_verified"`
	NetworkTrafficEnforcementVerified  bool   `json:"network_traffic_enforcement_verified"`
	KubernetesObjectsChangedByVerifier bool   `json:"kubernetes_objects_changed_by_verifier"`
	TerraformExecutedByVerifier        bool   `json:"terraform_executed_by_verifier"`
	RecoveryProven                     bool   `json:"recovery_proven"`
}

type expectedCase struct{ source, dest, expectation, body string }

var cases = map[string]expectedCase{
	"edge-to-app":          {"edge", "app", "allow", "FIREDOOR_APP_OK"},
	"app-to-data":          {"app", "data", "allow", "FIREDOOR_DATA_OK"},
	"data-to-edge-control": {"data", "edge", "allow", "FIREDOOR_EDGE_OK"},
	"edge-to-data":         {"edge", "data", "deny", ""},
	"data-to-app":          {"data", "app", "deny", ""},
}

func Evaluate(o Observation) (Result, error) {
	reject := func(s string) (Result, error) { return Result{}, fmt.Errorf("F5 traffic proof rejected: %s", s) }
	if o.SchemaVersion != Schema || o.Cluster != Cluster || o.Context != "kind-"+Cluster ||
		!strings.EqualFold(o.ApplyResultSHA256, ApplyResultSHA) {
		return reject("unexpected schema, cluster, context, or apply evidence")
	}
	if !o.F4AuditPassed || !o.TerraformStateHashVerified || o.IngressPolicyCount != 2 ||
		!o.NoKubernetesObjectWrites || !o.NoTerraformExecution {
		return reject("missing independent deployment audit or read-only guarantee")
	}
	if len(o.Probes) != len(cases) {
		return reject("expected exactly five probe observations")
	}
	seen := make(map[string]struct{}, len(cases))
	allowed, denied := 0, 0
	ips := map[string]string{}
	for _, p := range o.Probes {
		want, ok := cases[p.ID]
		if !ok {
			return reject("unexpected probe ID")
		}
		if _, ok := seen[p.ID]; ok {
			return reject("duplicate probe ID")
		}
		seen[p.ID] = struct{}{}
		if p.SourceZone != want.source || p.DestinationZone != want.dest || p.Expected != want.expectation ||
			!p.SourcePodReady || !p.DestinationServiceReady || !p.TransportPrecheckOK {
			return reject("probe identity, endpoint readiness, or exec failure: " + p.ID)
		}
		ip := net.ParseIP(p.ServiceIP)
		if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			return reject("invalid service ClusterIP in " + p.ID)
		}
		if earlier, ok := ips[p.DestinationZone]; ok && earlier != p.ServiceIP {
			return reject("destination service IP changed between probes")
		}
		ips[p.DestinationZone] = p.ServiceIP
		switch p.Expected {
		case "allow":
			if p.KubectlExitCode != 0 || p.WgetExitCode != 0 || strings.TrimSpace(p.ResponseBody) != want.body {
				return reject("permitted traffic did not return the expected service marker: " + p.ID)
			}
			allowed++
		case "deny":
			// Direct `kubectl exec -- /bin/busybox wget` forwards remote wget
			// exit 1 as kubectl exit 1 for a bounded connection failure.
			// A successful direct BusyBox precheck, healthy destination, and
			// successful HTTP control to this same Service are also required.
			// Exit 127 (missing binary) and other exits cannot count as deny.
			if p.KubectlExitCode != 1 || p.WgetExitCode != 1 || strings.TrimSpace(p.ResponseBody) != "" {
				return reject("denied traffic did not fail cleanly: " + p.ID)
			}
			denied++
		default:
			return reject("invalid expected outcome")
		}
	}
	// For each denied destination, a different permitted source successfully
	// reached that *same* ClusterIP. This helps rule out an unavailable Service.
	if allowed != 3 || denied != 2 || len(ips) != 3 {
		return reject("incomplete allowed, denied, or control observations")
	}
	return Result{SchemaVersion: "firedoor.v10.fd004-traffic-proof-result.v1", Status: "five_live_network_probes_verified", Cluster: Cluster, AllowedVerified: allowed, DeniedVerified: denied, NetworkTrafficEnforcementVerified: true, KubernetesObjectsChangedByVerifier: false, TerraformExecutedByVerifier: false, RecoveryProven: false}, nil
}

func Decode(r io.Reader) (Observation, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1<<20+1))
	if err != nil {
		return Observation{}, err
	}
	if len(data) > 1<<20 {
		return Observation{}, fmt.Errorf("observation exceeds one MiB")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}) // One Windows PowerShell 5.1 UTF-8 BOM is acceptable.
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var o Observation
	if err := decoder.Decode(&o); err != nil {
		return Observation{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Observation{}, fmt.Errorf("trailing or malformed JSON: %v", err)
	}
	return o, nil
}
