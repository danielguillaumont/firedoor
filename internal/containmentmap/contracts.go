// Package containmentmap defines strictly scoped, read-only topology contracts.
//
// Slice 7A is pure data validation: it never queries a cluster, reads private
// evidence, serves HTTP, runs probes, or verifies NetworkPolicy enforcement.
package containmentmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	SchemaVersion       = "firedoor.v07.containment-map.v1"
	OwnedCluster        = "firedoor-local"
	OwnedContext        = "kind-firedoor-local"
	EndpointMode        = "direct-pod-ip"
	Transport           = "HTTP/TCP"
	Port                = 8080
	MaxNodes            = 24
	MaxRoutes           = 6
	MaxEvidence         = 32
	MaxEvidencePerRoute = 8
	MaxJSONBytes        = 32 * 1024
)

// The six scoped zone-to-zone routes are the same fixed routes as the v0.3
// liveverify plan. Arbitrary addresses, ports, namespaces, or routes cannot
// enter this data contract.
var allowedRoutes = map[string][2]string{
	"edge-to-app":  {"edge", "app"},
	"app-to-data":  {"app", "data"},
	"edge-to-data": {"edge", "data"},
	"data-to-app":  {"data", "app"},
	"app-to-edge":  {"app", "edge"},
	"data-to-edge": {"data", "edge"},
}

var labelRE = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
var evidenceRefRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func validLabel(s string) bool   { return labelRE.MatchString(s) }
func validZone(s string) bool    { return s == "edge" || s == "app" || s == "data" }
func validProfile(s string) bool { return s == "baseline" || s == "hardened" }

// Node records a declared zone or an owned workload identity, not a detected
// runtime Pod. There are no arbitrary IP-address or URL fields.
type Node struct {
	ID                    string `json:"id"`
	Kind                  string `json:"kind"` // zone | workload
	Zone                  string `json:"zone"`
	Namespace             string `json:"namespace"`
	Workload              string `json:"workload,omitempty"`
	ObservationEvidenceID string `json:"observation_evidence_id,omitempty"`
}

func (n Node) Validate() error {
	if !validLabel(n.ID) || !validZone(n.Zone) || n.Namespace != "firedoor-"+n.Zone {
		return fmt.Errorf("invalid or out-of-scope map node %q", n.ID)
	}
	switch n.Kind {
	case "zone":
		if n.Workload != "" {
			return fmt.Errorf("zone node %q must not claim a workload", n.ID)
		}
	case "workload":
		if !validLabel(n.Workload) || n.Workload != "firedoor-"+n.Zone {
			return fmt.Errorf("workload node %q is outside the fixed lab", n.ID)
		}
	default:
		return fmt.Errorf("invalid node kind in %q", n.ID)
	}
	return nil
}

