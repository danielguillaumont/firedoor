// Package regression also provides a separate v0.9E policy-spec drift gate.
// This is an OFFLINE interpretation of a claimed Kubernetes API snapshot:
// it never queries Kubernetes and a source string is NOT an attestation.
package regression

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
)

const DriftSchemaVersion = "firedoor.v09.drift.snapshot.v1"
const DriftMaxSnapshotBytes = 16384
const driftSource = "guarded_kind_calico_api"

// The approved security policy is held in code, not supplied by the snapshot.
// This mirrors infrastructure/kind/v09c2-trusted-ingress-policy.yaml.
// This narrowly scoped lab expectation does not inspect other namespaces,
// cluster-scoped Calico policies, egress restrictions, or Terraform state.
const approvedIngressSpec = `{
    "podSelector":{"matchLabels":{"app":"data-server"}},
    "policyTypes":["Ingress"],
    "ingress":[{"from":[{"podSelector":{"matchLabels":{"role":"trusted"}}}],
                "ports":[{"protocol":"TCP","port":8080}]}]
}`

var driftResourceVersion = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// DriftSnapshot describes one observation by a future *guarded* Kubernetes
// API collector. "not_found" means confirmed 404; "unavailable" means the
// API outcome was not established and must NEVER be interpreted as absence.
type DriftSnapshot struct {
	SchemaVersion    string          `json:"schema_version"`
	Source           string          `json:"source"`
	RunID            string          `json:"run_id"`
	Cluster          string          `json:"cluster"`
	Context          string          `json:"context"`
	Namespace        string          `json:"namespace"`
	PolicyName       string          `json:"policy_name"`
	ObservationState string          `json:"observation_state"`
	PolicyUID        string          `json:"policy_uid"`
	ResourceVersion  string          `json:"resource_version"`
	Spec             json.RawMessage `json:"spec"`
}

type DriftReport struct {
	SchemaVersion    string   `json:"schema_version"`
	Source           string   `json:"source"`
	RunID            string   `json:"run_id"`
	Cluster          string   `json:"cluster"`
	PolicyName       string   `json:"policy_name"`
	PolicyUID        string   `json:"policy_uid"`
	ResourceVersion  string   `json:"resource_version"`
	ObservationState string   `json:"observation_state"`
	Verdict          string   `json:"verdict"`
	Differences      []string `json:"differences"`
	Limitations      []string `json:"limitations"`
}

var driftFields = map[string]bool{
	"schema_version": true, "source": true, "run_id": true, "cluster": true,
	"context": true, "namespace": true, "policy_name": true,
	"observation_state": true, "policy_uid": true, "resource_version": true,
	"spec": true,
}

// Strictly decode JSON at every nesting level. Reject duplicate object keys,
// oversized snapshots, non-object roots, extra/missing envelope fields,
// nulls outside the explicitly permitted absent/unavailable spec case,
// and trailing JSON. Go's ordinary json.Unmarshal permits duplicate keys.
func readDriftValue(d *json.Decoder, depth int) (any, error) {
	if depth > 16 {
		return nil, errors.New("snapshot nesting exceeds limit")
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); ok {
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return nil, err
				}
				key, ok := k.(string)
				if !ok {
					return nil, errors.New("invalid object key")
				}
				if _, exists := m[key]; exists {
					return nil, fmt.Errorf("duplicate JSON field %q", key)
				}
				v, err := readDriftValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				m[key] = v
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, errors.New("unclosed object")
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				v, err := readDriftValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, errors.New("unclosed array")
			}
			return a, nil
		default:
			return nil, errors.New("unexpected closing delimiter")
		}
	}
	return token, nil
}

