package prooflens

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Slice 2B2C: guarded local speculative plan adapter. The adapter has no
// implicit execution path: an explicit caller-provided consent string and a
// successful, fresh owned-lab admission are mandatory. It never applies.
const (
	TerraformLivePlanSchema  = "firedoor.v05.live-speculative-plan.v1"
	TerraformLivePlanMode    = "LOCAL_SPECULATIVE_PLAN_SANITIZED"
	TerraformPlanConsent     = "I-APPROVE-LOCAL-HARDENED-TERRAFORM-PLAN"
	terraformPlanMaxDuration = 120 * time.Second
)

type TerraformLivePlanReport struct {
	SchemaVersion        string                 `json:"schema_version"`
	Mode                 string                 `json:"mode"`
	SpeculativeOnly      bool                   `json:"speculative_only"`
	PlanCompleted        bool                   `json:"plan_completed"`
	Applied              bool                   `json:"applied"`
	RawOutputSaved       bool                   `json:"raw_output_saved"`
	SourceTreeSHA256     string                 `json:"source_tree_sha256"`
	SanitizedPlanSummary TerraformUIPlanSummary `json:"sanitized_plan_summary"`
	Limitations          []string               `json:"limitations"`
}

// terraformPlanProcess abstracts a started process for unit tests. No test
// invokes terraform or a real provider. Terminate must be safe to call twice.
type terraformPlanProcess struct {
	output    io.ReadCloser
	wait      func() error
	terminate func()
}

type terraformPlanStarter func(context.Context, string, []string, string) (terraformPlanProcess, error)

func rejectedPlan() error {
	return errors.New("guarded Terraform plan refused; no raw diagnostics are available")
}

func checkPlanTestbed(root string) (string, error) {
	working := filepath.Join(root, "infrastructure", "terraform", "testbed")
	for _, path := range []string{working, filepath.Join(working, ".terraform"),
		filepath.Join(working, "main.tf"), filepath.Join(working, "versions.tf"),
		filepath.Join(working, ".terraform.lock.hcl"), filepath.Join(working, "terraform.tfstate")} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", rejectedPlan()
		}
		if (path == working || path == filepath.Join(working, ".terraform")) != info.IsDir() {
			return "", rejectedPlan()
		}
		if !info.IsDir() && (!info.Mode().IsRegular() || info.Size() == 0) {
			return "", rejectedPlan()
		}
	}
	if _, err := os.Lstat(filepath.Join(working, ".terraform.tfstate.lock.info")); !os.IsNotExist(err) {
		return "", rejectedPlan()
	}
	return working, nil
}

// ambientTerraformSettingsAllowed denies variables that can silently inject
// CLI arguments, override the working directory, or change provider behavior.
func ambientTerraformSettingsAllowed(environ []string) bool {
	for _, item := range environ {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "TF_") && key != "TF_IN_AUTOMATION" && key != "TF_INPUT" {
			return false
		}
	}
	return true
}

func minimalTerraformEnvironment(environ []string) []string {
	allowed := map[string]bool{
		"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "COMSPEC": true,
		"WINDIR": true, "TEMP": true, "TMP": true, "USERPROFILE": true,
		"HOME": true, "APPDATA": true, "LOCALAPPDATA": true, "KUBECONFIG": true,
		"SYSTEMDRIVE": true, "PROGRAMDATA": true,
	}
	result := make([]string, 0, 16)
	for _, item := range environ {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(key)] {
			result = append(result, item)
		}
	}
	return append(result, "TF_IN_AUTOMATION=1", "TF_INPUT=0")
}

func startTerraformPlan(ctx context.Context, working string, args []string, binary string) (terraformPlanProcess, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = working
	cmd.Env = minimalTerraformEnvironment(os.Environ())
	cmd.Stdin = nil
	cmd.Stderr = io.Discard // NEVER forward provider diagnostics or state-derived errors.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return terraformPlanProcess{}, rejectedPlan()
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return terraformPlanProcess{}, rejectedPlan()
	}
	return terraformPlanProcess{output: stdout, wait: cmd.Wait, terminate: func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill() // Best-effort parent termination; providers may survive on Windows.
		}
	}}, nil
}

