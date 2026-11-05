// proofreport is the Slice 4A legacy entry point; the first-class CLI also
// uses CollectApprovedProofReport, so both paths share identical validation.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	_ "modernc.org/sqlite"
)

func run() error {
	root := flag.String("root", ".", "Firedoor repository root")
	report := flag.Bool("report", false, "Consent-gated read-only evidence report")
	format := flag.String("format", "text", "Output: text, markdown, json")
	id := flag.String("receipt-id", "", "Existing Terraform receipt ID")
	consent := flag.String("consent", "", "Exact read-only consent phrase")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected CLI argument")
	}
	if !*report {
		if *id != "" || *consent != "" || *format != "text" {
			return errors.New("report flags require --report")
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Mode               string `json:"mode"`
			LocalFilesOpened   bool   `json:"local_files_opened"`
			DatabaseOpened     bool   `json:"database_opened"`
			KubernetesObserved bool   `json:"kubernetes_observed"`
			TerraformExecuted  bool   `json:"terraform_executed"`
			WitnessReplayed    bool   `json:"witness_replayed"`
			ReportSaved        bool   `json:"report_saved"`
			ConsentRequired    bool   `json:"consent_required"`
			ProofVerdict       string `json:"proof_verdict"`
		}{"REVIEW_ONLY_NO_REPORT_ACCESS", false, false, false, false, false, false, true, prooflens.Inconclusive})
	}
	if *format != "text" && *format != "markdown" && *format != "json" {
		return errors.New("unapproved format")
	}
	r, err := prooflens.CollectApprovedProofReport(*root, *id, *consent)
	if err != nil {
		return err
	}
	switch *format {
	case "text":
		s, e := prooflens.RenderProofReportText(r)
		if e != nil {
			return e
		}
		_, e = fmt.Print(s)
		return e
	case "markdown":
		s, e := prooflens.RenderProofReportMarkdown(r)
		if e != nil {
			return e
		}
		_, e = fmt.Print(s)
		return e
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "Firedoor Proof Lens report refused; check branch, consent, receipt, historical journals and owned lab")
		os.Exit(1)
	}
}
