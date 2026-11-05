package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/evidence"
	"github.com/spf13/cobra"
)

func newEvidenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Inspect locally captured validation evidence (read-only)",
	}
	cmd.AddCommand(newEvidenceVerifyCommand())
	return cmd
}

func newEvidenceVerifyCommand() *cobra.Command {
	var baseline, hardened, format string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Compare v0.1 baseline and hardened connectivity JSON records",
		Long:  "Checks saved PowerShell evidence for complete and internally consistent before/after observations. Does not contact Kubernetes, run probes, or prove live enforcement.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "human" && format != "json" {
				return fmt.Errorf("invalid format %q: choose human or json", format)
			}
			result, err := evidence.VerifyFiles(baseline, hardened)
			if err != nil {
				return fmt.Errorf("evidence verification rejected: %w", err)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			lines := []string{
				"FIREDOOR v0.1 / Go evidence consistency verifier / READ ONLY",
				"PASS: two distinct local reports, correct lab identity, chronology and unchanged targets.",
				"PASS: baseline connected 6/6; hardened connected 4/6 and unreachable twice 2/6.",
				"Verdict: " + result.Status,
				"Limit: " + result.Limitations,
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
			return err
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "docs/evidence/local/v01-terraform-baseline-connectivity.json", "Baseline JSON evidence file")
	cmd.Flags().StringVar(&hardened, "hardened", "docs/evidence/local/v01-terraform-hardened-connectivity.json", "Hardened JSON evidence file")
	cmd.Flags().StringVar(&format, "format", "human", "Output: human or json")
	return cmd
}
