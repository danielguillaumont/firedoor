package cli

import (
	"errors"
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/spf13/cobra"
)

// The command deliberately accepts no arbitrary input/output/format flags.
// Its two independently supplied private inputs must already exist.
func newBoundaryComparisonExportCommand() *cobra.Command {
	var root, consent string
	cmd := &cobra.Command{
		Use:   "export-comparison",
		Short: "Consent-gated offline comparison export (INCONCLUSIVE, drift NOT ASSESSED)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonPath, htmlPath, err := prooflens.ExportApprovedBoundaryComparisonOnce(root, consent)
			if err != nil {
				return errors.New("offline comparison export refused; check exact consent, two distinct reviewed local proofs, ignored paths, and unused output filenames")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "PASS: Offline Boundary Proof comparison exported once.\nJSON: %s\nHTML: %s\nVerdict: INCONCLUSIVE; drift NOT ASSESSED. Digest relationships do not prove enforcement.\nNo Kubernetes, Terraform, network or evidence collection.\n", jsonPath, htmlPath)
			return err
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Firedoor repository root (not a source or export path)")
	cmd.Flags().StringVar(&consent, "consent", "", "Exact approval for reading two private proofs and creating fixed local comparison outputs")
	return cmd
}
