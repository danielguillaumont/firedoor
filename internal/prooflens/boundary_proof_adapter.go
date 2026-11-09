package prooflens

// This is a pure, narrowly scoped v0.5 -> v0.8 adapter. It never collects,
// opens, writes or authenticates evidence. Its source is a previously built
// detached v0.5 ProofReport, whose checksum establishes local consistency only.
import (
	"errors"
	"time"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

// BuildHistoricalBoundaryProof projects the one currently documented v0.4
// historical route into an INCONCLUSIVE v0.8 report. FD-004 is a planning
// question, NOT a claim that its scenario ran or that network policy enforced
// the observed effect. Broader scenario/route mappings require source-specific
// review; in particular, they must not be guessed from the topology diagram.
func BuildHistoricalBoundaryProof(source ProofReport, proofID, scenarioID, route string) (reporting.Document, error) {
	reject := errors.New("detached Boundary Proof source or scope rejected")
	if scenarioID != "FD-004" || route != "edge-to-data" {
		return reporting.Document{}, reject
	}
	if err := VerifyProofReport(source); err != nil {
		return reporting.Document{}, reject
	}

	// Safe opaque identifiers deliberately omit source filenames, local paths,
	// the Terraform receipt ID, cluster metadata and raw object content.
	intentTime, err := canonicalBoundaryUTC(source.PlanCapturedUTC)
	if err != nil {
		return reporting.Document{}, reject
	}
	observedTime, err := canonicalBoundaryUTC(source.ObservationCapturedUTC)
	if err != nil {
		return reporting.Document{}, reject
	}
	intent := reporting.Layer{
		Status: reporting.StatusRecorded, Reference: "plan-" + source.ReceiptSHA256[:16],
		SHA256: source.ReceiptSHA256, CapturedAtUTC: intentTime,
	}
	observed := reporting.Layer{
		Status: reporting.StatusRecorded, Reference: "observation-" + source.ObservationSHA256[:16],
		SHA256: source.ObservationSHA256, CapturedAtUTC: observedTime,
	}

	// v0.4 network data was collected separately and BEFORE the v0.5 plan.
	// ReportSHA256 identifies the historical report artifact. The untrusted
	// filename and provenance-free network result are deliberately not copied.
	behavior := reporting.Layer{Status: reporting.StatusMissing}
	for _, witness := range source.HistoricalWitnesses {
		if witness.RouteID != route {
			continue
		}
		if behavior.Status != reporting.StatusMissing || !witness.HistoricalOnly {
			return reporting.Document{}, reject
		}
		when, parseErr := canonicalBoundaryUTC(witness.CapturedUTC)
		if parseErr != nil {
			return reporting.Document{}, reject
		}
		behavior = reporting.Layer{
			Status:        reporting.StatusRecorded,
			Reference:     "historical-witness-" + route,
			SHA256:        witness.ReportSHA256,
			CapturedAtUTC: when,
		}
	}
	if behavior.Status != reporting.StatusRecorded {
		return reporting.Document{}, reject
	}

	doc, err := reporting.NewDocument(proofID, scenarioID, source.Profile, route, intent, observed, behavior)
	if err != nil {
		return reporting.Document{}, reject
	}
	return doc, nil
}

func canonicalBoundaryUTC(value string) (string, error) {
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || timestamp.IsZero() || timestamp.Location() != time.UTC {
		return "", errors.New("invalid UTC evidence timestamp")
	}
	return timestamp.UTC().Format(time.RFC3339Nano), nil
}
