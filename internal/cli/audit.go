package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/danielguillaumont/firedoor/internal/audit"
	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
	"github.com/spf13/cobra"
)

// newAuditCommand performs only the scoped kubectl/kind reads already
// allowlisted by the inspector and then evaluates the resulting snapshot.
func newAuditCommand() *cobra.Command {
	var format, profile string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Evaluate six configuration controls in the owned Firedoor testbed (read-only)",
		Long:  "Read the three Firedoor-owned Kubernetes zones and produce 18 evidence-backed configuration findings. This does not execute network probes, prove effective RBAC or CNI enforcement, or verify Terraform state. The supplied profile is an unverified label.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "human" && format != "json" {
				return fmt.Errorf("invalid format %q: choose human or json", format)
			}
			if profile != "baseline" && profile != "hardened" {
				return fmt.Errorf("--profile must be baseline or hardened; this does not verify Terraform state")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			snapshot, err := (kubeinspect.Collector{Runner: kubeinspect.CLI{}}).Collect(ctx)
			if err != nil {
				return fmt.Errorf("owned-cluster audit refused: %w", err)
			}
			report, err := audit.Build(snapshot, profile)
			if err != nil {
				return fmt.Errorf("control evaluation refused: %w", err)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), audit.Human(report))
			return err
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Operator-reported Terraform profile: baseline or hardened (required; unverified)")
	cmd.Flags().StringVar(&format, "format", "human", "Output format: human or json")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}
