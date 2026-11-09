package containmentmap

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

// historicalFixture creates only in-memory, invented TEST data in the fixed
// owned lab scope. It is not presented as a captured real-world observation.
func historicalFixture(t *testing.T, profile liveverify.Profile) liveverify.SuiteObservation {
	t.Helper()
	plan, err := liveverify.NewPlan(profile)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	pods := []liveverify.OwnedPod{
		{Zone: liveverify.ZoneEdge, Namespace: "firedoor-edge", PodName: "firedoor-edge-abc-123", UID: "uid-edge", IP: "10.70.0.2"},
		{Zone: liveverify.ZoneApp, Namespace: "firedoor-app", PodName: "firedoor-app-abc-123", UID: "uid-app", IP: "10.70.0.3"},
		{Zone: liveverify.ZoneData, Namespace: "firedoor-data", PodName: "firedoor-data-abc-123", UID: "uid-data", IP: "10.70.0.4"},
	}
	pod := func(zone liveverify.Zone) liveverify.OwnedPod {
		for _, p := range pods {
			if p.Zone == zone {
				return p
			}
		}
		t.Fatalf("missing test zone: %s", zone)
		return liveverify.OwnedPod{}
	}
	suite := liveverify.SuiteObservation{
		SchemaVersion:     liveverify.ObservationSchema,
		Cluster:           liveverify.OwnedCluster,
		Context:           liveverify.OwnedContext,
		Profile:           profile,
		ProfileProvenance: "operator-supplied; independently verify Terraform state",
		StartedAtUTC:      start,
		FinishedAtUTC:     start.Add(60 * time.Second),
		Inventory: liveverify.Inventory{
			SchemaVersion: liveverify.ObservationSchema,
			Cluster:       liveverify.OwnedCluster,
			Context:       liveverify.OwnedContext,
			CapturedAtUTC: start.Add(time.Second),
			Pods:          pods,
		},
		Routes:      make([]liveverify.RouteObservation, 0, len(plan.Probes)),
		Status:      "completed",
		Limitations: "Synthetic in-memory test data; not proof of enforcement.",
	}
	attemptTime := start.Add(2 * time.Second)
	for _, p := range plan.Probes {
		src, dst := pod(p.Source), pod(p.Destination)
		r := liveverify.RouteObservation{
			ID: p.ID, Source: p.Source, Destination: p.Destination,
			Expected: p.Expected, Observed: liveverify.Observation(p.Expected),
			Reason: "synthetic fixture", SourcePod: src.PodName,
			TargetPod: dst.PodName, TargetPodIP: dst.IP,
		}
		count := 1
		if p.Expected == liveverify.ExpectUnreachable {
			count = 2
		}
		for n := 1; n <= count; n++ {
			reason := "destination returned the exact expected HTTP body"
			if p.Expected == liveverify.ExpectUnreachable {
				reason = "HTTP request failed; cause is not attributed to a NetworkPolicy"
			}
			r.Attempts = append(r.Attempts, liveverify.Attempt{
				Number: n, AtUTC: attemptTime, DurationMillis: 25,
				Observation: liveverify.Observation(p.Expected), Reason: reason,
			})
			attemptTime = attemptTime.Add(time.Second)
		}
		suite.Routes = append(suite.Routes, r)
	}
	if _, err := liveverify.Verify(plan, suite); err != nil {
		t.Fatalf("invalid test fixture: %v", err)
	}
	return suite
}

func TestProjectV03HistoricalObservationSixRoutesAndProvenance(t *testing.T) {
	for _, profile := range []liveverify.Profile{liveverify.ProfileBaseline, liveverify.ProfileHardened} {
		t.Run(string(profile), func(t *testing.T) {
			suite := historicalFixture(t, profile)
			m, err := ProjectV03HistoricalObservation(suite)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
			if m.Source != "historical_projection" || m.Profile != string(profile) || m.CurrentInventory || m.EnforcementVerified {
				t.Fatalf("invalid historical boundary: %+v", m)
			}
			if len(m.Nodes) != 3 || len(m.Routes) != 6 || len(m.Evidence) != 6 {
				t.Fatalf("unexpected historical shape: %+v", m)
			}
			for i, r := range m.Routes {
				if r.ID != suite.Routes[i].ID || r.Observed != string(suite.Routes[i].Observed) || r.Assessment != "inconclusive" ||
					r.Declared != "unspecified" || r.DeclaredEvidenceID != "" || len(r.ObservationEvidenceIDs) != 1 {
					t.Fatalf("route %s invented a finding: %+v", r.ID, r)
				}
				if m.Evidence[i].ID != r.ObservationEvidenceIDs[0] || m.Evidence[i].Profile != string(profile) ||
					m.Evidence[i].Result != r.Observed || m.Evidence[i].Kind != "network_observation" ||
					m.Evidence[i].CapturedAtUTC != suite.Routes[i].Attempts[len(suite.Routes[i].Attempts)-1].AtUTC {
					t.Fatalf("evidence for %s mismatches observation", r.ID)
				}
				if !strings.HasPrefix(m.Evidence[i].SourceID, "v03-suite-") {
					t.Fatal("missing bounded suite content identifier")
				}
			}
			encoded, err := json.Marshal(m)
			if err != nil || len(encoded) > MaxJSONBytes {
				t.Fatal("output encoding exceeded bound", err)
			}
			for _, sensitive := range []string{"10.70.0.", "firedoor-edge-abc-123", "uid-edge"} {
				if bytes.Contains(encoded, []byte(sensitive)) {
					t.Fatalf("private Pod identity leaked: %s", sensitive)
				}
			}
			decoded, err := DecodeMap(encoded)
			if err != nil || !reflect.DeepEqual(m, decoded) {
				t.Fatalf("strict map round-trip failed: %v", err)
			}
			again, err := ProjectV03HistoricalObservation(suite)
			if err != nil || !reflect.DeepEqual(again, m) {
				t.Fatal("same saved suite must produce deterministic result", err)
			}
		})
	}
}

