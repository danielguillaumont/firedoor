package prooflens

// Slice 8C3 exports a bounded, INCONCLUSIVE comparison of two previously
// reviewed detached Boundary Proofs. It never collects evidence, performs
// a lab experiment, queries Kubernetes, or authenticates source provenance.
import (
	"errors"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

const (
	BoundaryComparisonExportConsent = "I-APPROVE-FIREDOOR-V08-OFFLINE-COMPARISON-EXPORT"
	LocalComparisonJSON             = "docs/evidence/local/v08-boundary-comparison.json"
	LocalComparisonHTML             = "docs/evidence/local/v08-boundary-comparison.html"
	boundaryComparisonExportLock    = "docs/evidence/local/.v08-boundary-comparison-export.lock"
)

var comparisonExportRejected = errors.New("offline Boundary Proof comparison export refused; no evidence was collected or disclosed")

// ExportApprovedBoundaryComparisonOnce reads only the fixed baseline/hardened
// documents via the 8C2 loader and writes only two fixed ignored outputs.
// Consent is for local offline file access, not a provenance attestation.
func ExportApprovedBoundaryComparisonOnce(root, consent string) (string, string, error) {
	if consent != BoundaryComparisonExportConsent {
		return "", "", comparisonExportRejected
	}
	base, err := boundaryExportRepo(root)
	if err != nil {
		return "", "", comparisonExportRejected
	}
	for _, rel := range []string{LocalComparisonJSON, LocalComparisonHTML, boundaryComparisonExportLock} {
		tracked, err := boundaryTracked(base, rel)
		if err != nil || tracked || !GitIgnoredEvidencePath(base, rel) {
			return "", "", comparisonExportRejected
		}
	}

	comparison, err := LoadApprovedBoundaryProofPair(base, BoundaryPairReadConsent)
	if err != nil {
		return "", "", comparisonExportRejected
	}
	jsonData, err := reporting.RenderComparisonJSON(comparison)
	if err != nil {
		return "", "", comparisonExportRejected
	}
	htmlData, err := reporting.RenderComparisonHTML(comparison)
	if err != nil {
		return "", "", comparisonExportRejected
	}
	if err := saveComparisonOnce(base, jsonData, htmlData); err != nil {
		return "", "", comparisonExportRejected
	}
	return LocalComparisonJSON, LocalComparisonHTML, nil
}

// saveComparisonOnce shares the bounded, read-back-verified writer with the
// single-proof export. Power loss remains non-atomic and fails closed.
func saveComparisonOnce(base string, jsonData, htmlData []byte) error {
	if err := saveOfflineProofPairOnce(base, boundaryComparisonExportLock, LocalComparisonJSON, LocalComparisonHTML, jsonData, htmlData); err != nil {
		return comparisonExportRejected
	}
	return nil
}
