package prooflens

// CollectApprovedProofReport reconstructs a v0.5 report from its original local
// sources. Every call reopens SQLite read-only and performs fresh Kubernetes
// preflight; it does not run Terraform or execute any traffic experiment.
import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

const (
	ApprovedReportConsent = "I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT"
	LocalReportDatabase   = "docs/evidence/local/v05-prooflens-live.sqlite"
)

var approvedReceiptID = regexp.MustCompile(`^TFPLAN-[A-Za-z0-9-]{8,121}$`)

func GitIgnoredEvidencePath(root, rel string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", rel)
	cmd.Dir = root
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
}

func approvedEvidenceRepo(root string) bool {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	branch := strings.TrimSpace(string(b))
	// Release reporting is read-only and safe on main after merge.
	// Guarded Terraform execution remains restricted to the feature branch.
	return (branch == "feat/v05-evidence" || branch == "main") &&
		GitIgnoredEvidencePath(root, LocalReportDatabase)
}

func CollectApprovedProofReport(root, receiptID, consent string) (ProofReport, error) {
	if consent != ApprovedReportConsent || !approvedReceiptID.MatchString(receiptID) {
		return ProofReport{}, errors.New("report consent or receipt rejected")
	}
	absolute, err := filepath.Abs(root)
	if err != nil || !approvedEvidenceRepo(absolute) {
		return ProofReport{}, errors.New("unapproved Firedoor evidence repository")
	}
	dir := filepath.Join(absolute, "docs", "evidence", "local")
	names, err := DiscoverHistoricalWitnessReports(dir)
	if err != nil || len(names) != 2 {
		return ProofReport{}, errors.New("two approved historical records required")
	}
	history := make([]HistoricalWitnessRecord, 0, 2)
	for _, name := range names {
		w, err := InspectHistoricalWitnessPair(dir, name)
		if err != nil {
			return ProofReport{}, errors.New("historical evidence validation rejected")
		}
		history = append(history, w)
	}
	source, err := CollectTerraformSources(absolute)
	if err != nil {
		return ProofReport{}, errors.New("Terraform source inventory rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	store, err := OpenSQLiteStoreReadOnly(ctx, filepath.Join(absolute, filepath.FromSlash(LocalReportDatabase)))
	if err != nil {
		return ProofReport{}, errors.New("read-only evidence database rejected")
	}
	defer store.Close()
	receipt, err := store.GetPlanReceipt(ctx, receiptID)
	if err != nil || VerifyLivePlanReceipt(receipt) != nil || receipt.SourceTreeSHA256 != source.TreeSHA256 {
		return ProofReport{}, errors.New("stored Terraform receipt rejected")
	}
	version, err := store.Version(ctx)
	if err != nil || version != SQLiteSchemaVersion {
		return ProofReport{}, errors.New("unapproved database schema")
	}
	preflight, err := attribution.RunPreflight(ctx, attribution.ReadOnlyCommands{})
	if err != nil {
		return ProofReport{}, errors.New("owned Kubernetes lab admission rejected")
	}
	if err := RecheckTerraformSources(absolute, source); err != nil {
		return ProofReport{}, errors.New("Terraform source changed")
	}
	observation, err := BuildKubernetesObservation(preflight, source)
	if err != nil {
		return ProofReport{}, errors.New("Kubernetes observation rejected")
	}
	correlation, err := CorrelatePlanReceiptWithObservation(receipt, observation)
	if err != nil {
		return ProofReport{}, errors.New("receipt correlation rejected")
	}
	timeline, err := BuildHistoricalBehaviorContext(correlation, history)
	if err != nil {
		return ProofReport{}, errors.New("historical context rejected")
	}
	report, err := BuildProofReport(correlation, timeline)
	if err != nil || VerifyProofReport(report) != nil {
		return ProofReport{}, errors.New("Proof Lens projection rejected")
	}
	return report, nil
}