// collectGuardedTerraformPlan is kept injectable for offline negative tests.
// It requires both explicit approval and a fresh independent lab admission.
func collectGuardedTerraformPlan(ctx context.Context, root string, source TerraformSourceManifest,
	admission TerraformPlanAdmission, consent string, labAdmission func(context.Context, string) error,
	starter terraformPlanStarter, environ []string) (TerraformLivePlanReport, error) {
	var empty TerraformLivePlanReport
	if ctx == nil || consent != TerraformPlanConsent || labAdmission == nil || starter == nil ||
		!ambientTerraformSettingsAllowed(environ) || VerifyTerraformPlanAdmission(admission) != nil ||
		VerifyTerraformSourceManifest(source) != nil || source.TreeSHA256 != admission.SourceTreeSHA256 {
		return empty, rejectedPlan()
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil || RecheckTerraformSources(absoluteRoot, source) != nil {
		return empty, rejectedPlan()
	}
	working, err := checkPlanTestbed(absoluteRoot)
	if err != nil {
		return empty, rejectedPlan()
	}
	limited, cancel := context.WithTimeout(ctx, terraformPlanMaxDuration)
	defer cancel()
	if err := labAdmission(limited, absoluteRoot); err != nil || limited.Err() != nil {
		return empty, rejectedPlan()
	}
	// Recheck after all external admission calls, before the actual launch.
	if RecheckTerraformSources(absoluteRoot, source) != nil {
		return empty, rejectedPlan()
	}
	if _, err := checkPlanTestbed(absoluteRoot); err != nil {
		return empty, rejectedPlan()
	}
	proc, err := starter(limited, working, append([]string{}, admission.Arguments...), admission.Executable)
	if err != nil || proc.output == nil || proc.wait == nil || proc.terminate == nil {
		return empty, rejectedPlan()
	}
	// The reducer receives raw stdout in memory, but never logs it, saves it,
	// or copies any unapproved keys into its summary. On early failure, kill
	// the process to prevent a blocked pipe from deadlocking Wait.
	summary, parseErr := SummarizeTerraformUIStream(proc.output, source)
	if parseErr != nil || limited.Err() != nil {
		proc.terminate()
	}
	waitErr := proc.wait()
	_ = proc.output.Close()
	if parseErr != nil || waitErr != nil || limited.Err() != nil ||
		RecheckTerraformSources(absoluteRoot, source) != nil ||
		VerifyTerraformUIPlanSummary(summary) != nil || summary.SourceTreeSHA256 != admission.SourceTreeSHA256 {
		return empty, rejectedPlan()
	}
	return TerraformLivePlanReport{
		SchemaVersion:   TerraformLivePlanSchema,
		Mode:            TerraformLivePlanMode,
		SpeculativeOnly: true, PlanCompleted: true, Applied: false, RawOutputSaved: false,
		SourceTreeSHA256: source.TreeSHA256, SanitizedPlanSummary: summary,
		Limitations: []string{
			"A speculative plan may read Terraform state and provider resources; it does not apply or prove Kubernetes enforcement.",
			"Raw UI messages and stderr are discarded, not saved; the SHA256 stream hash is not an authentication claim.",
			"After abnormal termination, inspect local Terraform lock metadata and any surviving provider processes before another run.",
		},
	}, nil
}

// RunGuardedTerraformPlan is the only production entry point for execution.
// It runs the v0.5 owned-lab admission with all output suppressed, then the
// fixed allowlisted Terraform CLI command. It never starts a shell command
// assembled from user-supplied Terraform arguments.
func RunGuardedTerraformPlan(ctx context.Context, root string, source TerraformSourceManifest,
	admission TerraformPlanAdmission, consent string) (TerraformLivePlanReport, error) {
	gate := func(ctx context.Context, root string) error {
		// The v0.5 admission script verifies owned cluster, witness cleanup,
		// hardened policy inventory, default workspace, state metadata and
		// Terraform source drift. No output or diagnostics are retained.
		path := filepath.Join(root, "scripts", "v05", "Review-FiredoorV05PlanAdmission.ps1")
		if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
			return rejectedPlan()
		}
		cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		if cmd.Run() != nil {
			return rejectedPlan()
		}
		return nil
	}
	return collectGuardedTerraformPlan(ctx, root, source, admission, consent, gate, startTerraformPlan, os.Environ())
}
