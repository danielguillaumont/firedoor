package prooflens

// One-time, opt-in import of a previously generated detached v0.5 JSON report.
// It never opens Terraform state, SQLite, Kubernetes, or the network. The
// detached source is only consistency-checked: hashes are NOT signatures.
import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ConsoleReportRelativePath = "docs/evidence/local/v06-console-proof-report.json"
	ConsoleApprovalConsent    = "I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW"
	maxDetachedProofJSON      = 128 * 1024
)

func LoadApprovedConsoleProof(root, consent string) (ConsoleProofSummary, error) {
	reject := errors.New("approved detached Proof Lens report unavailable or rejected")
	if consent != ConsoleApprovalConsent {
		return ConsoleProofSummary{}, reject
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ConsoleProofSummary{}, reject
	}
	rootAbs, err = filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return ConsoleProofSummary{}, reject
	}
	repo := exec.Command("git", "rev-parse", "--show-toplevel")
	repo.Dir = rootAbs
	out, err := repo.Output()
	if err != nil || !strings.EqualFold(filepath.Clean(filepath.FromSlash(strings.TrimSpace(string(out)))), filepath.Clean(rootAbs)) {
		return ConsoleProofSummary{}, reject
	}
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = rootAbs
	branch, err := branchCmd.Output()
	if err != nil || (strings.TrimSpace(string(branch)) != "main" && strings.TrimSpace(string(branch)) != "feat/v06-console") {
		return ConsoleProofSummary{}, reject
	}
	if !GitIgnoredEvidencePath(rootAbs, ConsoleReportRelativePath) {
		return ConsoleProofSummary{}, reject
	}
	// Explicitly reject files already tracked despite a later ignore rule.
	tracked := exec.Command("git", "ls-files", "--error-unmatch", "--", ConsoleReportRelativePath)
	tracked.Dir = rootAbs
	if tracked.Run() == nil {
		return ConsoleProofSummary{}, reject
	}
	// Refuse symlinked evidence directories and symlinked report files.
	for _, part := range []string{"docs", "docs/evidence", "docs/evidence/local", ConsoleReportRelativePath} {
		info, err := os.Lstat(filepath.Join(rootAbs, filepath.FromSlash(part)))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ConsoleProofSummary{}, reject
		}
	}
	fullPath := filepath.Join(rootAbs, filepath.FromSlash(ConsoleReportRelativePath))
	f, err := os.Open(fullPath)
	if err != nil {
		return ConsoleProofSummary{}, reject
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxDetachedProofJSON {
		return ConsoleProofSummary{}, reject
	}
	var report ProofReport
	d := json.NewDecoder(io.LimitReader(f, maxDetachedProofJSON+1))
	d.DisallowUnknownFields()
	if err := d.Decode(&report); err != nil {
		return ConsoleProofSummary{}, reject
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return ConsoleProofSummary{}, reject
	}
	summary, err := ProjectConsoleProofReport(report)
	if err != nil {
		return ConsoleProofSummary{}, reject
	}
	return summary, nil
}
