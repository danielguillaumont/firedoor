package cli

import (
	"encoding/json"
	"fmt"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
	"github.com/spf13/cobra"
)

// newVerifyCommand verifies saved, already-captured live observations.
// It does not contact a cluster, execute probes, or verify Terraform state.
func newVerifyCommand() *cobra.Command {
	var input, profile, format string
	var requireMatch bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify saved v0.3 network observations against the six approved routes (offline)",
		Long:  "Read an existing local JSON report from the owned Firedoor lab, validate its integrity and compare observed HTTP behavior with the baseline or hardened expectations. This is offline verification, NOT a live probe or proof of NetworkPolicy causality. The profile label must be independently confirmed at collection time.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "human" && format != "json" {
				return fmt.Errorf("invalid --format %q: choose human or json", format)
			}
			report, err := liveverify.VerifyFile(input, liveverify.Profile(profile))
			if err != nil {
				return fmt.Errorf("v0.3 evidence rejected: %w", err)
			}
			if format == "json" {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprint(cmd.OutOrStdout(), liveverify.HumanVerificationReport(report)); err != nil {
					return err
				}
			}
			// By default a successfully generated report exits 0, even if it
			// describes a mismatch. CI can request a failing exit code while
			// still receiving the full human/JSON report on stdout.
			if requireMatch && report.Status != "matched" {
				return fmt.Errorf("verification status %q requires review (--require-match)", report.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Path to the saved v0.3 live-observation JSON file (required)")
	cmd.Flags().StringVar(&profile, "profile", "", "Approved expected profile: baseline or hardened (required; operator supplied)")
	cmd.Flags().StringVar(&format, "format", "human", "Output: human or json")
	cmd.Flags().BoolVar(&requireMatch, "require-match", false, "Exit nonzero for valid mismatched or inconclusive findings (CI mode)")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}
