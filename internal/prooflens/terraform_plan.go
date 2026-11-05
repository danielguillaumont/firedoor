package prooflens

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Slice 2B1 converts already-available Terraform JSON into an intentionally
// sparse, non-secret summary. It NEVER starts Terraform or reads any state file.
// Raw JSON can contain cleartext secrets. Callers must not log or persist it.
const (
	TerraformPlanSummarySchema = "firedoor.v05.terraform-plan-summary.v1"
	TerraformPlanSummaryMode   = "OFFLINE_SANITIZED_ONLY"
	maxPlanJSONBytes           = 8 * 1024 * 1024
	maxOutputJSONBytes         = 1 * 1024 * 1024
	maxPlanChanges             = 4000
	maxOutputChanges           = 500
)

type TerraformActionCounts struct {
	Total   int `json:"total"`
	NoOp    int `json:"no_op"`
	Create  int `json:"create"`
	Update  int `json:"update"`
	Delete  int `json:"delete"`
	Replace int `json:"replace"`
	Read    int `json:"read"`
	Forget  int `json:"forget"`
}

type TerraformPlanSummary struct {
	SchemaVersion       string                `json:"schema_version"`
	Mode                string                `json:"mode"`
	CapturedUTC         string                `json:"captured_utc"`
	TerraformVersion    string                `json:"terraform_version"`
	FormatVersion       string                `json:"format_version"`
	SourceTreeSHA256    string                `json:"source_tree_sha256"`
	PlanJSONSHA256      string                `json:"plan_json_sha256"`
	OutputsJSONSHA256   string                `json:"outputs_json_sha256"`
	ResourceChanges     TerraformActionCounts `json:"resource_changes"`
	DriftChangeCount    int                   `json:"drift_change_count"`
	DeferredChangeCount int                   `json:"deferred_change_count"`
	OutputChangeCount   int                   `json:"output_change_count"`
	OutputCount         int                   `json:"output_count"`
	SensitiveOutputs    int                   `json:"sensitive_output_count"`
	PlanComplete        string                `json:"plan_complete"`
	PlanApplyable       string                `json:"plan_applyable"`
	Verdict             string                `json:"proof_verdict"`
	Limitations         []string              `json:"limitations"`
}

// Fields with potentially sensitive content (before/after, variables, prior
// state, configuration, planned values, output values) are purposefully not
// declared. encoding/json discards those fields instead of retaining them.
type terraformPlanSubset struct {
	FormatVersion    string    `json:"format_version"`
	TerraformVersion string    `json:"terraform_version"`
	PlannedValues    *struct{} `json:"planned_values"`
	Configuration    *struct{} `json:"configuration"`
	ResourceChanges  []struct {
		Address string `json:"address"`
		Mode    string `json:"mode"`
		Change  struct {
			Actions []string `json:"actions"`
		} `json:"change"`
	} `json:"resource_changes"`
	ResourceDrift             []struct{}          `json:"resource_drift"`
	DeferredChanges           []struct{}          `json:"deferred_changes"`
	OutputChanges             map[string]struct{} `json:"output_changes"`
	ActionInvocations         []struct{}          `json:"action_invocations"`
	DeferredActionInvocations []struct{}          `json:"deferred_action_invocations"`
	Complete                  *bool               `json:"complete"`
	Applyable                 *bool               `json:"applyable"`
	Errored                   *bool               `json:"errored"`
}

type terraformOutputSubset struct {
	Sensitive *bool `json:"sensitive"`
}

