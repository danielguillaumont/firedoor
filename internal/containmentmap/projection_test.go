package containmentmap

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func TestDeclaredProjectionMatchesValidatedSixRoutePlans(t *testing.T) {
	for _, profile := range []liveverify.Profile{liveverify.ProfileBaseline, liveverify.ProfileHardened} {
		t.Run(string(profile), func(t *testing.T) {
			plan, err := liveverify.NewPlan(profile)
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewDeclaredMap(profile)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(m.Nodes) != 3 || len(m.Routes) != 6 || len(m.Evidence) != 0 || m.Source != "declared_contract" {
				t.Fatalf("unexpected bounded topology shape: %+v", m)
			}
			if m.CurrentInventory || m.EnforcementVerified || m.Profile != string(profile) {
				t.Fatal("invented inventory, enforcement, or wrong profile")
			}
			for i, p := range plan.Probes {
				r := m.Routes[i]
				if r.ID != p.ID || r.From != string(p.Source) || r.To != string(p.Destination) {
					t.Fatalf("route %d does not map the validated plan: %+v vs %+v", i, r, p)
				}
				if !strings.Contains(r.Reason, "expects "+string(p.Expected)+" HTTP/TCP") {
					t.Fatalf("expected planning-only profile expectation missing on %s", r.ID)
				}
				if r.Declared != "unspecified" || r.DeclaredEvidenceID != "" || r.Observed != "not_observed" ||
					r.Assessment != "not_assessed" || len(r.ObservationEvidenceIDs) != 0 {
					t.Fatalf("route %s claims unsupported declared or observed state: %+v", r.ID, r)
				}
			}
			for _, node := range m.Nodes {
				if node.Kind != "zone" || node.Workload != "" || node.ObservationEvidenceID != "" {
					t.Fatalf("invented observed workload: %+v", node)
				}
			}
			serialized, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if len(serialized) > MaxJSONBytes {
				t.Fatal("bounded projection is too large")
			}
			decoded, err := DecodeMap(serialized)
			if err != nil || !reflect.DeepEqual(m, decoded) {
				t.Fatalf("map cannot round-trip through strict decoder: %v", err)
			}
		})
	}
}

func TestProjectionStableRegardlessOfValidatedPlanOrder(t *testing.T) {
	plan, err := liveverify.NewPlan(liveverify.ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ProjectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(plan.Probes)-1; i < j; i, j = i+1, j-1 {
		plan.Probes[i], plan.Probes[j] = plan.Probes[j], plan.Probes[i]
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("source validates a reordered fixed route set: %v", err)
	}
	second, err := ProjectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("a reordered valid plan must not change the canonical topology")
	}
	one, _ := json.Marshal(first)
	two, _ := json.Marshal(second)
	if !bytes.Equal(one, two) {
		t.Fatal("deterministic JSON output differed")
	}
}

func TestProjectionRefusesAlteredOrUnownedPlans(t *testing.T) {
	tests := []struct {
		name string
		edit func(*liveverify.Plan)
	}{
		{"wrong cluster", func(p *liveverify.Plan) { p.Cluster = "production" }},
		{"wrong context", func(p *liveverify.Plan) { p.Context = "kind-other" }},
		{"wrong profile", func(p *liveverify.Plan) { p.Profile = "production" }},
		{"wrong endpoint mode", func(p *liveverify.Plan) { p.EndpointMode = "service-dns" }},
		{"wrong port", func(p *liveverify.Plan) { p.Port = 443 }},
		{"modified time budget", func(p *liveverify.Plan) { p.Limits.MaxAttemptsUnreachable++ }},
		{"unapproved route", func(p *liveverify.Plan) { p.Probes[0].ID = "edge-to-internet" }},
		{"unapproved zone", func(p *liveverify.Plan) { p.Probes[0].Source = "outside" }},
		{"modified expected outcome", func(p *liveverify.Plan) { p.Probes[2].Expected = liveverify.ExpectReachable }},
		{"missing route", func(p *liveverify.Plan) { p.Probes = p.Probes[:5] }},
		{"duplicate route", func(p *liveverify.Plan) { p.Probes[5] = p.Probes[0] }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := liveverify.NewPlan(liveverify.ProfileHardened)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(&p)
			m, err := ProjectPlan(p)
			if err == nil {
				t.Fatalf("unsafe plan yielded a map: %+v", m)
			}
			if !reflect.DeepEqual(m, Map{}) {
				t.Fatal("failed projection must return a zero map")
			}
		})
	}
	if _, err := NewDeclaredMap("production"); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func TestProjectionDoesNotAliasResultOrMutatePlan(t *testing.T) {
	plan, err := liveverify.NewPlan(liveverify.ProfileBaseline)
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := json.Marshal(plan)
	first, err := ProjectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	first.Nodes[0].Namespace = "prod"
	first.Routes[0].Reason = "fabricated result"
	first.Limitations[0] = ""
	second, err := ProjectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if second.Nodes[0].Namespace != "firedoor-edge" || !strings.Contains(second.Routes[0].Reason, "planning-only") || second.Limitations[0] == "" {
		t.Fatal("returned map aliases reusable plan or global state")
	}
	current, _ := json.Marshal(plan)
	if !bytes.Equal(initial, current) {
		t.Fatal("projection changed its input plan")
	}
}
