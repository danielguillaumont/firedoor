package prooflens

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planRunnerFixture(t *testing.T) (string, TerraformSourceManifest, TerraformPlanAdmission) {
	t.Helper()
	root := tfFixture(t)
	working := filepath.Join(root, "infrastructure", "terraform", "testbed")
	if err := os.Mkdir(filepath.Join(working, ".terraform"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"versions.tf", ".terraform.lock.hcl", "terraform.tfstate"} {
		if err := os.WriteFile(filepath.Join(working, name), []byte("SYNTHETIC DATA ONLY"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := PrepareTerraformPlanAdmission(root, source)
	if err != nil {
		t.Fatal(err)
	}
	return root, source, admission
}

func TestPlanRunnerRequiresConsentAndFreshIndependentLabAdmission(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	started, gated := 0, 0
	gate := func(context.Context, string) error { gated++; return nil }
	start := func(context.Context, string, []string, string) (terraformPlanProcess, error) {
		started++
		return terraformPlanProcess{output: io.NopCloser(strings.NewReader(syntheticUILines)), wait: func() error { return nil }, terminate: func() {}}, nil
	}
	for _, consent := range []string{"", "yes", "I-APPROVE-PRODUCTION", "I-APPROVE-LOCAL-HARDENED-TERRAFORM-PLAN-extra"} {
		if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, consent, gate, start, nil); err == nil {
			t.Fatalf("invalid consent accepted: %q", consent)
		}
	}
	if started != 0 || gated != 0 {
		t.Fatalf("execution or gate bypassed consent: %d %d", started, gated)
	}
	rejectingGate := func(context.Context, string) error { return errors.New("PRIVATE ERROR FROM LAB") }
	if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, rejectingGate, start, nil); err == nil || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatal("gate error leaked or ignored")
	}
	if started != 0 {
		t.Fatal("process started after rejected preflight")
	}
}

func TestPlanRunnerSuccessIsSanitizedAndOnlySpeculative(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	started, terminated := 0, false
	gate := func(context.Context, string) error { return nil }
	start := func(_ context.Context, dir string, args []string, binary string) (terraformPlanProcess, error) {
		started++
		if binary != "terraform" || dir != filepath.Join(root, "infrastructure", "terraform", "testbed") {
			t.Fatal("wrong executable or directory")
		}
		if strings.Join(args, " ") != strings.Join(fixedPlanArguments(), " ") {
			t.Fatal("different arguments")
		}
		return terraformPlanProcess{output: io.NopCloser(strings.NewReader(syntheticUILines)), wait: func() error { return nil }, terminate: func() { terminated = true }}, nil
	}
	report, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	if started != 1 || terminated || !report.PlanCompleted || !report.SpeculativeOnly || report.Applied || report.RawOutputSaved ||
		report.SanitizedPlanSummary.ProofVerdict != Inconclusive || report.SanitizedPlanSummary.Changes.Change != 1 ||
		report.SourceTreeSHA256 != source.TreeSHA256 {
		t.Fatalf("unsafe report: %+v", report)
	}
	bytes, _ := json.Marshal(report)
	if strings.Contains(string(bytes), "VERY_PRIVATE") || strings.Contains(string(bytes), "SECRET_LEAK") ||
		strings.Contains(string(bytes), "@message") || strings.Contains(string(bytes), "tfstate") {
		t.Fatalf("raw/sensitive information leaked into report: %s", string(bytes))
	}
}

func TestPlanRunnerNeverPublishesRawFailure(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	gate := func(context.Context, string) error { return nil }
	for _, tc := range []struct {
		name, input string
		waitErr     error
	}{
		{"invalid JSON with sensitive text", "PRIVATE_API_KEY=SECRET_LEAK\n", nil},
		{"truncated UI", `{"type":"version","terraform":"1.17.1","ui":"1.2"}` + "\n", nil},
		{"nonzero process", syntheticUILines, errors.New("PROVIDER SECRET_LEAK")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terminated := false
			start := func(context.Context, string, []string, string) (terraformPlanProcess, error) {
				return terraformPlanProcess{output: io.NopCloser(strings.NewReader(tc.input)), wait: func() error { return tc.waitErr }, terminate: func() { terminated = true }}, nil
			}
			report, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, nil)
			if err == nil || report.Mode != "" || strings.Contains(err.Error(), "SECRET_LEAK") || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("unsafe result: report=%+v err=%v", report, err)
			}
			if tc.waitErr == nil && !terminated {
				t.Fatal("malformed stream did not cancel")
			}
		})
	}
}

