// planrun is an opt-in, guarded adapter for LOCAL speculative Terraform plans.
// Without both --execute and exact --consent, it never invokes external tools.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func fail(message string) {
	// Never print underlying provider, state or Terraform diagnostic strings.
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func main() {
	rootFlag := flag.String("root", ".", "Firedoor repository root")
	execute := flag.Bool("execute", false, "explicitly allow a guarded local speculative plan")
	consent := flag.String("consent", "", "exact human approval token (not a credential)")
	flag.Parse()
	if flag.NArg() != 0 || (!*execute && *consent != "") {
		fail("unsupported arguments")
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fail("invalid repository root")
	}
	manifest, err := prooflens.CollectTerraformSources(root)
	if err != nil {
		fail("approved Terraform sources unavailable")
	}
	admission, err := prooflens.PrepareTerraformPlanAdmission(root, manifest)
	if err != nil || prooflens.VerifyTerraformPlanAdmission(admission) != nil {
		fail("source admission rejected")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if !*execute {
		// Source-only. No terraform, kubectl, state reads or lab side effects.
		_ = encoder.Encode(struct {
			Mode             string `json:"mode"`
			Profile          string `json:"profile"`
			Context          string `json:"context"`
			SourceTreeSHA256 string `json:"source_tree_sha256"`
			PlanExecuted     bool   `json:"plan_executed"`
			RawOutputSaved   bool   `json:"raw_output_saved"`
			ConsentRequired  bool   `json:"consent_required"`
		}{"REVIEW_ONLY_NO_TERRAFORM_EXECUTION", admission.Profile, admission.Context,
			admission.SourceTreeSHA256, false, false, true})
		return
	}
	if *consent != prooflens.TerraformPlanConsent {
		fail("explicit local-plan consent missing or invalid")
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	branch, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(branch)) != "feat/v05-evidence" {
		fail("wrong Git branch for guarded plan")
	}
	report, err := prooflens.RunGuardedTerraformPlan(context.Background(), root, manifest, admission, *consent)
	if err != nil {
		fail("guarded speculative plan failed; inspect local state lock and providers before retrying")
	}
	if err := encoder.Encode(report); err != nil {
		fail("cannot encode sanitized report")
	}
}
