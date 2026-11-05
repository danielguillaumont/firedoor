package prooflens

// Export is deliberately limited to one local Git-ignored filename.
// It is opt-in, never overwrites, never follows a symlink and writes only the
// verified, whitelisted Markdown projection (not raw plan/state/HTTP data).
import (
	"errors"
	"os"
	"path/filepath"
)

const LocalMarkdownReport = "docs/evidence/local/v05-proof-lens-report.md"

func ExportVerifiedMarkdownOnce(root string, r ProofReport) (string, error) {
	text, err := RenderProofReportMarkdown(r)
	if err != nil {
		return "", errors.New("unverified report cannot be exported")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", errors.New("invalid repository root")
	}
	if !GitIgnoredEvidencePath(absolute, LocalMarkdownReport) {
		return "", errors.New("Markdown export path must be Git-ignored")
	}
	// Reject symlinked ancestors, including a redirected evidence folder.
	dir := absolute
	for _, name := range []string{"docs", "evidence", "local"} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("unsafe evidence directory")
		}
		dir = filepath.Join(dir, name)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsafe local evidence directory")
	}
	path := filepath.Join(dir, "v05-proof-lens-report.md")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", errors.New("report already exists or cannot be created")
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
	n, writeErr := file.WriteString(text)
	if writeErr == nil && n != len(text) {
		writeErr = errors.New("short write")
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return "", errors.New("report could not be completely saved")
	}
	committed = true
	return LocalMarkdownReport, nil
}
