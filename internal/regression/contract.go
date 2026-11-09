// Package regression implements the v0.9A synthetic boundary-regression
// contract. It does not run network probes or establish policy causality.
package regression

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	SchemaVersion       = "firedoor.v09.regression.fixture.v1"
	MaxFixtureBytes     = 4 * 1024
	expectedObservation = "unreachable"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// Observation is intentionally synthetic-only in Slice 9A. A future guarded
// live capture needs a distinct input and provenance contract.
type Observation struct {
	SchemaVersion string `json:"schema_version"`
	Source        string `json:"source"`
	RunID         string `json:"run_id"`
	Cluster       string `json:"cluster"`
	Context       string `json:"context"`
	Profile       string `json:"profile"`
	RouteID       string `json:"route_id"`
	Transport     string `json:"transport"`
	Port          int    `json:"port"`
	Observed      string `json:"observed"`
}

// Report signals whether a synthetic network observation matches a fixed
// hardened-profile expectation. MATCH is NOT a policy enforcement verdict.
type Report struct {
	SchemaVersion string   `json:"schema_version"`
	Source        string   `json:"source"`
	RunID         string   `json:"run_id"`
	RouteID       string   `json:"route_id"`
	Profile       string   `json:"profile"`
	Expected      string   `json:"expected"`
	Observed      string   `json:"observed"`
	Verdict       string   `json:"verdict"`
	Limitations   []string `json:"limitations"`
}

var allowedKeys = map[string]bool{
	"schema_version": true, "source": true, "run_id": true,
	"cluster": true, "context": true, "profile": true,
	"route_id": true, "transport": true, "port": true,
	"observed": true,
}

// Decode rejects oversized data, duplicate/unknown keys, trailing values and
// mismatched identity. Keeping 9A flat makes strict decoding auditable.
func Decode(input []byte) (Observation, error) {
	var o Observation
	if len(input) == 0 || len(input) > MaxFixtureBytes {
		return o, errors.New("fixture is empty or exceeds bounded input size")
	}
	d := json.NewDecoder(bytes.NewReader(input))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return o, errors.New("fixture must be a single JSON object")
	}
	seen := make(map[string]bool, len(allowedKeys))
	values := make(map[string]json.RawMessage, len(allowedKeys))
	for d.More() {
		keyToken, err := d.Token()
		if err != nil {
			return o, fmt.Errorf("fixture field token: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok || !allowedKeys[key] || seen[key] {
			return o, fmt.Errorf("fixture has unknown or repeated field %q", key)
		}
		seen[key] = true
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return o, fmt.Errorf("invalid value for %q: %w", key, err)
		}
		values[key] = raw
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return o, errors.New("fixture object not closed")
	}
	if _, err := d.Token(); err != io.EOF {
		return o, errors.New("fixture has trailing JSON or malformed content")
	}
	if len(values) != len(allowedKeys) {
		return o, errors.New("fixture has missing required fields")
	}
	// Re-marshal the validated, unique, known keys into typed fields. This
	// step also rejects strings in numeric fields and other type confusion.
	normalized, err := json.Marshal(values)
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(normalized, &o); err != nil {
		return o, fmt.Errorf("invalid fixture field type: %w", err)
	}
	if err := o.Validate(); err != nil {
		return Observation{}, err
	}
	return o, nil
}

func (o Observation) Validate() error {
	if o.SchemaVersion != SchemaVersion || o.Source != "synthetic_fixture" {
		return errors.New("only the v0.9A synthetic fixture schema is supported")
	}
	if !slugPattern.MatchString(o.RunID) || strings.Contains(o.RunID, "--") {
		return errors.New("invalid synthetic run ID")
	}
	if o.Cluster != "firedoor-local" || o.Context != "kind-firedoor-local" ||
		o.Profile != "hardened" || o.RouteID != "edge-to-data" ||
		o.Transport != "HTTP/TCP" || o.Port != 8080 {
		return errors.New("fixture is outside the fixed hardened lab boundary")
	}
	switch o.Observed {
	case "reachable", "unreachable", "not_observed":
	default:
		return errors.New("unsupported reachability observation")
	}
	return nil
}

func Evaluate(o Observation) (Report, error) {
	if err := o.Validate(); err != nil {
		return Report{}, err
	}
	verdict := "INCONCLUSIVE"
	switch o.Observed {
	case "unreachable":
		verdict = "MATCH"
	case "reachable":
		verdict = "REGRESSION"
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Source:        o.Source, RunID: o.RunID, RouteID: o.RouteID,
		Profile: o.Profile, Expected: expectedObservation,
		Observed: o.Observed, Verdict: verdict,
		Limitations: []string{
			"Synthetic fixture only: no Kubernetes cluster was queried or mutated.",
			"Matching reachability is not proof that NetworkPolicy caused the result.",
			"The v0.9A gate tests the evaluator; it does not validate the live Terraform policy.",
		},
	}, nil
}
