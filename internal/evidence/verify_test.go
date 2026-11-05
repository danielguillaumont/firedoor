package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func fixture(profile string) Record {
	when := "2026-11-04T23:15:12Z"
	if profile == "hardened" {
		when = "2026-11-04T23:48:30Z"
	}
	ips := map[string]string{"edge": "192.168.91.3", "app": "192.168.91.4", "data": "192.168.91.5"}
	r := Record{
		TimestampUTC:    when,
		Cluster:         "firedoor-local",
		Context:         "kind-firedoor-local",
		SecurityProfile: profile,
		Protocol:        "HTTP over TCP/8080 direct Pod IP",
		Conclusion:      "OBSERVED_" + profile + "_INGRESS_BEHAVIOR",
		Limits:          "Lab-only test; timeout does not prove packet-drop attribution.",
	}
	for _, p := range paths {
		connected := profile == "baseline" || p.hardenedConnected
		observed, attempts := "CONNECTED", 1
		if !connected {
			observed, attempts = "UNREACHABLE_TWICE", 2
		}
		r.Observations = append(r.Observations, Observation{
			SourceZone: p.from, DestinationZone: p.to,
			TargetPodIP: ips[p.to], ExpectedConnected: boolPtr(connected),
			Observed: observed, Attempts: attempts,
		})
	}
	return r
}

func TestVerifyAcceptsControlledComparison(t *testing.T) {
	got, err := Verify(fixture("baseline"), fixture("hardened"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "evidence_consistent" || got.PathsCompared != 6 || got.HardenedDeniedTwice != 2 || got.BaselineConnected != 6 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestVerifyRejectsInvalidEvidence(t *testing.T) {
	tests := []struct {
		name     string
		change   func(*Record, *Record)
		contains string
	}{
		{"bad cluster", func(b, h *Record) { h.Cluster = "other-cluster" }, "unexpected cluster"},
		{"wrong hardened profile", func(b, h *Record) { h.SecurityProfile = "baseline" }, "expected profile"},
		{"missing observation", func(b, h *Record) { h.Observations = h.Observations[:5] }, "expected 6 observations"},
		{"duplicate path", func(b, h *Record) { h.Observations[1] = h.Observations[0] }, "duplicate path"},
		{"false denied outcome", func(b, h *Record) { h.Observations[2].Observed = "CONNECTED" }, "expected at least two denied attempts"},
		{"only one denial", func(b, h *Record) { h.Observations[2].Attempts = 1 }, "expected at least two denied attempts"},
		{"missing expected flag", func(b, h *Record) { h.Observations[0].ExpectedConnected = nil }, "expected_connected"},
		{"baseline not connected", func(b, h *Record) { b.Observations[0].Observed = "UNREACHABLE_TWICE" }, "expected one successful"},
		{"invalid address", func(b, h *Record) { h.Observations[0].TargetPodIP = "127.0.0.1" }, "invalid target"},
		{"changed pod", func(b, h *Record) {
			for i := range h.Observations {
				if h.Observations[i].DestinationZone == "app" {
					h.Observations[i].TargetPodIP = "192.168.91.90"
				}
			}
		}, "target pod IP changed"},
		{"out of order", func(b, h *Record) { h.TimestampUTC = "2026-11-04T23:00:00Z" }, "timestamp must be later"},
		{"wrong protocol", func(b, h *Record) { h.Protocol = "UDP" }, "unexpected protocol"},
		{"missing limitations", func(b, h *Record) { h.Limits = "" }, "limitations missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, h := fixture("baseline"), fixture("hardened")
			tt.change(&b, &h)
			_, err := Verify(b, h)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("want error containing %q; got %v", tt.contains, err)
			}
		})
	}
}

func TestLoadPowerShellUTF8BOMAndRejectsUnknownKeys(t *testing.T) {
	r := fixture("baseline")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, data...), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecurityProfile != "baseline" {
		t.Fatalf("unexpected profile %q", got.SecurityProfile)
	}
	bad := append([]byte(`{"unexpected":true}`), byte('\n'))
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestLoadRejectsSecondValueAndOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{} {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected multiple JSON value rejection")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxEvidenceBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestVerifyFiles(t *testing.T) {
	dir := t.TempDir()
	bPath, hPath := filepath.Join(dir, "baseline.json"), filepath.Join(dir, "hardened.json")
	for _, x := range []struct {
		path   string
		record Record
	}{{bPath, fixture("baseline")}, {hPath, fixture("hardened")}} {
		data, err := json.Marshal(x.record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(x.path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifyFiles(bPath, hPath); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyFiles(bPath, bPath); err == nil {
		t.Fatal("expected duplicate file path rejection")
	}
}
