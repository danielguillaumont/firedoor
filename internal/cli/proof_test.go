package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func proofTestCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestProofLensCLIInertDefaultAndSubcommandRegistration(t *testing.T) {
	output, err := proofTestCommand(t, "proof")
	if err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Mode               string `json:"mode"`
		ConsentRequired    bool   `json:"consent_required"`
		ReportSaved        bool   `json:"report_saved"`
		TerraformExecuted  bool   `json:"terraform_executed"`
		KubernetesObserved bool   `json:"kubernetes_observed"`
		ProofVerdict       string `json:"proof_verdict"`
	}
	if err := json.Unmarshal([]byte(output), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Mode != "REVIEW_ONLY_NO_PROOF_ACCESS" || !preview.ConsentRequired || preview.ReportSaved || preview.TerraformExecuted || preview.KubernetesObserved || preview.ProofVerdict != "inconclusive" {
		t.Fatal("inert preview overstated or accessed evidence")
	}
	proof, _, err := NewRootCommand().Find([]string{"proof", "report"})
	if err != nil || proof.Name() != "report" {
		t.Fatalf("missing proof report subcommand: %v", err)
	}
	for _, name := range []string{"root", "receipt-id", "consent", "format", "export-markdown"} {
		if proof.Flags().Lookup(name) == nil {
			t.Fatalf("missing %s safety flag", name)
		}
	}
}

func TestProofLensCLIRejectsInvalidOrUnconsentedRequests(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no consent", []string{"proof", "report", "--receipt-id", "TFPLAN-12345678"}, "consent"},
		{"wrong consent", []string{"proof", "report", "--receipt-id", "TFPLAN-12345678", "--consent", "yes"}, "consent"},
		{"no receipt", []string{"proof", "report", "--consent", "I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT"}, "receipt"},
		{"bad format", []string{"proof", "report", "--format", "html"}, "format"},
		{"export json", []string{"proof", "report", "--format", "json", "--export-markdown"}, "markdown"},
		{"no report destination", []string{"proof", "report", "--out", "other.md"}, "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := proofTestCommand(t, tc.args...)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected %s, got %v", tc.want, err)
			}
		})
	}
}
