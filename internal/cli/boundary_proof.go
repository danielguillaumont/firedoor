package cli

import (
	"errors"
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/spf13/cobra"
)

// No path or format flags: the exporter accepts only one previously saved
// detached report and writes one bounded local JSON/HTML pair, once.
func newBoundaryProofExportCommand() *cobra.Command {
	var consent string
	var root string
	cmd := &cobra.Command{
		Use:   "export-boundary",
		Short: "Offline, consent-gated one-time export of an INCONCLUSIVE historical Boundary Proof",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonPath, htmlPath, err := prooflens.ExportApprovedBoundaryProofOnce(root, consent)
			if err != nil {
				// The underlying error and all source metadata are deliberately hidden.
				return errors.New("offline Boundary Proof export refused; confirm approved local report, consent, ignored paths and unused output filenames")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "PASS: Offline Boundary Proof exported once.\nJSON: %s\nHTML: %s\nVerdict: INCONCLUSIVE — detached historical consistency only; no enforcement claim.\nNo Kubernetes, Terraform, network or evidence collection.\n", jsonPath, htmlPath)
			return err
		},
	}
	cmd.Flags().StringVar(&consent, "consent", "", "Exact approval for offline reading of one fixed ignored historical report")
	cmd.Flags().StringVar(&root, "root", ".", "Firedoor repository root (not an evidence or export path)")
	return cmd
}
