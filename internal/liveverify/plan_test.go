package liveverify

import (
	"encoding/json"
	"testing"
)

func TestFixedMatrixMatchesV01ConnectivityChecks(t *testing.T) {
	hardened, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := NewPlan(ProfileBaseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(hardened.Probes) != 6 || len(baseline.Probes) != 6 {
		t.Fatalf("expected exactly six routes per profile")
	}
	allowed := map[string]Expectation{
		"edge-to-app":  ExpectReachable,
		"app-to-data":  ExpectReachable,
		"edge-to-data": ExpectUnreachable,
		"data-to-app":  ExpectUnreachable,
		"app-to-edge":  ExpectReachable,
		"data-to-edge": ExpectReachable,
	}
	for i := range hardened.Probes {
		got := hardened.Probes[i]
		if got.Expected != allowed[got.ID] {
			t.Errorf("hardened expected %s -> %s, got %s", got.ID, allowed[got.ID], got.Expected)
		}
		if baseline.Probes[i].ID != got.ID || baseline.Probes[i].Expected != ExpectReachable {
			t.Errorf("baseline must allow %s", got.ID)
		}
	}
	for _, p := range []Plan{baseline, hardened} {
		if err := p.Validate(); err != nil {
			t.Errorf("generated plan did not validate: %v", err)
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip Plan
		if err := json.Unmarshal(data, &roundtrip); err != nil {
			t.Fatal(err)
		}
		if err := roundtrip.Validate(); err != nil {
			t.Errorf("serialized plan invalid: %v", err)
		}
	}
}

func TestPlanIsStrictlyLimitedToOwnedLab(t *testing.T) {
	base, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Plan)
	}{
		{"wrong cluster", func(p *Plan) { p.Cluster = "production" }},
		{"wrong context", func(p *Plan) { p.Context = "admin@prod" }},
		{"wrong schema", func(p *Plan) { p.SchemaVersion = "firedoor.v99" }},
		{"wrong profile", func(p *Plan) { p.Profile = "unknown" }},
		{"different endpoint mode", func(p *Plan) { p.EndpointMode = "user-supplied-ip" }},
		{"different port", func(p *Plan) { p.Port = 443 }},
		{"different transport", func(p *Plan) { p.Transport = "UDP" }},
		{"unbounded attempt timeout", func(p *Plan) { p.Limits.AttemptTimeoutSeconds = 999 }},
		{"too many retries", func(p *Plan) { p.Limits.MaxAttemptsUnreachable = 100 }},
		{"missing denial confirmation", func(p *Plan) { p.Limits.RequiredConsecutiveFailures = 1 }},
		{"missing probe", func(p *Plan) { p.Probes = p.Probes[:5] }},
		{"unexpected destination", func(p *Plan) { p.Probes[0].Destination = "outside" }},
		{"unexpected source", func(p *Plan) { p.Probes[0].Source = "outside" }},
		{"changed expected denial", func(p *Plan) { p.Probes[2].Expected = ExpectReachable }},
		{"duplicate route", func(p *Plan) { p.Probes[5] = p.Probes[0] }},
		{"renamed route", func(p *Plan) { p.Probes[0].ID = "custom" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := base
			p.Probes = append([]Probe(nil), base.Probes...)
			tt.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("expected unsafe/altered plan to be rejected")
			}
		})
	}
}

func TestExpectedBodiesAreExactAndUnknownZonesFailClosed(t *testing.T) {
	tests := []struct {
		zone Zone
		body string
	}{
		{ZoneEdge, "FIREDOOR_EDGE_OK"},
		{ZoneApp, "FIREDOOR_APP_OK"},
		{ZoneData, "FIREDOOR_DATA_OK"},
	}
	for _, tt := range tests {
		got, err := ExpectedHTTPBody(tt.zone)
		if err != nil || got != tt.body {
			t.Errorf("unexpected response marker for %s: %q, %v", tt.zone, got, err)
		}
	}
	if _, err := ExpectedHTTPBody("other"); err == nil {
		t.Fatal("unknown destination must fail closed")
	}
}

func TestUnsupportedProfileRejected(t *testing.T) {
	if _, err := NewPlan("prod"); err == nil {
		t.Fatal("unexpected profile accepted")
	}
}

func TestPlanIsPureContractAndHasStrictTimeBudget(t *testing.T) {
	p, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != 8080 || p.EndpointMode != EndpointModeDirectPodIP {
		t.Fatal("expected only direct Pod IP on 8080")
	}
	if p.Limits.RequiredConsecutiveFailures != 2 {
		t.Fatal("must require two consecutive negative observations")
	}
	if p.Limits.MaxSuiteDurationSeconds > 180 {
		t.Fatal("suite time bound is too high")
	}
}
