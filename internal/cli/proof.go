package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// proof command defaults to an inert preview; active operations require a
// named subcommand and the exact consent string. No Terraform/apply/probes.
func newProofCommand() *cobra.Command {
	proof := &cobra.Command{
		Use: "proof", Short: "Proof Lens evidence reporting (local development lab)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				Mode               string `json:"mode"`
				ConsentRequired    bool   `json:"consent_required"`
				ReportSaved        bool   `json:"report_saved"`
				TerraformExecuted  bool   `json:"terraform_executed"`
				KubernetesObserved bool   `json:"kubernetes_observed"`
				ProofVerdict       string `json:"proof_verdict"`
			}{"REVIEW_ONLY_NO_PROOF_ACCESS", true, false, false, false, prooflens.Inconclusive})
		},
	}
	var root, receiptID, consent, format string
	var export bool
	report := &cobra.Command{
		Use: "report", Short: "Generate a read-only terminal, Markdown or JSON evidence timeline",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "text" && format != "markdown" && format != "json" {
				return errors.New("Proof Lens format rejected; choose text, markdown or json")
			}
			if export && format != "markdown" {
				return errors.New("--export-markdown requires --format markdown")
			}
			if strings.TrimSpace(receiptID) == "" || consent != prooflens.ApprovedReportConsent {
				return errors.New("Proof Lens report requires exact receipt ID and explicit read-only consent")
			}
			r, err := prooflens.CollectApprovedProofReport(root, receiptID, consent)
			if err != nil {
				return errors.New("Proof Lens evidence inspection rejected; check approved lab, receipt and recovery journals")
			}
			if export {
				rel, err := prooflens.ExportVerifiedMarkdownOnce(root, r)
				if err != nil {
					return errors.New("Proof Lens local Markdown export refused; check Git-ignore, path and existing report")
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "PASS: Verified local Markdown report saved once: %s\nVERDICT: INCONCLUSIVE (historical context only)\n", rel)
				return err
			}
			switch format {
			case "text":
				value, err := prooflens.RenderProofReportText(r)
				if err != nil {
					return errors.New("invalid report")
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), value)
				return err
			case "markdown":
				value, err := prooflens.RenderProofReportMarkdown(r)
				if err != nil {
					return errors.New("invalid report")
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), value)
				return err
			default:
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}
		},
	}
	report.Flags().StringVar(&root, "root", ".", "Firedoor checkout root")
	report.Flags().StringVar(&receiptID, "receipt-id", "", "Exact existing sanitized Terraform receipt ID")
	report.Flags().StringVar(&consent, "consent", "", "Exact approval for read-only local evidence access")
	report.Flags().StringVar(&format, "format", "text", "text, markdown or json")
	report.Flags().BoolVar(&export, "export-markdown", false, "Create the one fixed Git-ignored Markdown report only (never overwrite)")
	proof.AddCommand(report, newBoundaryProofExportCommand())
	return proof
}
