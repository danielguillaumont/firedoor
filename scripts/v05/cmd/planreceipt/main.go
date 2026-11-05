// planreceipt adds an explicitly approved real speculative plan to a local,
// Git-ignored SQLite plan receipt store. The default is a non-executing preview.
// It NEVER saves raw plan JSON, Terraform state, provider logs or UI events.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	_ "modernc.org/sqlite"
)

func fail(message string) {
	// Never print raw provider, state, Terraform or SQLite diagnostics.
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

const relativeDB = "docs/evidence/local/v05-prooflens-live.sqlite"

func freshID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		fail("secure receipt ID generation failed")
	}
	return "TFPLAN-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(bytes)
}

func gitIgnored(root, relative string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", relative)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() == nil
}

func ownedBranch(root string) bool {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) == "feat/v05-evidence"
}

func ensureSafeLocalDirectory(root string) (string, error) {
	relativeParts := []string{"docs", "evidence", "local"}
	path := root
	for _, part := range relativeParts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if err := os.Mkdir(path, 0700); err != nil {
				return "", err
			}
			info, err = os.Lstat(path)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("invalid local evidence directory")
		}
	}
	return filepath.Join(path, "v05-prooflens-live.sqlite"), nil
}

func main() {
	rootArg := flag.String("root", ".", "Firedoor repository root")
	execute := flag.Bool("execute", false, "opt in to a real speculative plan followed by local SQLite write")
	consent := flag.String("consent", "", "exact local-plan consent token (not a credential)")
	flag.Parse()
	if flag.NArg() != 0 || (!*execute && *consent != "") {
		fail("unsupported arguments")
	}
	root, err := filepath.Abs(*rootArg)
	if err != nil {
		fail("invalid Firedoor repository root")
	}
	manifest, err := prooflens.CollectTerraformSources(root)
	if err != nil {
		fail("Terraform source admission rejected")
	}
	admission, err := prooflens.PrepareTerraformPlanAdmission(root, manifest)
	if err != nil || prooflens.VerifyTerraformPlanAdmission(admission) != nil {
		fail("Terraform command admission rejected")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if !*execute {
		_ = encoder.Encode(struct {
			Mode             string `json:"mode"`
			PlanExecuted     bool   `json:"plan_executed"`
			DatabaseOpened   bool   `json:"database_opened"`
			RawOutputSaved   bool   `json:"raw_output_saved"`
			ConsentRequired  bool   `json:"consent_required"`
			LocalStore       string `json:"local_store"`
			SourceTreeSHA256 string `json:"source_tree_sha256"`
		}{"REVIEW_ONLY_NO_TERRAFORM_EXECUTION", false, false, false, true, relativeDB, manifest.TreeSHA256})
		return
	}
	if *consent != prooflens.TerraformPlanConsent || !ownedBranch(root) || !gitIgnored(root, relativeDB) {
		fail("plan receipt refused: invalid consent, branch or local Git-ignore rule")
	}
	// No SQLite file or directory is created before the real plan succeeds.
	report, err := prooflens.RunGuardedTerraformPlan(context.Background(), root, manifest, admission, *consent)
	if err != nil {
		fail("guarded plan failed; review Terraform lock and provider processes before retrying")
	}
	receipt, err := prooflens.SealLivePlanReceipt(report, freshID())
	if err != nil {
		fail("sanitized report validation failed; no evidence stored")
	}
	dbPath, err := ensureSafeLocalDirectory(root)
	if err != nil {
		fail("unsafe local receipt directory; plan finished but was not stored")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := prooflens.OpenSQLiteStore(ctx, dbPath)
	if err != nil {
		fail("local SQLite open/migration failed; plan finished but was not stored")
	}
	defer store.Close()
	if err := store.PutPlanReceipt(ctx, receipt); err != nil {
		fail("local SQLite plan receipt insert failed")
	}
	verified, err := store.GetPlanReceipt(ctx, receipt.ReceiptID)
	if err != nil || verified.ReceiptSHA256 != receipt.ReceiptSHA256 {
		fail("stored plan receipt did not verify")
	}
	version, err := store.Version(ctx)
	if err != nil || version != 2 {
		fail("SQLite schema version incorrect")
	}
	// Emit only allowlisted metrics. No receipt JSON or raw plan output is printed.
	_ = encoder.Encode(struct {
		Mode             string                             `json:"mode"`
		PlanCompleted    bool                               `json:"plan_completed"`
		Applied          bool                               `json:"applied"`
		RawOutputSaved   bool                               `json:"raw_output_saved"`
		DBPath           string                             `json:"db_path"`
		DBSchemaVersion  int                                `json:"db_schema_version"`
		ReceiptID        string                             `json:"receipt_id"`
		ReceiptSHA256    string                             `json:"receipt_sha256"`
		SourceTreeSHA256 string                             `json:"source_tree_sha256"`
		UIStreamSHA256   string                             `json:"ui_stream_sha256"`
		Changes          prooflens.TerraformUIChangeSummary `json:"changes"`
		ProofVerdict     string                             `json:"proof_verdict"`
		StoredVerified   bool                               `json:"stored_verified"`
	}{"LOCAL_SPECULATIVE_PLAN_RECEIPT_STORED", true, false, false, relativeDB, version, verified.ReceiptID, verified.ReceiptSHA256,
		verified.SourceTreeSHA256, verified.SanitizedPlanSummary.UIStreamSHA256, verified.SanitizedPlanSummary.Changes,
		verified.ProofVerdict, true})
}
