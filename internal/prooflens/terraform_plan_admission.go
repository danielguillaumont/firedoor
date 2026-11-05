package prooflens

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Slice 2B2B is a source-only review gate. It does NOT start Terraform.
// A separate, later adapter must implement actual process execution and
// operator consent. Do not treat this spec as authority to execute a plan.
const (
	TerraformPlanAdmissionSchema = "firedoor.v05.plan-admission.v1"
	TerraformPlanAdmissionMode   = "REVIEW_ONLY_NO_TERRAFORM_EXECUTION"
	ApprovedTerraformProfile     = "hardened"
	ApprovedTerraformContext     = "kind-firedoor-local"
)

type TerraformPlanAdmission struct {
	SchemaVersion            string   `json:"schema_version"`
	Mode                     string   `json:"mode"`
	Profile                  string   `json:"profile"`
	Context                  string   `json:"context"`
	WorkingDirectory         string   `json:"working_directory"`
	Executable               string   `json:"executable"`
	Arguments                []string `json:"arguments"`
	SourceTreeSHA256         string   `json:"source_tree_sha256"`
	PlanExecuted             bool     `json:"plan_executed"`
	ExternalCommandsExecuted bool     `json:"external_commands_executed"`
	RawPlanSaved             bool     `json:"raw_plan_saved"`
	Limitations              []string `json:"limitations"`
}

func planAdmissionLimitations() []string {
	return []string{
		"This is an offline command specification, not an executed or applied Terraform plan.",
		"A future speculative plan may read provider resources and state and acquire a state lock.",
		"Never log Terraform UI messages, output values, state, diagnostics or saved plan data.",
		"An actual plan requires separate owned-lab admission and explicit operator consent.",
	}
}

func fixedPlanArguments() []string {
	return []string{
		"plan",
		"-json",
		"-input=false",
		"-lock-timeout=5s",
		"-var=security_profile=hardened",
		"-var=kube_context=kind-firedoor-local",
	}
}

// PrepareTerraformPlanAdmission emits an exact allowlisted invocation plan.
// Repository paths are only labels in the report; no secrets are read.
func PrepareTerraformPlanAdmission(repoRoot string, manifest TerraformSourceManifest) (TerraformPlanAdmission, error) {
	var empty TerraformPlanAdmission
	if err := VerifyTerraformSourceManifest(manifest); err != nil {
		return empty, errors.New("unverified Terraform source manifest")
	}
	if err := RecheckTerraformSources(repoRoot, manifest); err != nil {
		return empty, errors.New("Terraform source drift detected")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return empty, errors.New("invalid repository root")
	}
	if !filepath.IsAbs(root) {
		return empty, errors.New("nonabsolute repository root")
	}
	return TerraformPlanAdmission{
		SchemaVersion:            TerraformPlanAdmissionSchema,
		Mode:                     TerraformPlanAdmissionMode,
		Profile:                  ApprovedTerraformProfile,
		Context:                  ApprovedTerraformContext,
		WorkingDirectory:         "infrastructure/terraform/testbed",
		Executable:               "terraform",
		Arguments:                fixedPlanArguments(),
		SourceTreeSHA256:         manifest.TreeSHA256,
		PlanExecuted:             false,
		ExternalCommandsExecuted: false,
		RawPlanSaved:             false,
		Limitations:              planAdmissionLimitations(),
	}, nil
}

func VerifyTerraformPlanAdmission(v TerraformPlanAdmission) error {
	if v.SchemaVersion != TerraformPlanAdmissionSchema || v.Mode != TerraformPlanAdmissionMode ||
		v.Profile != ApprovedTerraformProfile || v.Context != ApprovedTerraformContext ||
		v.WorkingDirectory != "infrastructure/terraform/testbed" || v.Executable != "terraform" ||
		!validHex(v.SourceTreeSHA256, 64) || v.PlanExecuted || v.ExternalCommandsExecuted || v.RawPlanSaved ||
		len(v.Limitations) != len(planAdmissionLimitations()) {
		return errors.New("unexpected or unsafe plan admission")
	}
	args := fixedPlanArguments()
	if len(v.Arguments) != len(args) {
		return errors.New("modified Terraform arguments")
	}
	for i := range args {
		if v.Arguments[i] != args[i] {
			return fmt.Errorf("modified Terraform argument %d", i)
		}
	}
	for i, line := range planAdmissionLimitations() {
		if v.Limitations[i] != line {
			return errors.New("modified limitations")
		}
	}
	return nil
}

// ReviewTerraformUIPlanStream reduces a supplied stream and validates a
// matching, already reviewed admission. No external commands or file writes
// occur. A future adapter must supply the live stream with independent guards.
func ReviewTerraformUIPlanStream(stream io.Reader, root string, source TerraformSourceManifest, admission TerraformPlanAdmission) (TerraformUIPlanSummary, error) {
	var empty TerraformUIPlanSummary
	if err := VerifyTerraformPlanAdmission(admission); err != nil {
		return empty, errors.New("invalid plan admission")
	}
	if err := VerifyTerraformSourceManifest(source); err != nil || admission.SourceTreeSHA256 != source.TreeSHA256 {
		return empty, errors.New("source fingerprint differs from admission")
	}
	if strings.TrimSpace(root) == "" {
		return empty, errors.New("repository root missing")
	}
	if err := RecheckTerraformSources(root, source); err != nil {
		return empty, errors.New("source tree drift before stream")
	}
	summary, err := SummarizeTerraformUIStream(stream, source)
	if err != nil {
		return empty, errors.New("Terraform UI stream is invalid or incomplete")
	}
	if err := RecheckTerraformSources(root, source); err != nil {
		return empty, errors.New("source tree drift after stream")
	}
	if summary.SourceTreeSHA256 != admission.SourceTreeSHA256 || summary.ProofVerdict != Inconclusive {
		return empty, errors.New("unsafe summary or source mismatch")
	}
	return summary, nil
}
