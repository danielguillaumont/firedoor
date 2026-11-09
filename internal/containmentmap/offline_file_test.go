package containmentmap

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func savedFixtureJSON(t *testing.T, profile liveverify.Profile) []byte {
	t.Helper()
	data, err := json.Marshal(historicalFixture(t, profile))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeSavedFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private-v03-observation.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProjectV03HistoricalFileAcceptsOnlyRealSchemaWithRequestedProfile(t *testing.T) {
	for _, profile := range []liveverify.Profile{liveverify.ProfileBaseline, liveverify.ProfileHardened} {
		t.Run(string(profile), func(t *testing.T) {
			path := writeSavedFixture(t, savedFixtureJSON(t, profile))
			got, err := ProjectV03HistoricalFile(path, profile)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := ProjectV03HistoricalObservation(historicalFixture(t, profile))
			if err != nil || !reflect.DeepEqual(expected, got) {
				t.Fatalf("disk projection does not match verified in-memory projection: %v", err)
			}
			if got.Source != "historical_projection" || got.CurrentInventory || got.EnforcementVerified || len(got.Routes) != 6 {
				t.Fatal("historical route claims or cardinality violated")
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			for _, private := range []string{"10.70.0.", "firedoor-edge-abc-123", "uid-edge", path} {
				if bytes.Contains(encoded, []byte(private)) {
					t.Fatalf("private historical input leaked into projection: %s", private)
				}
			}
		})
	}
}

func TestProjectV03HistoricalFileFailsClosedWithoutLeakingPath(t *testing.T) {
	valid := savedFixtureJSON(t, liveverify.ProfileHardened)
	var extra map[string]any
	if err := json.Unmarshal(valid, &extra); err != nil {
		t.Fatal(err)
	}
	extra["unexpected_secret"] = "do not print me"
	unknown, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		data    []byte
		profile liveverify.Profile
	}{
		{"cross-profile", valid, liveverify.ProfileBaseline},
		{"unknown field", unknown, liveverify.ProfileHardened},
		{"trailing document", append(append([]byte{}, valid...), []byte(` {"extra":true}`)...), liveverify.ProfileHardened},
		{"trailing invalid token", append(append([]byte{}, valid...), []byte(" !!")...), liveverify.ProfileHardened},
		{"malformed", []byte(`{"schema_version":"partial"`), liveverify.ProfileHardened},
		{"empty", nil, liveverify.ProfileHardened},
		{"oversized", []byte(strings.Repeat("x", liveverify.MaxObservationBytes+1)), liveverify.ProfileHardened},
		{"unsupported profile", valid, "production"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := writeSavedFixture(t, test.data)
			got, err := ProjectV03HistoricalFile(path, test.profile)
			if err == nil || !reflect.DeepEqual(got, Map{}) {
				t.Fatal("invalid historical input accepted")
			}
			if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "do not print me") {
				t.Fatal("private input leaked through rejection error")
			}
		})
	}
	for _, path := range []string{"", "   ", filepath.Join(t.TempDir(), "missing-secret.json")} {
		if _, err := ProjectV03HistoricalFile(path, liveverify.ProfileHardened); err == nil || strings.Contains(err.Error(), "missing-secret") {
			t.Fatal("missing path accepted or leaked")
		}
	}
}
