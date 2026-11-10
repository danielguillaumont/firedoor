// Live boundary evaluation is separate from the v0.9A synthetic contract.
// Inputs originate from the guarded disposable-kind PowerShell driver; the
// source field is not an independently authenticated provenance signature.
package regression

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const LiveSchemaVersion = "firedoor.v09.regression.live.v1"
const MaxLiveObservationBytes = 8192

var ephemeralClusterPattern = regexp.MustCompile(`^firedoor-v09-[0-9a-f]{12}$`)
var kubernetesUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// LiveObservation records ONE specifically verified destination in ONE phase.
// Required health signals prevent ordinary endpoint outages being called a
// policy denial; an inconclusive run can never count as a passing gate.
type LiveObservation struct {
	SchemaVersion   string `json:"schema_version"`
	Source          string `json:"source"`
	RunID           string `json:"run_id"`
	Cluster         string `json:"cluster"`
	Context         string `json:"context"`
	Namespace       string `json:"namespace"`
	PolicyName      string `json:"policy_name"`
	PolicyUID       string `json:"policy_uid"`
	Phase           string `json:"phase"`
	PolicyMode      string `json:"policy_mode"`
	RouteID         string `json:"route_id"`
	Target          string `json:"target"`
	Transport       string `json:"transport"`
	Port            int    `json:"port"`
	Observed        string `json:"observed"`
	TrustedHealthy  bool   `json:"trusted_healthy"`
	EdgeExecHealthy bool   `json:"edge_exec_healthy"`
	CNIReady        bool   `json:"cni_ready"`
	PolicyVerified  bool   `json:"policy_verified"`
}

type LiveReport struct {
	SchemaVersion string   `json:"schema_version"`
	Source        string   `json:"source"`
	RunID         string   `json:"run_id"`
	Cluster       string   `json:"cluster"`
	PolicyUID     string   `json:"policy_uid"`
	Phase         string   `json:"phase"`
	RouteID       string   `json:"route_id"`
	Target        string   `json:"target"`
	Expected      string   `json:"expected"`
	Observed      string   `json:"observed"`
	Verdict       string   `json:"verdict"`
	Limitations   []string `json:"limitations"`
}

var liveKeys = map[string]bool{
	"schema_version": true, "source": true, "run_id": true, "cluster": true,
	"context": true, "namespace": true, "policy_name": true, "policy_uid": true,
	"phase": true, "policy_mode": true, "route_id": true, "target": true,
	"transport": true, "port": true, "observed": true,
	"trusted_healthy": true, "edge_exec_healthy": true,
	"cni_ready": true, "policy_verified": true,
}

// DecodeLive enforces a flat exact-key envelope, bounded bytes, no duplicates,
// no nulls, no trailing values, and strict Go field types.
func DecodeLive(input []byte) (LiveObservation, error) {
	var o LiveObservation
	if len(input) == 0 || len(input) > MaxLiveObservationBytes {
		return o, errors.New("live observation missing or oversized")
	}
	d := json.NewDecoder(bytes.NewReader(input))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return o, errors.New("live observation must be one JSON object")
	}
	seen := make(map[string]json.RawMessage, len(liveKeys))
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return o, fmt.Errorf("live key: %w", err)
		}
		key, ok := k.(string)
		if !ok || !liveKeys[key] {
			return o, fmt.Errorf("unknown live field %q", key)
		}
		if _, exists := seen[key]; exists {
			return o, fmt.Errorf("duplicate live field %q", key)
		}
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return o, fmt.Errorf("live field %q: %w", key, err)
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return o, fmt.Errorf("null live field %q", key)
		}
		seen[key] = v
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return o, errors.New("live observation JSON object unclosed")
	}
	if _, err = d.Token(); err != io.EOF {
		return o, errors.New("live observation has trailing or malformed JSON")
	}
	if len(seen) != len(liveKeys) {
		return o, errors.New("live observation missing fields")
	}
	b, err := json.Marshal(seen)
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return o, fmt.Errorf("live field type: %w", err)
	}
	if err := o.Validate(); err != nil {
		return LiveObservation{}, err
	}
	return o, nil
}

func (o LiveObservation) Validate() error {
	if o.SchemaVersion != LiveSchemaVersion || o.Source != "guarded_kind_calico_probe" {
		return errors.New("live observation source/schema mismatch")
	}
	if !ephemeralClusterPattern.MatchString(o.Cluster) || o.RunID != o.Cluster || o.Context != "kind-"+o.Cluster {
		return errors.New("live observation not bound to disposable kind context")
	}
	if o.Namespace != "firedoor-v09c1" || o.PolicyName != "allow-trusted-to-data" || !kubernetesUIDPattern.MatchString(o.PolicyUID) {
		return errors.New("live policy identity mismatch")
	}
	if o.RouteID != "edge-to-data" || o.Transport != "HTTP/TCP" || o.Port != 8080 {
		return errors.New("unexpected live network route")
	}
	if o.Target != "service" && o.Target != "pod_ip" {
		return errors.New("unexpected live route target")
	}
	switch o.Phase {
	case "protected", "restored":
		if o.PolicyMode != "trusted_only" {
			return errors.New("protected phase has unexpected policy mode")
		}
	case "weakened":
		if o.PolicyMode != "all_namespace_pods" {
			return errors.New("weakened phase has unexpected policy mode")
		}
	default:
		return errors.New("unsupported live observation phase")
	}
	switch o.Observed {
	case "reachable", "unreachable", "not_observed":
	default:
		return errors.New("unsupported live reachability observation")
	}
	return nil
}

func EvaluateLive(o LiveObservation) (LiveReport, error) {
	if err := o.Validate(); err != nil {
		return LiveReport{}, err
	}
	verdict := "INCONCLUSIVE"
	if o.TrustedHealthy && o.EdgeExecHealthy && o.CNIReady && o.PolicyVerified {
		if o.Observed == "reachable" {
			verdict = "REGRESSION"
		}
		if o.Observed == "unreachable" && o.Phase != "weakened" {
			verdict = "MATCH"
		}
		// Weakened but still blocked is not proof of a secure protected policy;
		// report INCONCLUSIVE until causality and effective policy are reviewed.
	}
	return LiveReport{
		SchemaVersion: LiveSchemaVersion, Source: o.Source, RunID: o.RunID, Cluster: o.Cluster,
		PolicyUID: o.PolicyUID, Phase: o.Phase, RouteID: o.RouteID, Target: o.Target,
		Expected: "unreachable", Observed: o.Observed, Verdict: verdict,
		Limitations: []string{
			"Guarded driver observations; source value is not a cryptographic attestation.",
			"Local disposable kind/Calico proof only; not production or Terraform drift evidence.",
			"MATCH requires trusted-health, edge-process, CNI, and API policy controls; no timeout alone proves a deny.",
		},
	}, nil
}
