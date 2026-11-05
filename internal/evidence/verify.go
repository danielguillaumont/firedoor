// Package evidence validates the consistency of locally captured v0.1 network matrices.
// It reads JSON evidence files only; it does not contact or change Kubernetes/Terraform.
// Internally consistent records are NOT a standalone proof of live enforcement.
package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const maxEvidenceBytes = 1024 * 1024

// Observation records one in-cluster TCP/8080 test from the v0.1 PowerShell harness.
type Observation struct {
	SourceZone        string `json:"source_zone"`
	DestinationZone   string `json:"destination_zone"`
	TargetPodIP       string `json:"target_pod_ip"`
	ExpectedConnected *bool  `json:"expected_connected"`
	Observed          string `json:"observed"`
	Attempts          int    `json:"attempts"`
}

// Record matches Test-FiredoorTerraformConnectivity.ps1's output schema.
type Record struct {
	TimestampUTC    string        `json:"timestamp_utc"`
	Cluster         string        `json:"cluster"`
	Context         string        `json:"context"`
	SecurityProfile string        `json:"security_profile"`
	Protocol        string        `json:"protocol"`
	Observations    []Observation `json:"observations"`
	Conclusion      string        `json:"conclusion"`
	Limits          string        `json:"limits"`
}

// Result confirms that two recorded matrices match v0.1 expectations.
// It does not independently verify the Kubernetes/CNI state at execution time.
type Result struct {
	SchemaVersion        string `json:"schema_version"`
	Status               string `json:"status"`
	Cluster              string `json:"cluster"`
	Context              string `json:"context"`
	BaselineTimestampUTC string `json:"baseline_timestamp_utc"`
	HardenedTimestampUTC string `json:"hardened_timestamp_utc"`
	PathsCompared        int    `json:"paths_compared"`
	BaselineConnected    int    `json:"baseline_connected"`
	HardenedConnected    int    `json:"hardened_connected"`
	HardenedDeniedTwice  int    `json:"hardened_denied_twice"`
	Limitations          string `json:"limitations"`
}

type pathSpec struct {
	from, to          string
	hardenedConnected bool
}

var paths = []pathSpec{
	{"edge", "app", true},
	{"app", "data", true},
	{"edge", "data", false},
	{"data", "app", false},
	{"app", "edge", true},
	{"data", "edge", true},
}

func key(from, to string) string { return from + "->" + to }