func strictDriftObject(b []byte) (map[string]any, error) {
	if len(b) == 0 || len(b) > DriftMaxSnapshotBytes {
		return nil, errors.New("snapshot empty or oversized")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := readDriftValue(d, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot JSON: %w", err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("snapshot must be one JSON object")
	}
	_, err = d.Token()
	if err != io.EOF {
		return nil, errors.New("trailing or invalid JSON")
	}
	return m, nil
}

func DecodeDrift(b []byte) (DriftSnapshot, error) {
	var o DriftSnapshot
	m, err := strictDriftObject(b)
	if err != nil {
		return o, err
	}
	if len(m) != len(driftFields) {
		return o, errors.New("drift snapshot missing or unknown envelope fields")
	}
	for k := range m {
		if !driftFields[k] {
			return o, fmt.Errorf("unknown drift field %q", k)
		}
	}
	for k, v := range m {
		if v == nil && k != "spec" {
			return o, fmt.Errorf("null drift field %q", k)
		}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return DriftSnapshot{}, fmt.Errorf("snapshot field type: %w", err)
	}
	if err := o.Validate(); err != nil {
		return DriftSnapshot{}, err
	}
	return o, nil
}

func (o DriftSnapshot) Validate() error {
	if o.SchemaVersion != DriftSchemaVersion || o.Source != driftSource {
		return errors.New("drift source/schema mismatch")
	}
	if !ephemeralClusterPattern.MatchString(o.Cluster) || o.RunID != o.Cluster || o.Context != "kind-"+o.Cluster {
		return errors.New("snapshot outside isolated disposable kind scope")
	}
	if o.Namespace != "firedoor-v09c1" || o.PolicyName != "allow-trusted-to-data" {
		return errors.New("unexpected drift policy identity")
	}
	switch o.ObservationState {
	case "found":
		if !kubernetesUIDPattern.MatchString(o.PolicyUID) || !driftResourceVersion.MatchString(o.ResourceVersion) {
			return errors.New("found policy missing valid Kubernetes UID or resourceVersion")
		}
		if len(o.Spec) == 0 || bytes.Equal(bytes.TrimSpace(o.Spec), []byte("null")) {
			return errors.New("found policy must include full spec object")
		}
		sm, err := strictDriftObject(o.Spec)
		if err != nil {
			return fmt.Errorf("policy spec: %w", err)
		}
		if sm == nil {
			return errors.New("policy spec missing")
		}
	case "not_found", "unavailable":
		if o.PolicyUID != "" || o.ResourceVersion != "" || !bytes.Equal(bytes.TrimSpace(o.Spec), []byte("null")) {
			return errors.New("non-found snapshot must contain empty identifiers and null spec")
		}
	default:
		return errors.New("unknown drift observation state")
	}
	return nil
}

// Compare complete spec JSON, not a handpicked 'safe-looking' field subset.
// Map key order is irrelevant; additional rules, selectors, ports, peer
// permissions, missing entries, and egress fields are flagged as DRIFT.
func driftDiff(want, got any, path string, out *[]string) {
	if len(*out) >= 24 {
		return
	}
	wm, wMap := want.(map[string]any)
	gm, gMap := got.(map[string]any)
	if wMap && gMap {
		keys := make([]string, 0, len(wm)+len(gm))
		seen := map[string]bool{}
		for k := range wm {
			keys = append(keys, k)
			seen[k] = true
		}
		for k := range gm {
			if !seen[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			w, wok := wm[k]
			g, gok := gm[k]
			p := path + "." + k
			if !wok {
				*out = append(*out, p+": unexpected field")
			} else if !gok {
				*out = append(*out, p+": missing field")
			} else {
				driftDiff(w, g, p, out)
			}
			if len(*out) >= 24 {
				return
			}
		}
		return
	}
	wa, wArr := want.([]any)
	ga, gArr := got.([]any)
	if wArr && gArr {
		if len(wa) != len(ga) {
			*out = append(*out, fmt.Sprintf("%s: expected %d entries, observed %d", path, len(wa), len(ga)))
		}
		for i := 0; i < len(wa) && i < len(ga); i++ {
			driftDiff(wa[i], ga[i], fmt.Sprintf("%s[%d]", path, i), out)
			if len(*out) >= 24 {
				return
			}
		}
		return
	}
	if fmt.Sprintf("%T:%v", want, want) != fmt.Sprintf("%T:%v", got, got) {
		*out = append(*out, path+": value/type differs")
	}
}

func EvaluateDrift(o DriftSnapshot) (DriftReport, error) {
	if err := o.Validate(); err != nil {
		return DriftReport{}, err
	}
	r := DriftReport{
		SchemaVersion: DriftSchemaVersion, Source: o.Source, RunID: o.RunID, Cluster: o.Cluster,
		PolicyName: o.PolicyName, PolicyUID: o.PolicyUID, ResourceVersion: o.ResourceVersion,
		ObservationState: o.ObservationState, Verdict: "INCONCLUSIVE", Differences: []string{},
		Limitations: []string{
			"A JSON source label is not an attestation; only the guarded 9E2 harness can establish live API origin within its isolated cluster.",
			"Only this disposable kind namespace/policy spec is compared; no Terraform drift or production coverage.",
			"A matching spec is configuration equivalence, not proof of effective network enforcement; live reachability is a separate check.",
		},
	}
	switch o.ObservationState {
	case "unavailable":
		return r, nil
	case "not_found":
		r.Verdict = "DRIFT"
		r.Differences = append(r.Differences, "policy: confirmed absent")
		return r, nil
	}
	desired, err := strictDriftObject([]byte(approvedIngressSpec))
	if err != nil {
		return DriftReport{}, err
	}
	observed, err := strictDriftObject(o.Spec)
	if err != nil {
		return DriftReport{}, err
	}
	driftDiff(desired, observed, "spec", &r.Differences)
	if len(r.Differences) == 0 {
		r.Verdict = "MATCH"
	} else {
		r.Verdict = "DRIFT"
	}
	return r, nil
}
