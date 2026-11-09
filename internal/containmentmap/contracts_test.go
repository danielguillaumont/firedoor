package containmentmap

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func declaredFixture() Map {
	return Map{
		SchemaVersion: SchemaVersion,
		Cluster:       OwnedCluster, Context: OwnedContext, Profile: "hardened",
		Source: "declared_contract", EndpointMode: EndpointMode, Transport: Transport, Port: Port, CurrentInventory: false, EnforcementVerified: false,
		Nodes: []Node{
			{ID: "edge", Kind: "zone", Zone: "edge", Namespace: "firedoor-edge"},
			{ID: "data", Kind: "zone", Zone: "data", Namespace: "firedoor-data"},
		},
		Routes: []Route{{
			ID: "edge-to-data", From: "edge", To: "data", Declared: "unspecified",
			Observed: "not_observed", Assessment: "not_assessed",
			Reason: "Planning contract only; no traffic measurement or enforcement assessment.",
		}},
		Limitations: []string{"Topology is a planning contract, not live network inventory or enforcement proof."},
	}
}

func captured() time.Time { return time.Date(2026, 11, 5, 17, 22, 35, 0, time.UTC) }

func addPlan(m *Map) {
	m.Evidence = append(m.Evidence, EvidenceRef{
		ID: "plan-1", Kind: "terraform_plan", SourceID: "TFPLAN-20261105-01",
		CapturedAtUTC: captured(), Profile: m.Profile,
	})
	m.Routes[0].Declared = "expected_unreachable"
	m.Routes[0].DeclaredEvidenceID = "plan-1"
}

func addObservation(m *Map) {
	m.Source = "historical_projection"
	m.Evidence = append(m.Evidence, EvidenceRef{
		ID: "probe-1", Kind: "network_observation", SourceID: "probe-edge-data-01",
		CapturedAtUTC: captured(), Profile: m.Profile, Result: "unreachable",
	})
	m.Routes[0].Observed = "unreachable"
	m.Routes[0].ObservationEvidenceIDs = []string{"probe-1"}
	m.Routes[0].Assessment = "inconclusive"
	m.Routes[0].Reason = "An unreachable historical route does not establish CNI or policy causation."
}

func TestValidContractsDoNotClaimProof(t *testing.T) {
	cases := []struct {
		name    string
		makeMap func() Map
	}{
		{"declared unknown", func() Map { return declaredFixture() }},
		{"plan intent only", func() Map { m := declaredFixture(); addPlan(&m); return m }},
		{"historical network observation", func() Map { m := declaredFixture(); addPlan(&m); addObservation(&m); return m }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.makeMap()
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
			if m.CurrentInventory || m.EnforcementVerified {
				t.Fatal("contract must not assert live inventory or enforcement")
			}
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeMap(b)
			if err != nil {
				t.Fatal(err)
			}
			if got.Routes[0].Assessment != m.Routes[0].Assessment {
				t.Fatal("JSON assessment changed")
			}
		})
	}
}

func TestRejectsInvalidSecurityAndProvenance(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*Map)
	}{
		{"wrong schema", func(m *Map) { m.SchemaVersion = "firedoor.v00" }},
		{"wrong cluster", func(m *Map) { m.Cluster = "production" }},
		{"wrong kube context", func(m *Map) { m.Context = "kind-other" }},
		{"wrong profile", func(m *Map) { m.Profile = "unknown" }},
		{"unsupported endpoint mode", func(m *Map) { m.EndpointMode = "service-dns" }},
		{"unsupported transport", func(m *Map) { m.Transport = "UDP" }},
		{"unsupported target port", func(m *Map) { m.Port = 443 }},
		{"invented source mode", func(m *Map) { m.Source = "live_scan" }},
		{"current inventory claim", func(m *Map) { m.CurrentInventory = true }},
		{"enforcement claim", func(m *Map) { m.EnforcementVerified = true }},
		{"missing limitations", func(m *Map) { m.Limitations = nil }},
		{"blank limitations", func(m *Map) { m.Limitations = []string{"  "} }},
		{"oversized limitation", func(m *Map) { m.Limitations = []string{strings.Repeat("a", 251)} }},
		{"empty nodes", func(m *Map) { m.Nodes = nil }},
		{"oversized nodes", func(m *Map) { m.Nodes = make([]Node, MaxNodes+1) }},
		{"empty routes", func(m *Map) { m.Routes = nil }},
		{"oversized routes", func(m *Map) { m.Routes = make([]Route, MaxRoutes+1) }},
		{"invalid node id", func(m *Map) { m.Nodes[0].ID = "../edge" }},
		{"wrong namespace", func(m *Map) { m.Nodes[0].Namespace = "kube-system" }},
		{"invalid node kind", func(m *Map) { m.Nodes[0].Kind = "external_ip" }},
		{"zone with workload", func(m *Map) { m.Nodes[0].Workload = "firedoor-edge" }},
		{"invalid workload", func(m *Map) { m.Nodes[0].Kind = "workload"; m.Nodes[0].Workload = "attacker" }},
		{"duplicate node id", func(m *Map) { m.Nodes[1].ID = "edge" }},
		{"duplicate identity", func(m *Map) {
			m.Nodes = append(m.Nodes, Node{ID: "edge-2", Kind: "zone", Zone: "edge", Namespace: "firedoor-edge"})
		}},
		{"orphan node", func(m *Map) {
			m.Nodes = append(m.Nodes, Node{ID: "app", Kind: "zone", Zone: "app", Namespace: "firedoor-app"})
		}},
		{"unknown route", func(m *Map) { m.Routes[0].ID = "edge-to-internet" }},
		{"duplicate route", func(m *Map) { m.Routes = append(m.Routes, m.Routes[0]) }},
		{"wrong route zone", func(m *Map) { m.Routes[0].ID = "app-to-data" }},
		{"dangling source", func(m *Map) { m.Routes[0].From = "missing" }},
		{"dangling target", func(m *Map) { m.Routes[0].To = "missing" }},
		{"empty reason", func(m *Map) { m.Routes[0].Reason = " " }},
		{"oversized reason", func(m *Map) { m.Routes[0].Reason = strings.Repeat("x", 351) }},
		{"unbacked expected outcome", func(m *Map) { m.Routes[0].Declared = "expected_unreachable" }},
		{"invented intent enum", func(m *Map) { m.Routes[0].Declared = "enforced" }},
		{"unspecified intent with evidence", func(m *Map) { m.Routes[0].DeclaredEvidenceID = "plan-1" }},
		{"unbacked reachable result", func(m *Map) { m.Routes[0].Observed = "reachable" }},
		{"invented verification result", func(m *Map) { m.Routes[0].Assessment = "verified_contained" }},
		{"unknown observed enum", func(m *Map) { m.Routes[0].Observed = "blocked_by_policy" }},
		{"unexpected observed references", func(m *Map) { m.Routes[0].ObservationEvidenceIDs = []string{"fake"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := declaredFixture()
			tc.alter(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("validation unexpectedly succeeded")
			}
		})
	}
}

