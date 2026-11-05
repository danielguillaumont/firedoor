// receiptcorrelate independently reopens the existing LIVE plan receipt DB in
// SQLite read-only mode. Optionally it performs a fresh consent-gated Kubernetes
// preflight and joins two validated records by a Terraform source-tree digest.
// It never runs Terraform, applies a policy, probes traffic, or writes evidence.
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
	consentText = "I-APPROVE-READ-ONLY-FIREDOOR-RECEIPT-CORRELATION"
	dbRelative  = "docs/evidence/local/v05-prooflens-live.sqlite"
)

func fail() {
	// Deliberately do not echo raw provider, state, SQLite, or kubectl diagnostics.
	fmt.Fprintln(os.Stderr, "Firedoor receipt correlation refused; verify lab, receipt, source fingerprint and local database")
	os.Exit(1)
}

func gitCheck(root string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() == nil
}

func correctBranch(root string) bool {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) == "feat/v05-evidence"
}

func run() error {
	rootFlag := flag.String("root", ".", "Firedoor repository root")
	idFlag := flag.String("receipt-id", "", "Exact existing TFPLAN receipt ID")
	inspect := flag.Bool("inspect", false, "Read existing SQLite receipt without Kubernetes access")
	correlate := flag.Bool("correlate", false, "Read receipt and perform a fresh read-only Kubernetes observation")
	consent := flag.String("consent", "", "Exact opt-in consent for Kubernetes observation")
	flag.Parse()
	if flag.NArg() > 0 || (*inspect && *correlate) || (!*correlate && *consent != "") {
		return errors.New("invalid CLI mode")
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	manifest, err := prooflens.CollectTerraformSources(root)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if !*inspect && !*correlate {
		if *idFlag != "" {
			return errors.New("receipt ID without mode not accepted")
		}
		return encoder.Encode(struct {
			Mode               string `json:"mode"`
			DatabaseOpened     bool   `json:"database_opened"`
			KubernetesObserved bool   `json:"kubernetes_observed"`
			TerraformExecuted  bool   `json:"terraform_executed"`
			ConsentRequired    bool   `json:"consent_required"`
			SourceTreeSHA256   string `json:"source_tree_sha256"`
		}{"REVIEW_ONLY_NO_EXTERNAL_ACCESS", false, false, false, true, manifest.TreeSHA256})
	}
	if !strings.HasPrefix(*idFlag, "TFPLAN-") || len(*idFlag) > 128 || !correctBranch(root) ||
		!gitCheck(root, "check-ignore", "--quiet", "--", dbRelative) {
		return errors.New("receipt path or branch not admitted")
	}
	if *correlate && *consent != consentText {
		return errors.New("live Kubernetes read requires exact operator consent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	path := filepath.Join(root, filepath.FromSlash(dbRelative))
	store, err := prooflens.OpenSQLiteStoreReadOnly(ctx, path)
	if err != nil {
		return err
	}
	defer store.Close()
	p, err := store.GetPlanReceipt(ctx, *idFlag)
	if err != nil || prooflens.VerifyLivePlanReceipt(p) != nil || p.SourceTreeSHA256 != manifest.TreeSHA256 {
		return errors.New("stored receipt missing, corrupted or source digest differs")
	}
	version, err := store.Version(ctx)
	if err != nil || version != 2 {
		return errors.New("unexpected local database schema")
	}
	if *inspect {
		return encoder.Encode(struct {
			Mode                string                             `json:"mode"`
			DatabaseReadOnly    bool                               `json:"database_read_only"`
			DBSchemaVersion     int                                `json:"db_schema_version"`
			ReceiptID           string                             `json:"receipt_id"`
			ReceiptSHA256       string                             `json:"receipt_sha256"`
			SourceTreeSHA256    string                             `json:"source_tree_sha256"`
			SourceDigestMatches bool                               `json:"source_digest_matches"`
			PlanChanges         prooflens.TerraformUIChangeSummary `json:"plan_changes"`
			ProofVerdict        string                             `json:"proof_verdict"`
			KubernetesObserved  bool                               `json:"kubernetes_observed"`
		}{"LOCAL_SQLITE_RECEIPT_READ_ONLY", true, version, p.ReceiptID, p.ReceiptSHA256,
			p.SourceTreeSHA256, true, p.SanitizedPlanSummary.Changes, p.ProofVerdict, false})
	}
	// Fresh real Kubernetes read only after the pre-existing receipt validates.
	preflight, err := attribution.RunPreflight(ctx, attribution.ReadOnlyCommands{})
	if err != nil {
		return errors.New("owned Kubernetes preflight rejected")
	}
	if err := prooflens.RecheckTerraformSources(root, manifest); err != nil {
		return errors.New("Terraform source changed during Kubernetes admission")
	}
	obs, err := prooflens.BuildKubernetesObservation(preflight, manifest)
	if err != nil {
		return err
	}
	link, err := prooflens.CorrelatePlanReceiptWithObservation(p, obs)
	if err != nil {
		return err
	}
	return encoder.Encode(link)
}

func main() {
	if err := run(); err != nil {
		fail()
	}
}