func validTerraformPlanFormat(s string) bool {
	if !strings.HasPrefix(s, "1.") || len(s) < 3 || len(s) > 12 {
		return false
	}
	for _, c := range s[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func booleanLabel(v *bool) string {
	if v == nil {
		return "not_reported"
	}
	if *v {
		return "true"
	}
	return "false"
}

func parseTerraformInput(raw []byte, max int, v any) error {
	if len(raw) == 0 || len(raw) > max {
		return errors.New("missing or oversized Terraform JSON input")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid Terraform JSON input")
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errors.New("multiple Terraform JSON values")
	} else if !errors.Is(err, io.EOF) {
		return errors.New("trailing invalid Terraform JSON")
	}
	return nil
}

func countAction(actions []string, counts *TerraformActionCounts) error {
	counts.Total++
	switch strings.Join(actions, ",") {
	case "no-op":
		counts.NoOp++
	case "create":
		counts.Create++
	case "update":
		counts.Update++
	case "delete":
		counts.Delete++
	case "delete,create", "create,delete":
		counts.Replace++
	case "read":
		counts.Read++
	case "forget":
		counts.Forget++
	default:
		return errors.New("unsupported Terraform action sequence")
	}
	return nil
}

func terraformPlanLimitations() []string {
	return []string{
		"Sanitized local JSON is unauthenticated; hashes are not signatures or proof of origin.",
		"Terraform planning is not applying; no live Kubernetes state or enforcement is proven.",
		"Only change counts and output sensitivity flags are retained; raw values must not be saved or published.",
	}
}

// SummarizeTerraformPlanJSON returns only metadata and counts. It does not
// persist either input, invoke Terraform, or claim that changes were applied.
// Pass the already verified source manifest from Slice 2A.
func SummarizeTerraformPlanJSON(planJSON, outputsJSON []byte, source TerraformSourceManifest) (TerraformPlanSummary, error) {
	var empty TerraformPlanSummary
	if err := VerifyTerraformSourceManifest(source); err != nil {
		return empty, fmt.Errorf("unverified Terraform source: %w", err)
	}
	var plan terraformPlanSubset
	if err := parseTerraformInput(planJSON, maxPlanJSONBytes, &plan); err != nil {
		return empty, err
	}
	if !validTerraformPlanFormat(plan.FormatVersion) || !validText(plan.TerraformVersion, 40) {
		return empty, errors.New("unsupported Terraform plan format/version")
	}
	// State JSON alone is not a plan. The plan JSON representation includes
	// configuration or planned values; the change array may be empty.
	if plan.PlannedValues == nil && plan.Configuration == nil {
		return empty, errors.New("input is not a Terraform plan representation")
	}
	if plan.Errored != nil && *plan.Errored {
		return empty, errors.New("errored Terraform plan rejected")
	}
	if len(plan.ResourceChanges) > maxPlanChanges || len(plan.ResourceDrift) > maxPlanChanges || len(plan.DeferredChanges) > maxPlanChanges || len(plan.OutputChanges) > maxOutputChanges {
		return empty, errors.New("Terraform plan exceeds bounded change limits")
	}
	if len(plan.ActionInvocations) > 0 || len(plan.DeferredActionInvocations) > 0 {
		return empty, errors.New("Terraform action invocations are outside v0.5 sanitizer scope")
	}
	var outputs map[string]terraformOutputSubset
	if err := parseTerraformInput(outputsJSON, maxOutputJSONBytes, &outputs); err != nil {
		return empty, err
	}
	if outputs == nil || len(outputs) > maxOutputChanges {
		return empty, errors.New("invalid Terraform outputs object/count")
	}
	var counts TerraformActionCounts
	for _, rc := range plan.ResourceChanges {
		if !validText(rc.Address, 300) || (rc.Mode != "managed" && rc.Mode != "data") {
			return empty, errors.New("invalid Terraform resource change identity")
		}
		if err := countAction(rc.Change.Actions, &counts); err != nil {
			return empty, err
		}
	}
	sensitive := 0
	for name, out := range outputs {
		if !validText(name, 256) {
			return empty, errors.New("invalid Terraform output name")
		}
		if out.Sensitive == nil {
			return empty, errors.New("Terraform output is missing sensitive flag")
		}
		if *out.Sensitive {
			sensitive++
		}
	}
	p := sha256.Sum256(planJSON)
	o := sha256.Sum256(outputsJSON)
	summary := TerraformPlanSummary{
		SchemaVersion: TerraformPlanSummarySchema, Mode: TerraformPlanSummaryMode,
		CapturedUTC:      time.Now().UTC().Format(time.RFC3339Nano),
		TerraformVersion: plan.TerraformVersion, FormatVersion: plan.FormatVersion,
		SourceTreeSHA256: source.TreeSHA256,
		PlanJSONSHA256:   hex.EncodeToString(p[:]), OutputsJSONSHA256: hex.EncodeToString(o[:]),
		ResourceChanges: counts, DriftChangeCount: len(plan.ResourceDrift),
		DeferredChangeCount: len(plan.DeferredChanges), OutputChangeCount: len(plan.OutputChanges),
		OutputCount: len(outputs), SensitiveOutputs: sensitive,
		PlanComplete: booleanLabel(plan.Complete), PlanApplyable: booleanLabel(plan.Applyable),
		Verdict: Inconclusive, Limitations: terraformPlanLimitations(),
	}
	if err := VerifyTerraformPlanSummary(summary); err != nil {
		return empty, err
	}
	return summary, nil
}

// Verification checks a SAVED sanitized summary's internal consistency.
// It cannot authenticate the inputs or prove that Terraform was applied.
func VerifyTerraformPlanSummary(s TerraformPlanSummary) error {
	if s.SchemaVersion != TerraformPlanSummarySchema || s.Mode != TerraformPlanSummaryMode || !validUTC(s.CapturedUTC) {
		return errors.New("invalid Terraform plan summary schema/mode/timestamp")
	}
	if !validTerraformPlanFormat(s.FormatVersion) || !validText(s.TerraformVersion, 40) ||
		!validHex(s.SourceTreeSHA256, 64) || !validHex(s.PlanJSONSHA256, 64) || !validHex(s.OutputsJSONSHA256, 64) {
		return errors.New("invalid Terraform plan summary source/version/digests")
	}
	c := s.ResourceChanges
	if c.Total < 0 || c.Total > maxPlanChanges || c.NoOp < 0 || c.Create < 0 || c.Update < 0 || c.Delete < 0 || c.Replace < 0 || c.Read < 0 || c.Forget < 0 ||
		c.Total != c.NoOp+c.Create+c.Update+c.Delete+c.Replace+c.Read+c.Forget {
		return errors.New("inconsistent Terraform action counts")
	}
	if s.DriftChangeCount < 0 || s.DriftChangeCount > maxPlanChanges || s.DeferredChangeCount < 0 || s.DeferredChangeCount > maxPlanChanges ||
		s.OutputChangeCount < 0 || s.OutputChangeCount > maxOutputChanges || s.OutputCount < 0 || s.OutputCount > maxOutputChanges ||
		s.SensitiveOutputs < 0 || s.SensitiveOutputs > s.OutputCount {
		return errors.New("invalid Terraform output/drift counts")
	}
	validLabel := func(v string) bool { return v == "true" || v == "false" || v == "not_reported" }
	if !validLabel(s.PlanComplete) || !validLabel(s.PlanApplyable) || s.Verdict != Inconclusive ||
		len(s.Limitations) != 3 || !checkLimitations(s.Limitations) ||
		s.Limitations[0] != terraformPlanLimitations()[0] ||
		s.Limitations[1] != terraformPlanLimitations()[1] ||
		s.Limitations[2] != terraformPlanLimitations()[2] {
		return errors.New("invalid Terraform verdict or limitations")
	}
	return nil
}

// TerraformPlanSourceSummary is a reference to a local untrusted JSON input.
// It is not a reference to state, an authenticated run, or an applied plan.
func TerraformPlanSourceSummary(s TerraformPlanSummary) (SourceSummary, error) {
	if err := VerifyTerraformPlanSummary(s); err != nil {
		return SourceSummary{}, err
	}
	return SourceSummary{Kind: "terraform_plan_json", Reference: "offline-ephemeral-plan-input", SHA256: s.PlanJSONSHA256}, nil
}
