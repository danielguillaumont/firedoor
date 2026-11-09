package containmentmap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

// ProjectV03HistoricalObservation converts one already-captured, owned v0.3
// observation suite into a strictly bounded, historical-only topology. It
// verifies the saved data against the canonical six-route plan, but neither
// authenticates the original capture nor establishes NetworkPolicy causality.
//
// This function only accepts an in-memory value. It never reads files, queries
// Kubernetes/Terraform, runs probes, exposes IPs or enables HTTP routes.
func ProjectV03HistoricalObservation(suite liveverify.SuiteObservation) (Map, error) {
	plan, err := liveverify.NewPlan(suite.Profile)
	if err != nil {
		return Map{}, fmt.Errorf("historical observation has unsupported profile: %w", err)
	}
	// Verify validates fixed cluster/context, scoped Pod ownership, attempt
	// timestamps, exact HTTP markers, repeated failures and the suite status.
	// A match is only consistency with a probe expectation, not enforcement.
	report, err := liveverify.Verify(plan, suite)
	if err != nil {
		return Map{}, fmt.Errorf("refuse inconsistent v0.3 historical observation: %w", err)
	}
	base, err := ProjectPlan(plan)
	if err != nil {
		return Map{}, fmt.Errorf("cannot construct canonical topology: %w", err)
	}

	// Hash canonical in-memory JSON as a stable content label only. This is
	// NOT the original file's byte-level SHA-256 and NOT an attestation.
	raw, err := json.Marshal(suite)
	if err != nil || len(raw) == 0 || len(raw) > liveverify.MaxObservationBytes {
		return Map{}, fmt.Errorf("historical observation exceeds fixed input bound or cannot be encoded")
	}
	digest := sha256.Sum256(raw)
	sourceID := "v03-suite-" + hex.EncodeToString(digest[:])

	base.Source = "historical_projection"
	base.Limitations = []string{
		"Topology contains fixed Firedoor lab zones and routes, not a current Kubernetes workload inventory.",
		"Saved HTTP observations are consistency-checked, not independently authenticated or attributed to NetworkPolicy.",
		"The observed profile is operator-supplied; this adapter does not verify Terraform application or state.",
		"Inconclusive probe outcomes remain not observed and not assessed, even if attempts were recorded.",
		"The canonical suite content fingerprint is only an identifier, not an attestation or source-file hash.",
	}

	// Verification and suite share the canonical fixed plan order. Do not
	// expose Pod names/IPs, raw request bodies or the original evidence file.
	if len(report.Routes) != len(base.Routes) || len(suite.Routes) != len(base.Routes) {
		return Map{}, fmt.Errorf("historical route cardinality differs from the canonical plan")
	}
	for i, observed := range report.Routes {
		r := &base.Routes[i]
		original := suite.Routes[i]
		if r.ID != observed.ID || r.ID != original.ID {
			return Map{}, fmt.Errorf("historical route ordering disagrees with fixed plan")
		}
		if observed.Observed == liveverify.ObservedInconclusive {
			r.Reason = "Saved v0.3 probe outcome was inconclusive. No reliable reachability result was established for this route; it remains not assessed."
			continue
		}
		if observed.Observed != liveverify.ObservedReachable && observed.Observed != liveverify.ObservedUnreachable {
			return Map{}, fmt.Errorf("historical route %q has unsupported outcome", r.ID)
		}
		if len(original.Attempts) == 0 {
			return Map{}, fmt.Errorf("historical route %q has no supporting attempts", r.ID)
		}

		id := "observation-" + r.ID
		base.Evidence = append(base.Evidence, EvidenceRef{
			ID:            id,
			Kind:          "network_observation",
			SourceID:      sourceID,
			CapturedAtUTC: original.Attempts[len(original.Attempts)-1].AtUTC.UTC(),
			Profile:       string(suite.Profile),
			Result:        string(observed.Observed),
		})
		r.Observed = string(observed.Observed)
		r.ObservationEvidenceIDs = []string{id}
		r.Assessment = "inconclusive"
		r.Reason = fmt.Sprintf(
			"Saved v0.3 HTTP probe observed %s (%s against the fixed %s expectation). This historical result does not prove Terraform state or NetworkPolicy causality.",
			observed.Observed, observed.Verdict, observed.Expected,
		)
	}

	if err := base.Validate(); err != nil {
		return Map{}, fmt.Errorf("historical topology validation failed: %w", err)
	}
	encoded, err := json.Marshal(base)
	if err != nil || len(encoded) > MaxJSONBytes {
		return Map{}, fmt.Errorf("historical topology exceeds fixed JSON output bound")
	}
	return base, nil
}
