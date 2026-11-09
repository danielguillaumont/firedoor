package containmentmap

import (
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

// NewDeclaredMap builds a deterministic, declared-only topology from the
// already-reviewed v0.3 six-route plan. It does not read Terraform state,
// Kubernetes resources, evidence files, or network sockets.
//
// A probe-plan expectation is NOT a Terraform-backed declared intent in the
// v0.7 Map schema. Until a validated Terraform receipt is attached by a later
// importer, Declared must remain "unspecified" for every route.
func NewDeclaredMap(profile liveverify.Profile) (Map, error) {
	plan, err := liveverify.NewPlan(profile)
	if err != nil {
		return Map{}, fmt.Errorf("create scoped probe plan: %w", err)
	}
	return ProjectPlan(plan)
}

// ProjectPlan validates an existing immutable-scope v0.3 plan before deriving
// zone nodes and six canonical route edges. Input probe order is deliberately
// irrelevant: the output follows liveverify.NewPlan's reviewed ordering.
//
// Neither a correct expected outcome nor a generated zone node is evidence of
// observed traffic, an actual Pod, or Kubernetes NetworkPolicy enforcement.
func ProjectPlan(plan liveverify.Plan) (Map, error) {
	if err := plan.Validate(); err != nil {
		return Map{}, fmt.Errorf("refuse invalid probe plan: %w", err)
	}
	canonical, err := liveverify.NewPlan(plan.Profile)
	if err != nil {
		return Map{}, fmt.Errorf("create canonical probe plan: %w", err)
	}

	byID := make(map[string]liveverify.Probe, len(plan.Probes))
	for _, probe := range plan.Probes {
		byID[probe.ID] = probe
	}
	m := Map{
		SchemaVersion: SchemaVersion,
		Cluster:       OwnedCluster,
		Context:       OwnedContext,
		Profile:       string(plan.Profile),
		EndpointMode:  EndpointMode,
		Transport:     Transport,
		Port:          Port,
		Source:        "declared_contract",
		Nodes: []Node{
			{ID: "edge", Kind: "zone", Zone: "edge", Namespace: "firedoor-edge"},
			{ID: "app", Kind: "zone", Zone: "app", Namespace: "firedoor-app"},
			{ID: "data", Kind: "zone", Zone: "data", Namespace: "firedoor-data"},
		},
		Routes:   make([]Route, 0, len(canonical.Probes)),
		Evidence: make([]EvidenceRef, 0),
		Limitations: []string{
			"Zone nodes and routes are a validated fixed lab plan, not a current Kubernetes workload inventory.",
			"Probe expectations are planning-only; no Terraform plan receipt establishes declared policy intent.",
			"No network observations or enforcement checks were performed; every route remains not assessed.",
		},
	}
	for _, expected := range canonical.Probes {
		probe, exists := byID[expected.ID]
		if !exists || probe != expected {
			return Map{}, fmt.Errorf("validated plan differs from canonical route %q", expected.ID)
		}
		m.Routes = append(m.Routes, Route{
			ID:         probe.ID,
			From:       string(probe.Source),
			To:         string(probe.Destination),
			Declared:   "unspecified",
			Observed:   "not_observed",
			Assessment: "not_assessed",
			Reason: fmt.Sprintf(
				"The validated v0.3 plan expects %s HTTP/TCP connectivity. This is planning-only, not a Terraform-backed declaration, measured result, or enforcement proof.",
				probe.Expected,
			),
		})
	}
	if err := m.Validate(); err != nil {
		return Map{}, fmt.Errorf("generated topology failed v0.7 validation: %w", err)
	}
	return m, nil
}
