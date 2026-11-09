package prooflens

// Slice 8C3 exports a bounded, INCONCLUSIVE comparison of two previously
// reviewed detached Boundary Proofs. It never collects evidence, performs
// a lab experiment, queries Kubernetes, or authenticates source provenance.
import (
	"errors"
	"io"
	"os"
	"path/filepath"

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

// saveComparisonOnce serializes writers, refuses preexisting paths including
// symlinks, and rolls back newly created files on ordinary write failures.
// Power loss may leave a partial file or stale lock: future runs fail closed.
func saveComparisonOnce(base string, jsonData, htmlData []byte) error {
	lockPath := filepath.Join(base, filepath.FromSlash(boundaryComparisonExportLock))
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return comparisonExportRejected
	}
	_ = lock.Close()
	defer os.Remove(lockPath)

	jsonPath := filepath.Join(base, filepath.FromSlash(LocalComparisonJSON))
	htmlPath := filepath.Join(base, filepath.FromSlash(LocalComparisonHTML))
	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return comparisonExportRejected
		}
	}
	created := make([]string, 0, 2)
	committed := false
	defer func() {
		if !committed {
			for _, path := range created {
				_ = os.Remove(path)
			}
		}
	}()

	for _, item := range []struct {
		path string
		data []byte
	}{
		{jsonPath, jsonData},
		{htmlPath, htmlData},
	} {
		file, err := os.OpenFile(item.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return comparisonExportRejected
		}
		created = append(created, item.path)
		n, writeErr := file.Write(item.data)
		if writeErr == nil && n != len(item.data) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return comparisonExportRejected
		}
	}
	committed = true
	return nil
}