// EvidenceRef points to an already-recorded, scoped artifact. It is not a
// filename, a URL, a SHA-256 authentication claim, or evidence generated here.
// A structural validation check does NOT establish authenticity or causality.
type EvidenceRef struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`      // terraform_plan | kubernetes_snapshot | network_observation | historical_witness
	SourceID      string    `json:"source_id"` // bounded, opaque record identifier
	CapturedAtUTC time.Time `json:"captured_at_utc"`
	Profile       string    `json:"profile"`
	Result        string    `json:"result,omitempty"` // reachable | unreachable for network evidence
}

func (e EvidenceRef) Validate() error {
	if !validLabel(e.ID) || !evidenceRefRE.MatchString(e.SourceID) || !validProfile(e.Profile) ||
		e.CapturedAtUTC.IsZero() || e.CapturedAtUTC.Location() != time.UTC {
		return fmt.Errorf("invalid provenance for evidence %q", e.ID)
	}
	switch e.Kind {
	case "terraform_plan", "kubernetes_snapshot":
		if e.Result != "" {
			return fmt.Errorf("non-network evidence %q cannot claim reachability", e.ID)
		}
	case "network_observation", "historical_witness":
		if e.Result != "reachable" && e.Result != "unreachable" {
			return fmt.Errorf("network evidence %q must give a measured reachability result", e.ID)
		}
	default:
		return fmt.Errorf("unsupported evidence class %q", e.Kind)
	}
	return nil
}

// Route distinguishes planning intent, point-in-time connectivity observation,
// and supported interpretation. No route in this schema may be marked as
// verified containment or verified policy enforcement.
type Route struct {
	ID                     string   `json:"id"`   // one of the six existing owned lab routes
	From                   string   `json:"from"` // node ID, not an arbitrary endpoint
	To                     string   `json:"to"`
	Declared               string   `json:"declared"` // unspecified | expected_reachable | expected_unreachable
	DeclaredEvidenceID     string   `json:"declared_evidence_id,omitempty"`
	Observed               string   `json:"observed"` // not_observed | reachable | unreachable
	ObservationEvidenceIDs []string `json:"observation_evidence_ids,omitempty"`
	Assessment             string   `json:"assessment"` // not_assessed | inconclusive
	Reason                 string   `json:"reason"`
}

// Map is one profile's declared or historical topology projection. It cannot
// represent an active inventory, current drift, or an enforcement verdict.
type Map struct {
	SchemaVersion       string        `json:"schema_version"`
	Cluster             string        `json:"cluster"`
	Context             string        `json:"context"`
	Profile             string        `json:"profile"`
	EndpointMode        string        `json:"endpoint_mode"`
	Transport           string        `json:"transport"`
	Port                int           `json:"port"`
	Source              string        `json:"source"` // declared_contract | historical_projection
	CurrentInventory    bool          `json:"current_inventory"`
	EnforcementVerified bool          `json:"enforcement_verified"`
	Nodes               []Node        `json:"nodes"`
	Routes              []Route       `json:"routes"`
	Evidence            []EvidenceRef `json:"evidence"`
	Limitations         []string      `json:"limitations"`
}

// Validate checks bounds, identity, provenance and edge/evidence consistency.
// It does not authenticate evidence, query Kubernetes, or determine policy
// enforcement. Only a separate, guarded verifier can establish such claims.
func (m Map) Validate() error {
	if m.SchemaVersion != SchemaVersion || m.Cluster != OwnedCluster || m.Context != OwnedContext || !validProfile(m.Profile) || m.EndpointMode != EndpointMode || m.Transport != Transport || m.Port != Port {
		return fmt.Errorf("map schema or dedicated lab scope invalid")
	}
	if m.Source != "declared_contract" && m.Source != "historical_projection" {
		return fmt.Errorf("map source is not an approved declaration or historical projection")
	}
	if m.CurrentInventory || m.EnforcementVerified {
		return fmt.Errorf("Slice 7A cannot assert current inventory or enforcement verification")
	}
	if len(m.Nodes) < 2 || len(m.Nodes) > MaxNodes || len(m.Routes) < 1 || len(m.Routes) > MaxRoutes || len(m.Evidence) > MaxEvidence {
		return fmt.Errorf("topology array sizes outside fixed bounds")
	}
	if len(m.Limitations) == 0 || len(m.Limitations) > 8 {
		return fmt.Errorf("map must include bounded security limitations")
	}
	for _, limitation := range m.Limitations {
		if strings.TrimSpace(limitation) == "" || len(limitation) > 250 {
			return fmt.Errorf("empty or oversized limitation")
		}
	}
	nodes := make(map[string]Node, len(m.Nodes))
	identities := make(map[string]bool, len(m.Nodes))
	for _, n := range m.Nodes {
		if err := n.Validate(); err != nil {
			return err
		}
		if _, duplicate := nodes[n.ID]; duplicate {
			return fmt.Errorf("duplicate node %q", n.ID)
		}
		identity := n.Kind + "/" + n.Namespace + "/" + n.Workload
		if identities[identity] {
			return fmt.Errorf("duplicate map node identity %q", identity)
		}
		identities[identity] = true
		nodes[n.ID] = n
	}
	evidence := make(map[string]EvidenceRef, len(m.Evidence))
	for _, e := range m.Evidence {
		if err := e.Validate(); err != nil {
			return err
		}
		if e.Profile != m.Profile {
			return fmt.Errorf("mixed baseline and hardened evidence %q", e.ID)
		}
		if _, duplicate := evidence[e.ID]; duplicate {
			return fmt.Errorf("duplicate evidence %q", e.ID)
		}
		evidence[e.ID] = e
	}
	routes := make(map[string]bool, len(m.Routes))
	usedNodes := make(map[string]bool, len(m.Nodes))
	referenced := make(map[string]bool, len(m.Evidence))
	for _, n := range m.Nodes {
		if m.Source == "historical_projection" && n.Kind == "workload" && n.ObservationEvidenceID == "" {
			return fmt.Errorf("historical workload %q needs Kubernetes provenance", n.ID)
		}
		if n.ObservationEvidenceID != "" {
			e, ok := evidence[n.ObservationEvidenceID]
			if !ok || e.Kind != "kubernetes_snapshot" || m.Source != "historical_projection" {
				return fmt.Errorf("node %q has missing or unsupported snapshot provenance", n.ID)
			}
			referenced[e.ID] = true
		}
	}
	for _, r := range m.Routes {
		zones, approved := allowedRoutes[r.ID]
		if !approved || routes[r.ID] {
			return fmt.Errorf("unapproved or duplicate route %q", r.ID)
		}
		routes[r.ID] = true
		source, sourceExists := nodes[r.From]
		target, targetExists := nodes[r.To]
		if !sourceExists || !targetExists || r.From == r.To || source.Zone != zones[0] || target.Zone != zones[1] {
			return fmt.Errorf("route %q has invalid zone mapping or dangling nodes", r.ID)
		}
		usedNodes[r.From] = true
		usedNodes[r.To] = true
		if len(r.Reason) == 0 || strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 350 {
			return fmt.Errorf("route %q requires a bounded interpretation", r.ID)
		}
		switch r.Declared {
		case "unspecified":
			if r.DeclaredEvidenceID != "" {
				return fmt.Errorf("unspecified route %q has unsupported intent evidence", r.ID)
			}
		case "expected_reachable", "expected_unreachable":
			e, ok := evidence[r.DeclaredEvidenceID]
			if !ok || e.Kind != "terraform_plan" {
				return fmt.Errorf("declared intent for %q requires a Terraform plan receipt", r.ID)
			}
			referenced[e.ID] = true
		default:
			return fmt.Errorf("route %q has invalid declared state", r.ID)
		}
		if len(r.ObservationEvidenceIDs) > MaxEvidencePerRoute {
			return fmt.Errorf("too many observations on %q", r.ID)
		}
		switch r.Observed {
		case "not_observed":
			if len(r.ObservationEvidenceIDs) != 0 || r.Assessment != "not_assessed" {
				return fmt.Errorf("unobserved route %q has fabricated observation or assessment", r.ID)
			}
		case "reachable", "unreachable":
			if m.Source != "historical_projection" || len(r.ObservationEvidenceIDs) == 0 || r.Assessment != "inconclusive" {
				return fmt.Errorf("observed route %q requires historical evidence and inconclusive assessment", r.ID)
			}
			seen := make(map[string]bool, len(r.ObservationEvidenceIDs))
			for _, id := range r.ObservationEvidenceIDs {
				e, ok := evidence[id]
				if !ok || (e.Kind != "network_observation" && e.Kind != "historical_witness") || e.Result != r.Observed || seen[id] {
					return fmt.Errorf("missing, duplicate or conflicting network evidence on %q", r.ID)
				}
				seen[id] = true
				referenced[id] = true
			}
		default:
			return fmt.Errorf("route %q has invalid observed state", r.ID)
		}
	}
	if m.Source == "declared_contract" {
		for _, r := range m.Routes {
			if r.Observed != "not_observed" {
				return fmt.Errorf("declared-only topology cannot contain observed traffic")
			}
		}
	}
	for id := range nodes {
		if !usedNodes[id] {
			return fmt.Errorf("unused topology node %q", id)
		}
	}
	for id := range evidence {
		if !referenced[id] {
			return fmt.Errorf("unreferenced evidence %q in topology", id)
		}
	}
	return nil
}

// DecodeMap enforces a small, exact JSON envelope before semantic validation.
// Future importers should use this path rather than json.Unmarshal directly.
// This does not authenticate the evidence source.
func DecodeMap(data []byte) (Map, error) {
	if len(data) == 0 || len(data) > MaxJSONBytes {
		return Map{}, fmt.Errorf("containment map JSON outside fixed 32 KiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var m Map
	if err := decoder.Decode(&m); err != nil {
		return Map{}, fmt.Errorf("invalid containment map JSON: %w", err)
	}
	var tail any
	if err := decoder.Decode(&tail); err != io.EOF {
		return Map{}, fmt.Errorf("trailing or invalid data after map JSON")
	}
	if err := m.Validate(); err != nil {
		return Map{}, err
	}
	return m, nil
}
