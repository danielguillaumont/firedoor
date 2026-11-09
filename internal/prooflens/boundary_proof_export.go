package prooflens

// Offline v0.8 exporter: reads one previously saved detached v0.5 report and
// writes only a bounded, inconclusive v0.8 projection. It never performs live
// collection, Terraform operations, Kubernetes queries or network requests.
import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

const (
	BoundaryExportConsent = "I-APPROVE-FIREDOOR-V08-OFFLINE-BOUNDARY-EXPORT"
	LocalBoundaryJSON     = "docs/evidence/local/v08-boundary-proof.json"
	LocalBoundaryHTML     = "docs/evidence/local/v08-boundary-proof.html"
	boundaryExportLock    = "docs/evidence/local/.v08-boundary-proof-export.lock"
	boundarySourceLimit   = 128 * 1024
)

var boundaryExportRejected = errors.New("offline Boundary Proof export rejected; no evidence was collected or disclosed")

// ExportApprovedBoundaryProofOnce has no configurable input or destination:
// both paths must be ignored, untracked and underneath the actual repo root.
// Source integrity is local consistency, not independent authentication.
func ExportApprovedBoundaryProofOnce(root, consent string) (string, string, error) {
	if consent != BoundaryExportConsent {
		return "", "", boundaryExportRejected
	}
	base, err := boundaryExportRepo(root)
	if err != nil {
		return "", "", boundaryExportRejected
	}
	source, err := readBoundarySource(base)
	if err != nil {
		return "", "", boundaryExportRejected
	}
	doc, err := BuildHistoricalBoundaryProof(source, "BP-V08-HISTORICAL-004", "FD-004", "edge-to-data")
	if err != nil {
		return "", "", boundaryExportRejected
	}
	jsonData, err := reporting.RenderJSON(doc)
	if err != nil {
		return "", "", boundaryExportRejected
	}
	htmlData, err := reporting.RenderHTML(doc)
	if err != nil {
		return "", "", boundaryExportRejected
	}
	if err := saveBoundaryPairOnce(base, jsonData, htmlData); err != nil {
		return "", "", boundaryExportRejected
	}
	return LocalBoundaryJSON, LocalBoundaryHTML, nil
}

func boundaryExportRepo(root string) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	repo := exec.Command("git", "rev-parse", "--show-toplevel")
	repo.Dir = base
	out, err := repo.Output()
	if err != nil || !strings.EqualFold(filepath.Clean(filepath.FromSlash(strings.TrimSpace(string(out)))), filepath.Clean(base)) {
		return "", boundaryExportRejected
	}
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = base
	branchOut, err := branchCmd.Output()
	if err != nil || (strings.TrimSpace(string(branchOut)) != "feat/v08-proofs" && strings.TrimSpace(string(branchOut)) != "main") {
		return "", boundaryExportRejected
	}
	for _, path := range []string{ConsoleReportRelativePath, LocalBoundaryJSON, LocalBoundaryHTML, boundaryExportLock} {
		tracked, err := boundaryTracked(base, path)
		if err != nil || tracked || !GitIgnoredEvidencePath(base, path) {
			return "", boundaryExportRejected
		}
	}
	// Every ancestor is checked, not merely the final destination.
	for _, rel := range []string{"docs", "docs/evidence", "docs/evidence/local"} {
		fi, err := os.Lstat(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return "", boundaryExportRejected
		}
	}
	return base, nil
}

func boundaryTracked(base, rel string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--", rel)
	cmd.Dir = base
	out, err := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0, err
}

func readBoundarySource(base string) (ProofReport, error) {
	p := filepath.Join(base, filepath.FromSlash(ConsoleReportRelativePath))
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > boundarySourceLimit {
		return ProofReport{}, boundaryExportRejected
	}
	f, err := os.Open(p)
	if err != nil {
		return ProofReport{}, boundaryExportRejected
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() <= 0 || opened.Size() > boundarySourceLimit {
		return ProofReport{}, boundaryExportRejected
	}
	decoder := json.NewDecoder(io.LimitReader(f, boundarySourceLimit+1))
	decoder.DisallowUnknownFields()
	var source ProofReport
	if decoder.Decode(&source) != nil {
		return ProofReport{}, boundaryExportRejected
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || VerifyProofReport(source) != nil {
		return ProofReport{}, boundaryExportRejected
	}
	return source, nil
}

func saveBoundaryPairOnce(base string, jsonData, htmlData []byte) error {
	if err := saveOfflineProofPairOnce(base, boundaryExportLock, LocalBoundaryJSON, LocalBoundaryHTML, jsonData, htmlData); err != nil {
		return boundaryExportRejected
	}
	return nil
}