func TestEvidenceRejectsContradictions(t *testing.T) {
	cases := []struct {
		name  string
		alter func(*Map)
	}{
		{"missing recorded evidence", func(m *Map) { m.Routes[0].ObservationEvidenceIDs = []string{"missing"} }},
		{"cross profile", func(m *Map) { m.Evidence[len(m.Evidence)-1].Profile = "baseline" }},
		{"conflicting result", func(m *Map) { m.Evidence[len(m.Evidence)-1].Result = "reachable" }},
		{"duplicate observation ref", func(m *Map) { m.Routes[0].ObservationEvidenceIDs = []string{"probe-1", "probe-1"} }},
		{"non-network evidence ref", func(m *Map) { m.Routes[0].ObservationEvidenceIDs = []string{"plan-1"} }},
		{"non-UTC timestamp", func(m *Map) { m.Evidence[0].CapturedAtUTC = captured().In(time.FixedZone("EST", -5*60*60)) }},
		{"zero timestamp", func(m *Map) { m.Evidence[0].CapturedAtUTC = time.Time{} }},
		{"file path as source", func(m *Map) { m.Evidence[0].SourceID = "C:/private/report.json" }},
		{"unrecognized kind", func(m *Map) { m.Evidence[0].Kind = "kubeconfig" }},
		{"invalid network evidence outcome", func(m *Map) { m.Evidence[1].Result = "policy_enforced" }},
		{"duplicate evidence id", func(m *Map) { m.Evidence = append(m.Evidence, m.Evidence[0]) }},
		{"unreferenced evidence", func(m *Map) {
			m.Evidence = append(m.Evidence, EvidenceRef{ID: "unused", Kind: "terraform_plan", SourceID: "orphan", Profile: "hardened", CapturedAtUTC: captured()})
		}},
		{"too many evidence records", func(m *Map) { m.Evidence = make([]EvidenceRef, MaxEvidence+1) }},
		{"too many route refs", func(m *Map) { m.Routes[0].ObservationEvidenceIDs = make([]string, MaxEvidencePerRoute+1) }},
		{"non-inconclusive assessed outcome", func(m *Map) { m.Routes[0].Assessment = "not_assessed" }},
		{"missing node snapshot record", func(m *Map) { m.Nodes[0].Kind = "workload"; m.Nodes[0].Workload = "firedoor-edge" }},
		{"fabricated node snapshot ref", func(m *Map) {
			m.Nodes[0].Kind = "workload"
			m.Nodes[0].Workload = "firedoor-edge"
			m.Nodes[0].ObservationEvidenceID = "probe-1"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := declaredFixture()
			addPlan(&m)
			addObservation(&m)
			tc.alter(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("validation unexpectedly succeeded")
			}
		})
	}
}

func TestHistoricalWorkloadRequiresScopedSnapshotEvidence(t *testing.T) {
	m := declaredFixture()
	m.Source = "historical_projection"
	m.Nodes[0].Kind = "workload"
	m.Nodes[0].Workload = "firedoor-edge"
	m.Nodes[0].ObservationEvidenceID = "snapshot-1"
	m.Evidence = []EvidenceRef{{ID: "snapshot-1", Kind: "kubernetes_snapshot", SourceID: "snapshot-01", Profile: "hardened", CapturedAtUTC: captured()}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	m.Source = "declared_contract"
	if err := m.Validate(); err == nil {
		t.Fatal("declared-only topology must reject observed workload provenance")
	}
}

func TestDecodeMapFailsClosed(t *testing.T) {
	valid, _ := json.Marshal(declaredFixture())
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"oversize", []byte(strings.Repeat("x", MaxJSONBytes+1))},
		{"trailing document", append(append([]byte(nil), valid...), []byte(` {}`)...)},
		{"unknown top-level key", append(append([]byte(nil), valid[:len(valid)-1]...), []byte(`,"secret":true}`)...)},
		{"unknown nested key", []byte(strings.Replace(string(valid), `"kind":"zone"`, `"kind":"zone","ip":"1.2.3.4"`, 1))},
		{"truncated", valid[:len(valid)-1]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeMap(tc.data); err == nil {
				t.Fatal("decoder unexpectedly accepted invalid JSON")
			}
		})
	}
}
