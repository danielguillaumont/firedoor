// behaviorcontext joins separately verified v0.4 historical A/B/A witness
// reports to a freshly observed v0.5 receipt/Kubernetes correlation.
// This is a non-causal, read-only timeline. It never replays witnesses,
// executes Terraform, saves reports or writes to the SQLite evidence store.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
	_ "modernc.org/sqlite"
)

const (
	consentText = "I-APPROVE-READ-ONLY-FIREDOOR-BEHAVIOR-CONTEXT"
	dbRel       = "docs/evidence/local/v05-prooflens-live.sqlite"
)

func main() {
	if err := run(); err != nil {
		// Never print raw v0.4 HTTP diagnostics, Terraform/UI output, SQL errors,
		// kubectl responses, user-supplied paths, or state/provider values.
		fmt.Fprintln(os.Stderr, "Firedoor historical behavior context refused; verify paired witnesses, read-only receipt, source, branch and owned lab")
		os.Exit(1)
	}
}

func branchApproved(root string) bool {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	raw, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(raw)) == "feat/v05-evidence"
}
func databaseIgnored(root string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", dbRel)
	cmd.Dir = root
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func loadApprovedHistory(root string) ([]prooflens.HistoricalWitnessRecord, error) {
	dir := filepath.Join(root, "docs", "evidence", "local")
	names, err := prooflens.DiscoverHistoricalWitnessReports(dir)
	if err != nil || len(names) != 2 {
		return nil, errors.New("exactly two reviewed historical reports required")
	}
	records := make([]prooflens.HistoricalWitnessRecord, 0, 2)
	for _, name := range names {
		r, err := prooflens.InspectHistoricalWitnessPair(dir, name)
		if err != nil {
			return nil, errors.New("historical report/journal pair rejected")
		}
		records = append(records, r)
	}
	return records, nil
}

func run() error {
	rootFlag := flag.String("root", ".", "Firedoor repository root")
	history := flag.Bool("history", false, "Offline validate both historical A/B/A report/journal pairs")
	correlate := flag.Bool("correlate", false, "Join both historical records with the read-only receipt and fresh read-only Kubernetes observation")
	receiptID := flag.String("receipt-id", "", "Existing exact Terraform plan receipt ID")
	consent := flag.String("consent", "", "Exact operator consent for fresh Kubernetes inspection")
	flag.Parse()
	if flag.NArg() != 0 || (*history && *correlate) || (!*correlate && (*consent != "" || *receiptID != "")) {
		return errors.New("invalid CLI flags")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if !*history && !*correlate {
		return encoder.Encode(struct {
			Mode                  string `json:"mode"`
			HistoricalFilesOpened bool   `json:"historical_files_opened"`
			DatabaseOpened        bool   `json:"database_opened"`
			KubernetesObserved    bool   `json:"kubernetes_observed"`
			TerraformExecuted     bool   `json:"terraform_executed"`
			WitnessReplayed       bool   `json:"witness_replayed"`
			ConsentRequired       bool   `json:"consent_required"`
			ProofVerdict          string `json:"proof_verdict"`
		}{"REVIEW_ONLY_NO_BEHAVIOR_CONTEXT_ACCESS", false, false, false, false, false, true, prooflens.Inconclusive})
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil || !branchApproved(root) {
		return errors.New("unapproved repository")
	}
	historical, err := loadApprovedHistory(root)
	if err != nil {
		return err
	}
	if *history {
		return encoder.Encode(struct {
			Mode                   string                              `json:"mode"`
			HistoricalOnly         bool                                `json:"historical_only"`
			HistoricalWitnessCount int                                 `json:"historical_witness_count"`
			Records                []prooflens.HistoricalWitnessRecord `json:"records"`
			DatabaseOpened         bool                                `json:"database_opened"`
			KubernetesObserved     bool                                `json:"kubernetes_observed"`
			TerraformExecuted      bool                                `json:"terraform_executed"`
			ProofVerdict           string                              `json:"proof_verdict"`
		}{"HISTORICAL_PAIRS_ONLY_READ_ONLY", true, len(historical), historical, false, false, false, prooflens.Inconclusive})
	}
	if *consent != consentText || !strings.HasPrefix(*receiptID, "TFPLAN-") || len(*receiptID) > 128 || !databaseIgnored(root) {
		return errors.New("missing explicit consent or invalid receipt")
	}
	manifest, err := prooflens.CollectTerraformSources(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	store, err := prooflens.OpenSQLiteStoreReadOnly(ctx, filepath.Join(root, filepath.FromSlash(dbRel)))
	if err != nil {
		return err
	}
	defer store.Close()
	p, err := store.GetPlanReceipt(ctx, *receiptID)
	if err != nil || prooflens.VerifyLivePlanReceipt(p) != nil || p.SourceTreeSHA256 != manifest.TreeSHA256 {
		return errors.New("stored plan receipt invalid or unrelated to source")
	}
	version, err := store.Version(ctx)
	if err != nil || version != 2 {
		return errors.New("unexpected database schema")
	}
	// Only after reading the existing receipt and historical pairs successfully
	// do we request a fresh read-only view of the owned Kubernetes testbed.
	preflight, err := attribution.RunPreflight(ctx, attribution.ReadOnlyCommands{})
	if err != nil {
		return errors.New("Kubernetes read-only preflight rejected")
	}
	if err := prooflens.RecheckTerraformSources(root, manifest); err != nil {
		return errors.New("Terraform source drift during observation")
	}
	obs, err := prooflens.BuildKubernetesObservation(preflight, manifest)
	if err != nil {
		return err
	}
	joined, err := prooflens.CorrelatePlanReceiptWithObservation(p, obs)
	if err != nil {
		return err
	}
	context, err := prooflens.BuildHistoricalBehaviorContext(joined, historical)
	if err != nil {
		return err
	}
	return encoder.Encode(context)
}