// Load reads a bounded UTF-8 JSON evidence document. PowerShell 5.1 emits a
// UTF-8 BOM with Set-Content -Encoding UTF8; both BOM and non-BOM files work.
func Load(path string) (Record, error) {
	var r Record
	f, err := os.Open(path)
	if err != nil {
		return r, fmt.Errorf("open evidence %q: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEvidenceBytes+1))
	if err != nil {
		return r, fmt.Errorf("read evidence %q: %w", path, err)
	}
	if len(data) > maxEvidenceBytes {
		return r, fmt.Errorf("evidence %q exceeds 1 MiB", path)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("decode evidence %q: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return r, fmt.Errorf("evidence %q contains multiple JSON values", path)
		}
		return r, fmt.Errorf("trailing data in %q: %w", path, err)
	}
	return r, nil
}

// VerifyFiles checks two locally generated reports; it does not run new probes.
func VerifyFiles(baselineFile, hardenedFile string) (Result, error) {
	var empty Result
	if baselineFile == "" || hardenedFile == "" {
		return empty, fmt.Errorf("both baseline and hardened file paths are required")
	}
	if baselineFile == hardenedFile {
		return empty, fmt.Errorf("baseline and hardened evidence paths must differ")
	}
	baseline, err := Load(baselineFile)
	if err != nil {
		return empty, err
	}
	hardened, err := Load(hardenedFile)
	if err != nil {
		return empty, err
	}
	return Verify(baseline, hardened)
}

// Verify validates declared outcomes, independent endpoint controls, record
// uniqueness, expected chronology and consistency of target pod IPs.
func Verify(baseline, hardened Record) (Result, error) {
	var empty Result
	baseTime, baseMap, err := checkRecord(baseline, "baseline")
	if err != nil {
		return empty, fmt.Errorf("baseline evidence: %w", err)
	}
	hardTime, hardMap, err := checkRecord(hardened, "hardened")
	if err != nil {
		return empty, fmt.Errorf("hardened evidence: %w", err)
	}
	if !hardTime.After(baseTime) {
		return empty, fmt.Errorf("hardened evidence timestamp must be later than baseline")
	}
	if baseline.Cluster != hardened.Cluster || baseline.Context != hardened.Context {
		return empty, fmt.Errorf("evidence records target different cluster/context")
	}
	for _, p := range paths {
		k := key(p.from, p.to)
		if baseMap[k].TargetPodIP != hardMap[k].TargetPodIP {
			return empty, fmt.Errorf("target pod IP changed between profiles for %s; repeat the controlled comparison", k)
		}
	}
	return Result{
		SchemaVersion:        "firedoor.v01.evidence-check.v1",
		Status:               "evidence_consistent",
		Cluster:              baseline.Cluster,
		Context:              baseline.Context,
		BaselineTimestampUTC: baseline.TimestampUTC,
		HardenedTimestampUTC: hardened.TimestampUTC,
		PathsCompared:        len(paths),
		BaselineConnected:    len(paths),
		HardenedConnected:    4,
		HardenedDeniedTwice:  2,
		Limitations:          "Validates saved PowerShell probe evidence, not live cluster state or packet-drop attribution; does not certify FD-001 to FD-005.",
	}, nil
}

func checkRecord(r Record, expectedProfile string) (time.Time, map[string]Observation, error) {
	var zero time.Time
	if r.Cluster != "firedoor-local" || r.Context != "kind-firedoor-local" {
		return zero, nil, fmt.Errorf("unexpected cluster/context: %q / %q", r.Cluster, r.Context)
	}
	if r.SecurityProfile != expectedProfile {
		return zero, nil, fmt.Errorf("expected profile %q, got %q", expectedProfile, r.SecurityProfile)
	}
	if r.Protocol != "HTTP over TCP/8080 direct Pod IP" {
		return zero, nil, fmt.Errorf("unexpected protocol %q", r.Protocol)
	}
	if r.Conclusion != "OBSERVED_"+expectedProfile+"_INGRESS_BEHAVIOR" {
		return zero, nil, fmt.Errorf("unexpected evidence conclusion %q", r.Conclusion)
	}
	if strings.TrimSpace(r.Limits) == "" {
		return zero, nil, fmt.Errorf("evidence limitations missing")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, r.TimestampUTC)
	if err != nil || !strings.HasSuffix(r.TimestampUTC, "Z") {
		return zero, nil, fmt.Errorf("invalid UTC timestamp %q", r.TimestampUTC)
	}
	if len(r.Observations) != len(paths) {
		return zero, nil, fmt.Errorf("expected %d observations, got %d", len(paths), len(r.Observations))
	}
	expected := make(map[string]bool, len(paths))
	for _, p := range paths {
		expected[key(p.from, p.to)] = expectedProfile == "baseline" || p.hardenedConnected
	}
	seen := make(map[string]Observation, len(paths))
	endpointIPs := make(map[string]string, 3)
	for _, o := range r.Observations {
		k := key(o.SourceZone, o.DestinationZone)
		want, exists := expected[k]
		if !exists {
			return zero, nil, fmt.Errorf("unexpected path %s", k)
		}
		if _, exists := seen[k]; exists {
			return zero, nil, fmt.Errorf("duplicate path %s", k)
		}
		if o.ExpectedConnected == nil || *o.ExpectedConnected != want {
			return zero, nil, fmt.Errorf("unexpected expected_connected flag for %s", k)
		}
		ip := net.ParseIP(o.TargetPodIP)
		if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() {
			return zero, nil, fmt.Errorf("invalid target Pod IPv4 for %s: %q", k, o.TargetPodIP)
		}
		if previous, found := endpointIPs[o.DestinationZone]; found && previous != o.TargetPodIP {
			return zero, nil, fmt.Errorf("inconsistent destination pod IP for %s", o.DestinationZone)
		}
		endpointIPs[o.DestinationZone] = o.TargetPodIP
		if want {
			if o.Observed != "CONNECTED" || o.Attempts != 1 {
				return zero, nil, fmt.Errorf("expected one successful connection for %s", k)
			}
		} else {
			if o.Observed != "UNREACHABLE_TWICE" || o.Attempts < 2 || o.Attempts > 8 {
				return zero, nil, fmt.Errorf("expected at least two denied attempts for %s", k)
			}
		}
		seen[k] = o
	}
	return timestamp, seen, nil
}
