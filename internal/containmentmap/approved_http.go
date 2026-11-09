package containmentmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

const (
	// This fixed path is covered by the repository's Git ignore rule.
	ApprovedHistoricalMapRelativePath = "docs/evidence/local/v07-containment-map-saved-v03-suite.json"
	ApprovedHistoricalMapConsent      = "I-APPROVE-FIREDOOR-V07-LOCAL-HISTORICAL-MAP"
)

var errHistoricalHTTPRejected = errors.New("approved historical Containment Map unavailable or rejected; no server started")

// LoadApprovedHistoricalMapForHTTP may be called only as an explicit, pre-listen
// CLI startup choice. No HTTP request may read source files. A successful
// internal-consistency check DOES NOT authenticate file origin, source profile,
// Terraform application or network policy enforcement.
func LoadApprovedHistoricalMapForHTTP(repoRoot string, profile liveverify.Profile, consent string) (Map, error) {
	if consent != ApprovedHistoricalMapConsent || !validProfile(string(profile)) {
		return Map{}, errHistoricalHTTPRejected
	}
	// Refuse any symlink in the fixed input chain. Never allow a client- or
	// command-selected file path. In particular, don't follow a link outside
	// the Git-ignored evidence directory.
	path := repoRoot
	for _, part := range []string{"docs", "evidence", "local", "v07-containment-map-saved-v03-suite.json"} {
		path = filepath.Join(path, part)
		stat, err := os.Lstat(path)
		if err != nil || stat.Mode()&os.ModeSymlink != 0 {
			return Map{}, errHistoricalHTTPRejected
		}
		if part == "v07-containment-map-saved-v03-suite.json" {
			if !stat.Mode().IsRegular() {
				return Map{}, errHistoricalHTTPRejected
			}
		} else if !stat.IsDir() {
			return Map{}, errHistoricalHTTPRejected
		}
	}
	historical, err := ProjectV03HistoricalFile(path, profile)
	if err != nil {
		return Map{}, errHistoricalHTTPRejected
	}
	safe, err := PrepareHistoricalHTTPProjection(historical)
	if err != nil {
		return Map{}, errHistoricalHTTPRejected
	}
	return safe, nil
}

// PrepareHistoricalHTTPProjection emits a deliberately smaller contract than
// the offline adapter: fixed zone identities, canonical routes, recorded
// reachability and capture time, a synthetic opaque source label, and fixed
// interpretation text. Raw suite bytes, original identifiers and content
// fingerprints never enter the HTTP response. No attestation is implied.
func PrepareHistoricalHTTPProjection(input Map) (Map, error) {
	rejected := errors.New("historical HTTP map is outside the fixed reviewed projection")
	if err := input.Validate(); err != nil || input.Source != "historical_projection" ||
		input.CurrentInventory || input.EnforcementVerified || len(input.Nodes) != 3 || len(input.Routes) != 6 {
		return Map{}, rejected
	}
	canonical, err := NewDeclaredMap(liveverify.Profile(input.Profile))
	if err != nil {
		return Map{}, rejected
	}
	out := canonical
	out.Source = "historical_projection"
	out.Limitations = []string{
		"Approved saved v0.3 observation only, not current inventory or a live network scan.",
		"Original file origin and operator-supplied profile are not independently authenticated.",
		"Point-in-time HTTP connectivity does not prove NetworkPolicy causation or Terraform application.",
		"Unresolved routes are not observed or assessed; observed routes remain inconclusive for enforcement.",
		"Evidence source labels are deliberately generic; timestamps are historical capture times.",
	}
	for i, n := range input.Nodes {
		if n != canonical.Nodes[i] {
			return Map{}, rejected
		}
	}
	seenEvidence := make(map[string]EvidenceRef, len(input.Evidence))
	for _, e := range input.Evidence {
		if e.Kind != "network_observation" || e.Profile != input.Profile {
			return Map{}, rejected
		}
		seenEvidence[e.ID] = e
	}
	plan, err := liveverify.NewPlan(liveverify.Profile(input.Profile))
	if err != nil {
		return Map{}, rejected
	}
	for i, r := range input.Routes {
		fixed := canonical.Routes[i]
		if r.ID != fixed.ID || r.From != fixed.From || r.To != fixed.To ||
			r.Declared != "unspecified" || r.DeclaredEvidenceID != "" {
			return Map{}, rejected
		}
		switch r.Observed {
		case "not_observed":
			if r.Assessment != "not_assessed" || len(r.ObservationEvidenceIDs) != 0 {
				return Map{}, rejected
			}
			out.Routes[i].Reason = "Saved v0.3 probe did not establish reliable reachability. This route remains unobserved and not assessed."
		case "reachable", "unreachable":
			id := "observation-" + r.ID
			if r.Assessment != "inconclusive" || len(r.ObservationEvidenceIDs) != 1 || r.ObservationEvidenceIDs[0] != id {
				return Map{}, rejected
			}
			evidence, ok := seenEvidence[id]
			if !ok || evidence.Result != r.Observed {
				return Map{}, rejected
			}
			out.Routes[i].Observed = r.Observed
			out.Routes[i].Assessment = "inconclusive"
			out.Routes[i].ObservationEvidenceIDs = []string{id}
			out.Routes[i].Reason = fmt.Sprintf("Saved v0.3 HTTP/TCP observation recorded %s against a %s probe-plan expectation. Historical connectivity is not verified policy enforcement.", r.Observed, plan.Probes[i].Expected)
			out.Evidence = append(out.Evidence, EvidenceRef{
				ID: id, Kind: "network_observation", SourceID: "approved-local-v03",
				CapturedAtUTC: evidence.CapturedAtUTC, Profile: input.Profile, Result: r.Observed,
			})
		default:
			return Map{}, rejected
		}
	}
	if len(out.Evidence) != len(input.Evidence) {
		return Map{}, rejected
	}
	if err := out.Validate(); err != nil {
		return Map{}, rejected
	}
	data, err := json.Marshal(out)
	if err != nil || len(data) == 0 || len(data) > MaxJSONBytes {
		return Map{}, rejected
	}
	return out, nil
}
