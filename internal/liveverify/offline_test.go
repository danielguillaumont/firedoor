package liveverify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func observationFixtureFile(t *testing.T) string {
	t.Helper()
	_, observation := verifiedFixture(t)
	data, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "observation.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVerifyFileReportsRealisticSixRouteMatrix(t *testing.T) {
	report, err := VerifyFile(observationFixtureFile(t), ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != VerificationSchema || report.Status != "matched" || report.Summary != (VerificationSummary{Total: 6, Match: 6}) {
		t.Fatalf("unexpected report: %+v", report)
	}
	human := HumanVerificationReport(report)
	for _, part := range []string{"OFFLINE", "MATCH 6", "edge-to-data", "does NOT prove"} {
		if !strings.Contains(human, part) {
			t.Fatalf("human report missing %q", part)
		}
	}
}

func TestVerifyFileRejectsUntrustedJSON(t *testing.T) {
	original, err := os.ReadFile(observationFixtureFile(t))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(original, &object); err != nil {
		t.Fatal(err)
	}
	object["unexpected_field"] = true
	extraField, _ := json.Marshal(object)
	tests := []struct {
		name string
		data []byte
	}{
		{"malformed", []byte(`{"broken":`)},
		{"trailing-json", append(append([]byte{}, original...), []byte(` {"extra":true}`)...)},
		{"unknown-field", extraField},
		{"oversized", []byte(strings.Repeat("X", MaxObservationBytes+1))},
		{"empty", []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.json")
			if err := os.WriteFile(path, tt.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyFile(path, ProfileHardened); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
	t.Run("directory", func(t *testing.T) {
		if _, err := VerifyFile(t.TempDir(), ProfileHardened); err == nil {
			t.Fatal("directory accepted")
		}
	})
	t.Run("missing-file", func(t *testing.T) {
		if _, err := VerifyFile(filepath.Join(t.TempDir(), "missing.json"), ProfileHardened); err == nil {
			t.Fatal("missing file accepted")
		}
	})
	t.Run("wrong-profile", func(t *testing.T) {
		if _, err := VerifyFile(observationFixtureFile(t), ProfileBaseline); err == nil {
			t.Fatal("wrong profile accepted")
		}
	})
	t.Run("unsupported-profile", func(t *testing.T) {
		if _, err := VerifyFile(observationFixtureFile(t), Profile("production")); err == nil {
			t.Fatal("unsupported profile accepted")
		}
	})
}

func TestVerifyFileKeepsMismatchesVisible(t *testing.T) {
	_, obs := verifiedFixture(t)
	route := &obs.Routes[2]
	route.Observed = ObservedReachable
	route.Reason = "HTTP reachable with exact destination marker"
	route.Attempts = route.Attempts[:1]
	route.Attempts[0].Observation = ObservedReachable
	route.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "mismatch.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyFile(path, ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "mismatched" || report.Summary.Mismatch != 1 || report.Summary.Match != 5 {
		t.Fatalf("mismatch suppressed: %+v", report)
	}
	if !strings.Contains(HumanVerificationReport(report), "MISMATCH 1") {
		t.Fatal("human output concealed mismatch")
	}
}
