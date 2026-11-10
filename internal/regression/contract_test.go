package regression

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSyntheticFixtureVerdicts(t *testing.T) {
	cases := []struct{ name, want string }{
		{"intact", "MATCH"}, {"weakened", "REGRESSION"}, {"missing", "INCONCLUSIVE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Fixture(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			o, err := Decode(b)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Evaluate(o)
			if err != nil {
				t.Fatal(err)
			}
			if got.Verdict != tc.want {
				t.Fatalf("verdict=%q want %q", got.Verdict, tc.want)
			}
			if got.Expected != "unreachable" || got.Source != "synthetic_fixture" {
				t.Fatal("expectation/source drift")
			}
			if len(got.Limitations) != 3 {
				t.Fatal("missing truthful limitations")
			}
			output, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(output, []byte(`"enforcement_verified":true`)) {
				t.Fatal("fabricated enforcement")
			}
		})
	}
}

func TestFixtureRejectsUnknownName(t *testing.T) {
	for _, name := range []string{"../../docs/evidence/local", "", "baseline", "weakened.json"} {
		if _, err := Fixture(name); err == nil {
			t.Fatalf("accepted fixture name %q", name)
		}
	}
}

func TestFixtureStrictParsing(t *testing.T) {
	valid, err := Fixture("intact")
	if err != nil {
		t.Fatal(err)
	}
	change := func(key string, value any) []byte {
		t.Helper()
		var obj map[string]any
		if err := json.Unmarshal(valid, &obj); err != nil {
			t.Fatal(err)
		}
		obj[key] = value
		raw, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"array", []byte(`[]`)},
		{"unknown_key", change("unsafe_url", "https://example.org")},
		{"missing_key", bytes.Replace(valid, []byte(`"port": 8080,`), nil, 1)},
		{"duplicate_key", append(append([]byte(nil), bytes.TrimSuffix(bytes.TrimSpace(valid), []byte("}"))...), []byte(`,"port":8080}`)...)},
		{"trailing", append(append([]byte(nil), valid...), []byte(`{}`)...)},
		{"oversized", bytes.Repeat([]byte(" "), MaxFixtureBytes+1)},
		{"bad_type", change("port", "8080")},
		{"wrong_port", change("port", 443)},
		{"wrong_cluster", change("cluster", "external")},
		{"wrong_context", change("context", "kind-other")},
		{"wrong_profile", change("profile", "baseline")},
		{"wrong_route", change("route_id", "app-to-data")},
		{"wrong_transport", change("transport", "UDP")},
		{"wrong_source", change("source", "live_capture")},
		{"wrong_schema", change("schema_version", "firedoor.v08")},
		{"wrong_observation", change("observed", "pass")},
		{"bad_run_id", change("run_id", "../../local")},
		{"double_dash_run_id", change("run_id", "synthetic--pass")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(tc.data); err == nil {
				t.Fatal("malformed or out-of-scope fixture accepted")
			}
		})
	}
}

func TestDirectEvaluateFailClosed(t *testing.T) {
	b, _ := Fixture("intact")
	o, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	o.Source = "approved_live_proof"
	if _, err := Evaluate(o); err == nil {
		t.Fatal("direct evaluation bypassed contract")
	}
}

func TestNoCausalClaims(t *testing.T) {
	b, _ := Fixture("intact")
	o, _ := Decode(b)
	report, err := Evaluate(o)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict == "PASS" || report.Verdict == "VERIFIED" {
		t.Fatal("fixture interpreted as security attestation")
	}
	if !strings.Contains(report.Limitations[1], "not proof") {
		t.Fatal("missing causality warning")
	}
}
