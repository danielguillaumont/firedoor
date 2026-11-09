package regression

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func sampleDrift(t *testing.T) DriftSnapshot {
	t.Helper()
	b := testDriftFixture(t, "protected.json")
	o, err := DecodeDrift(b)
	if err != nil {
		t.Fatalf("valid fixture: %v", err)
	}
	return o
}
func testDriftFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := osReadFixture("testdata/drift/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func osReadFixture(name string) ([]byte, error) { return os.ReadFile(name) }
func TestDriftFixtures(t *testing.T) {
	cases := []struct {
		name, verdict string
		count         int
	}{
		{"protected.json", "MATCH", 0}, {"weakened.json", "DRIFT", 1}, {"missing.json", "DRIFT", 1}, {"unavailable.json", "INCONCLUSIVE", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := DecodeDrift(testDriftFixture(t, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			r, err := EvaluateDrift(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Verdict != tc.verdict {
				t.Fatalf("%s instead of %s", r.Verdict, tc.verdict)
			}
			if len(r.Differences) < tc.count {
				t.Fatalf("missing drift differences: %v", r.Differences)
			}
		})
	}
}
func TestDriftNegativeInputs(t *testing.T) {
	original := testDriftFixture(t, "protected.json")
	// Tests MUST change their input; otherwise a negative case could pass
	// without ever exercising the intended invalid-input condition.
	replace := func(before, after string) []byte {
		t.Helper()
		if !bytes.Contains(original, []byte(before)) {
			t.Fatalf("test fixture no longer contains %q", before)
		}
		return bytes.Replace(original, []byte(before), []byte(after), 1)
	}
	mutate := func(edit func(map[string]any)) []byte {
		t.Helper()
		var obj map[string]any
		if err := json.Unmarshal(original, &obj); err != nil {
			t.Fatal(err)
		}
		edit(obj)
		b, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	cases := map[string][]byte{
		"empty":              nil,
		"trailing":           append(append([]byte{}, original...), []byte("{}")...),
		"array":              []byte("[]"),
		"duplicate-envelope": replace(`"source": "guarded_kind_calico_api"`, `"source": "guarded_kind_calico_api", "source": "guarded_kind_calico_api"`),
		"duplicate-nested":   replace(`"app": "data-server"`, `"app": "data-server", "app": "data-server"`),
		"unknown-field":      mutate(func(m map[string]any) { m["new_field"] = true }),
		"missing-key":        mutate(func(m map[string]any) { delete(m, "source") }),
		"wrong-type":         mutate(func(m map[string]any) { m["cluster"] = 123 }),
		"wrong-cluster": mutate(func(m map[string]any) {
			m["cluster"] = "firedoor-local"
			m["run_id"] = "firedoor-local"
			m["context"] = "kind-firedoor-local"
		}),
		"wrong-context": mutate(func(m map[string]any) { m["context"] = "kind-firedoor-local" }),
		"wrong-source":  mutate(func(m map[string]any) { m["source"] = "untrusted" }),
		"null-state":    mutate(func(m map[string]any) { m["observation_state"] = nil }),
		"found-no-spec": mutate(func(m map[string]any) { m["spec"] = nil }),
		"huge":          bytes.Repeat([]byte("x"), DriftMaxSnapshotBytes+1),
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDrift(b); err == nil {
				t.Fatalf("unsafe input accepted: %s", name)
			}
		})
	}
}
func TestDriftSpecMutations(t *testing.T) {
	baseline := sampleDrift(t)
	tests := map[string]func(map[string]any){
		"unexpected-egress": func(m map[string]any) { m["egress"] = []any{map[string]any{}} },
		"allow-other-port": func(m map[string]any) {
			m["ingress"].([]any)[0].(map[string]any)["ports"] = []any{map[string]any{"protocol": "TCP", "port": float64(8080)}, map[string]any{"protocol": "TCP", "port": float64(443)}}
		},
		"no-ingress": func(m map[string]any) { m["ingress"] = []any{} },
		"wrong-destination": func(m map[string]any) {
			m["podSelector"].(map[string]any)["matchLabels"].(map[string]any)["app"] = "other"
		},
		"wrong-role": func(m map[string]any) {
			m["ingress"].([]any)[0].(map[string]any)["from"].([]any)[0].(map[string]any)["podSelector"].(map[string]any)["matchLabels"].(map[string]any)["role"] = "edge"
		},
		"empty-peer-selector": func(m map[string]any) {
			m["ingress"].([]any)[0].(map[string]any)["from"].([]any)[0].(map[string]any)["podSelector"] = map[string]any{}
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			var copySpec map[string]any
			if err := json.Unmarshal(baseline.Spec, &copySpec); err != nil {
				t.Fatal(err)
			}
			edit(copySpec)
			b, _ := json.Marshal(copySpec)
			o := baseline
			o.Spec = b
			r, err := EvaluateDrift(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Verdict != "DRIFT" || len(r.Differences) == 0 {
				t.Fatalf("mutation not detected: %+v", r)
			}
		})
	}
}
func TestDriftUntrustedMissingIsNotMatch(t *testing.T) {
	o := sampleDrift(t)
	o.ObservationState = "unavailable"
	o.Spec = []byte("null")
	o.PolicyUID = ""
	o.ResourceVersion = ""
	r, err := EvaluateDrift(o)
	if err != nil || r.Verdict != "INCONCLUSIVE" {
		t.Fatalf("unavailable passed: %#v %v", r, err)
	}
	o.ObservationState = "not_found"
	r, err = EvaluateDrift(o)
	if err != nil || r.Verdict != "DRIFT" {
		t.Fatalf("confirmed missing did not drift: %#v %v", r, err)
	}
	o.PolicyUID = "123"
	if _, err := EvaluateDrift(o); err == nil {
		t.Fatal("absent with policy uid accepted")
	}
}
func TestDriftNoRawValuesInDiff(t *testing.T) {
	o := sampleDrift(t)
	o.Spec = []byte(`{"podSelector":{"matchLabels":{"app":"private-sensitive-text"}}}`)
	r, err := EvaluateDrift(o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != "DRIFT" {
		t.Fatal("mismatched spec passed")
	}
	for _, d := range r.Differences {
		if strings.Contains(d, "private-sensitive-text") {
			t.Fatal("raw diff disclosed input value")
		}
	}
}
