package prooflens

import (
	"crypto/sha256"
	"encoding/hex"
)

func syntheticHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// SyntheticExample is NOT derived from a lab or real evidence. It is only for
// exercising deterministic contracts and unit tests.
func SyntheticExample() Bundle {
	const runID = "RUN-SYNTHETIC-0001"
	const timeUTC = "2026-01-01T00:00:00Z"
	source := func(name string) SourceSummary {
		return SourceSummary{Kind: "synthetic_fixture", Reference: "synthetic/example/" + name, SHA256: syntheticHash(name)}
	}
	return Bundle{
		SchemaVersion: BundleSchema,
		Run: Run{
			SchemaVersion: RunSchema, RunID: runID, ScenarioID: "FD-004",
			GitSHA:  "1111111111111111111111111111111111111111",
			Profile: "synthetic", ClusterFingerprint: syntheticHash("fictional-cluster"),
			TerraformPlanSHA256: syntheticHash("fictional-plan"), StartedUTC: timeUTC,
			EvidenceIDs: []string{"EV-SYNTHETIC-DECLARED", "EV-SYNTHETIC-OBSERVED", "EV-SYNTHETIC-TESTED"},
			Verdict:     Inconclusive, Limitations: []string{"Synthetic fixture only; no actual lab run."},
		},
		Evidence: []Evidence{
			{SchemaVersion: EvidenceSchema, EvidenceID: "EV-SYNTHETIC-DECLARED", RunID: runID,
				Stage: Declared, Kind: "iac_declaration", Source: source("declared"),
				CapturedUTC: timeUTC, Summary: "Synthetic Terraform network boundary intent", Expected: "deny-edge-to-data"},
			{SchemaVersion: EvidenceSchema, EvidenceID: "EV-SYNTHETIC-OBSERVED", RunID: runID,
				Stage: Observed, Kind: "kubernetes_state", Source: source("observed"),
				CapturedUTC: timeUTC, Summary: "Synthetic Kubernetes NetworkPolicy summary", Observed: "policy-present"},
			{SchemaVersion: EvidenceSchema, EvidenceID: "EV-SYNTHETIC-TESTED", RunID: runID,
				Stage: Tested, Kind: "network_test", Source: source("tested"),
				CapturedUTC: timeUTC, Summary: "Synthetic probe outcome; not real traffic", Expected: "unreachable", Observed: "unreachable"},
		},
		Proof: Proof{SchemaVersion: ProofSchema, ProofID: "PROOF-SYNTHETIC-0001", RunID: runID,
			Claim: "Synthetic reference to an expected network boundary", DeclaredEvidenceID: "EV-SYNTHETIC-DECLARED",
			ObservedEvidenceID: "EV-SYNTHETIC-OBSERVED", TestedEvidenceID: "EV-SYNTHETIC-TESTED",
			Verdict: Inconclusive, Limitations: []string{"No real enforcement or provenance verified."}},
	}
}
