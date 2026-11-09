package containmentmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func writeHTTPApprovedTestFile(t *testing.T, profile liveverify.Profile) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ApprovedHistoricalMapRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, savedFixtureJSON(t, profile), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestApprovedHistoricalHTTPMapOnlyFixedLocalPathAndConsent(t *testing.T) {
	for _, profile := range []liveverify.Profile{liveverify.ProfileBaseline, liveverify.ProfileHardened} {
		t.Run(string(profile), func(t *testing.T) {
			root := writeHTTPApprovedTestFile(t, profile)
			for _, consent := range []string{"", "WRONG"} {
				if _, err := LoadApprovedHistoricalMapForHTTP(root, profile, consent); err == nil {
					t.Fatal("missing consent accepted")
				}
			}
			if _, err := LoadApprovedHistoricalMapForHTTP(root, "production", ApprovedHistoricalMapConsent); err == nil {
				t.Fatal("unsupported profile accepted")
			}
			opposite := liveverify.ProfileBaseline
			if profile == opposite {
				opposite = liveverify.ProfileHardened
			}
			if _, err := LoadApprovedHistoricalMapForHTTP(root, opposite, ApprovedHistoricalMapConsent); err == nil {
				t.Fatal("cross-profile historical observation accepted")
			}
			got, err := LoadApprovedHistoricalMapForHTTP(root, profile, ApprovedHistoricalMapConsent)
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != "historical_projection" || got.CurrentInventory || got.EnforcementVerified || len(got.Routes) != 6 || len(got.Nodes) != 3 {
				t.Fatal("invalid historical HTTP projection")
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"10.70.0.", "uid-edge", "firedoor-edge-abc-123", "v03-suite-", root, "terraform-applied"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("leaked private source string: %q", secret)
				}
			}
		})
	}
}

func TestApprovedHistoricalHTTPMapFailsClosedForMissingOrLinkedFiles(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadApprovedHistoricalMapForHTTP(root, liveverify.ProfileBaseline, ApprovedHistoricalMapConsent); err == nil || strings.Contains(err.Error(), root) {
		t.Fatal("missing fixed input accepted or path leaked")
	}
	real := writeSavedFixture(t, savedFixtureJSON(t, liveverify.ProfileBaseline))
	path := filepath.Join(root, filepath.FromSlash(ApprovedHistoricalMapRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, path); err == nil {
		if _, err := LoadApprovedHistoricalMapForHTTP(root, liveverify.ProfileBaseline, ApprovedHistoricalMapConsent); err == nil {
			t.Fatal("symlink source accepted")
		}
	} else {
		t.Log("symlink creation unavailable; regular-file checks still run")
	}
}

func TestHistoricalHTTPProjectionRejectsUntrustedAndNonCanonicalMaps(t *testing.T) {
	historical, err := ProjectV03HistoricalObservation(historicalFixture(t, liveverify.ProfileHardened))
	if err != nil {
		t.Fatal(err)
	}
	got, err := PrepareHistoricalHTTPProjection(historical)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "historical_projection" || len(got.Evidence) != 6 || got.Evidence[0].SourceID != "approved-local-v03" {
		t.Fatal("wrong minimization")
	}
	if reflect.DeepEqual(got, historical) {
		t.Fatal("historical source identifiers not scrubbed")
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	testCases := map[string]func(*Map){
		"declared-only":     func(m *Map) { m.Source = "declared_contract" },
		"enforcement":       func(m *Map) { m.EnforcementVerified = true },
		"one route omitted": func(m *Map) { m.Routes = m.Routes[:5] },
		"extra workload": func(m *Map) {
			m.Nodes = append(m.Nodes, Node{ID: "foo", Kind: "workload", Zone: "app", Namespace: "firedoor-app", Workload: "firedoor-app"})
		},
		"route reordered":      func(m *Map) { m.Routes[0], m.Routes[1] = m.Routes[1], m.Routes[0] },
		"wrong evidence ID":    func(m *Map) { m.Routes[0].ObservationEvidenceIDs[0] = "observation-app-to-data" },
		"invented source kind": func(m *Map) { m.Evidence[0].Kind = "terraform_plan" },
		"extra evidence": func(m *Map) {
			m.Evidence = append(m.Evidence, EvidenceRef{ID: "extra", Kind: "network_observation", SourceID: "fake", CapturedAtUTC: m.Evidence[0].CapturedAtUTC, Profile: m.Profile, Result: "reachable"})
		},
	}
	for name, edit := range testCases {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(historical)
			var modified Map
			if err := json.Unmarshal(data, &modified); err != nil {
				t.Fatal(err)
			}
			edit(&modified)
			if _, err := PrepareHistoricalHTTPProjection(modified); err == nil {
				t.Fatal("unsafe map accepted")
			}
		})
	}
}