func TestProjectV03HistoricalObservationPreservesMismatchWithoutClaimingEnforcement(t *testing.T) {
	suite := historicalFixture(t, liveverify.ProfileHardened)
	r := &suite.Routes[2] // edge-to-data is expected unreachable when hardened
	r.Observed = liveverify.ObservedReachable
	r.Reason = "HTTP reachable with exact destination marker"
	r.Attempts = r.Attempts[:1]
	r.Attempts[0].Observation = liveverify.ObservedReachable
	r.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	m, err := ProjectV03HistoricalObservation(suite)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Routes[2]
	if got.Observed != "reachable" || got.Assessment != "inconclusive" || !strings.Contains(got.Reason, "mismatch") ||
		m.EnforcementVerified || got.Declared != "unspecified" {
		t.Fatalf("mismatch hidden or promoted to security verdict: %+v", got)
	}
}

func TestProjectV03HistoricalObservationKeepsInconclusiveRouteUnknown(t *testing.T) {
	suite := historicalFixture(t, liveverify.ProfileHardened)
	suite.Routes[0].Observed = liveverify.ObservedInconclusive
	suite.Routes[0].Reason = "kubectl exec failed"
	suite.Routes[0].Attempts = nil
	suite.Status = "incomplete"
	m, err := ProjectV03HistoricalObservation(suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Evidence) != 5 || m.Routes[0].Observed != "not_observed" ||
		m.Routes[0].Assessment != "not_assessed" || len(m.Routes[0].ObservationEvidenceIDs) != 0 ||
		!strings.Contains(m.Routes[0].Reason, "inconclusive") {
		t.Fatalf("failed probe converted into a valid network result: %+v", m.Routes[0])
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectV03HistoricalObservationRejectsUnsupportedOrTamperedReports(t *testing.T) {
	tests := []struct {
		name string
		edit func(*liveverify.SuiteObservation)
	}{
		{"wrong cluster", func(o *liveverify.SuiteObservation) { o.Cluster = "production" }},
		{"wrong context", func(o *liveverify.SuiteObservation) { o.Context = "kind-other" }},
		{"wrong profile", func(o *liveverify.SuiteObservation) { o.Profile = "production" }},
		{"wrong profile provenance", func(o *liveverify.SuiteObservation) { o.ProfileProvenance = "terraform-applied" }},
		{"wrong inventory IP", func(o *liveverify.SuiteObservation) { o.Inventory.Pods[0].IP = "8.8.8.8" }},
		{"wrong source Pod", func(o *liveverify.SuiteObservation) { o.Routes[0].SourcePod = "other-pod" }},
		{"wrong target IP", func(o *liveverify.SuiteObservation) { o.Routes[0].TargetPodIP = "10.2.2.2" }},
		{"changed route", func(o *liveverify.SuiteObservation) { o.Routes[0].ID = "edge-to-world" }},
		{"wrong expectation", func(o *liveverify.SuiteObservation) { o.Routes[2].Expected = "reachable" }},
		{"false reachable marker", func(o *liveverify.SuiteObservation) { o.Routes[0].Attempts[0].Reason = "HTTP 200" }},
		{"missing repeated failures", func(o *liveverify.SuiteObservation) { o.Routes[2].Attempts = o.Routes[2].Attempts[:1] }},
		{"future attempt", func(o *liveverify.SuiteObservation) { o.Routes[0].Attempts[0].AtUTC = o.FinishedAtUTC.Add(time.Second) }},
		{"missing suite status", func(o *liveverify.SuiteObservation) { o.Status = "" }},
		{"oversized input", func(o *liveverify.SuiteObservation) {
			o.Limitations = strings.Repeat("x", liveverify.MaxObservationBytes+1)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite := historicalFixture(t, liveverify.ProfileHardened)
			tt.edit(&suite)
			m, err := ProjectV03HistoricalObservation(suite)
			if err == nil || !reflect.DeepEqual(m, Map{}) {
				t.Fatalf("tampered observation accepted: %s", tt.name)
			}
		})
	}
}

func TestProjectV03HistoricalObservationDoesNotMutateSource(t *testing.T) {
	suite := historicalFixture(t, liveverify.ProfileHardened)
	original, _ := json.Marshal(suite)
	m, err := ProjectV03HistoricalObservation(suite)
	if err != nil {
		t.Fatal(err)
	}
	m.Routes[0].Reason = "tampered output"
	m.Nodes[0].Namespace = "production"
	m.Evidence[0].SourceID = "tampered"
	current, _ := json.Marshal(suite)
	if !bytes.Equal(original, current) {
		t.Fatal("projection changed saved observation input")
	}
	fresh, err := ProjectV03HistoricalObservation(suite)
	if err != nil || fresh.Routes[0].Reason == "tampered output" || fresh.Evidence[0].SourceID == "tampered" {
		t.Fatal("output aliases source or previous projection", err)
	}
}
