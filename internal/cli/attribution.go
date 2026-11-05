package cli

import (
	"encoding/json"
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/attribution"
	"github.com/spf13/cobra"
)

// The attribution command is intentionally offline and read-only. Live
// policy mutation remains in the separate, explicit lab-only helper.
func newAttributionCommand() *cobra.Command {
	var edgeReport, edgeJournal, dataReport, dataJournal, format string
	cmd := &cobra.Command{
		Use:   "attribution",
		Short: "Validate two saved v0.4 A/B/A experiments and recovery journals (offline)",
		Long:  "Read two explicit, existing local experiment reports and matching recovery journals. Validate fixed experiment identities, observed A/B/A behavior, witness UID, cleanup and journal integrity. Never contacts Kubernetes, runs probes, or creates policies. Independent live inventory remains required.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "human" && format != "json" {
				return fmt.Errorf("invalid --format %q: choose human or json", format)
			}
			summary, err := attribution.VerifyEvidenceBundle(
				attribution.EvidencePair{ReportPath: edgeReport, JournalPath: edgeJournal},
				attribution.EvidencePair{ReportPath: dataReport, JournalPath: dataJournal})
			if err != nil {
				return fmt.Errorf("v0.4 attribution evidence rejected: %w", err)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), attribution.HumanEvidenceBundle(summary))
			return err
		},
	}
	cmd.Flags().StringVar(&edgeReport, "edge-report", "", "Saved Edge→Data live JSON report (required)")
	cmd.Flags().StringVar(&edgeJournal, "edge-journal", "", "Matching Edge→Data JSONL recovery journal (required)")
	cmd.Flags().StringVar(&dataReport, "data-report", "", "Saved Data→App live JSON report (required)")
	cmd.Flags().StringVar(&dataJournal, "data-journal", "", "Matching Data→App JSONL recovery journal (required)")
	cmd.Flags().StringVar(&format, "format", "human", "Output format: human or json")
	for _, flag := range []string{"edge-report", "edge-journal", "data-report", "data-journal"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}