func TestPlanRunnerFailClosedOnAmbientInjectionAndSourceDrift(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	started := 0
	gate := func(context.Context, string) error { return nil }
	start := func(context.Context, string, []string, string) (terraformPlanProcess, error) {
		started++
		return terraformPlanProcess{output: io.NopCloser(strings.NewReader(syntheticUILines)), wait: func() error { return nil }, terminate: func() {}}, nil
	}
	for _, env := range [][]string{
		{"TF_CLI_ARGS_plan=-destroy"}, {"tf_var_secret=PRIVATE"}, {"TF_WORKSPACE=prod"}, {"TF_CLI_CONFIG_FILE=override"},
	} {
		if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, env); err == nil {
			t.Fatalf("injected env accepted: %v", env)
		}
	}
	if started != 0 {
		t.Fatal("process started despite ambient injection")
	}
	p := filepath.Join(root, "infrastructure", "terraform", "testbed", "main.tf")
	if err := os.WriteFile(p, []byte("# Drifted source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, nil); err == nil {
		t.Fatal("source drift accepted")
	}
}

func TestPlanRunnerFailsOnLockOrCancelledContext(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	started := 0
	gate := func(context.Context, string) error { return nil }
	start := func(context.Context, string, []string, string) (terraformPlanProcess, error) {
		started++
		return terraformPlanProcess{}, errors.New("PRIVATE")
	}
	working := filepath.Join(root, "infrastructure", "terraform", "testbed")
	if err := os.WriteFile(filepath.Join(working, ".terraform.tfstate.lock.info"), []byte("PRIVATE"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, nil); err == nil || started != 0 {
		t.Fatal("state lock marker accepted")
	}
	if err := os.Remove(filepath.Join(working, ".terraform.tfstate.lock.info")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := collectGuardedTerraformPlan(ctx, root, source, a, TerraformPlanConsent, gate, start, nil); err == nil || started != 0 {
		t.Fatal("cancelled context accepted")
	}
}

func TestPlanRunnerPostStartDriftIsNotReportedAsSuccess(t *testing.T) {
	root, source, a := planRunnerFixture(t)
	gate := func(context.Context, string) error { return nil }
	p := filepath.Join(root, "infrastructure", "terraform", "testbed", "main.tf")
	start := func(context.Context, string, []string, string) (terraformPlanProcess, error) {
		return terraformPlanProcess{output: io.NopCloser(strings.NewReader(syntheticUILines)), wait: func() error {
			return os.WriteFile(p, []byte("# drift during plan\n"), 0600)
		}, terminate: func() {}}, nil
	}
	if _, err := collectGuardedTerraformPlan(context.Background(), root, source, a, TerraformPlanConsent, gate, start, nil); err == nil {
		t.Fatal("source changed during plan and still returned success")
	}
}

func TestMinimalEnvironmentRemovesUnapprovedCredentials(t *testing.T) {
	got := strings.Join(minimalTerraformEnvironment([]string{
		"PATH=C:\\Bin", "AWS_SECRET_ACCESS_KEY=PRIVATE", "TF_CLI_ARGS_plan=-destroy", "KUBECONFIG=C:\\owned", "USERPROFILE=C:\\Users\\Owner",
	}), "\n")
	if strings.Contains(got, "PRIVATE") || strings.Contains(got, "-destroy") || !strings.Contains(got, "TF_INPUT=0") || !strings.Contains(got, "KUBECONFIG=") {
		t.Fatal("environment filter failed")
	}
}
