package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is overridden by -ldflags at release time. It is not a claim of a shipped product.
var Version = "0.0.0-dev"

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "firedoor",
		Short:         "Evidence-first Kubernetes security validation (in development)",
		Long:          "Firedoor links Terraform security intent, Kubernetes observations and bounded validation. v0.1 verifies previously captured network evidence; v0.2 audits owned Kubernetes configuration. v0.3 verifies saved observations collected by bounded Go probes in the owned lab; neither configuration inspection nor observed reachability proves policy causality. v0.4 validates bounded A/B/A attribution evidence and recovery journals offline. v0.5 Proof Lens provides consent-gated, read-only historical evidence reporting; its current security verdict is inconclusive.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = Version
	root.AddCommand(newVersionCommand(), newDoctorCommand(), newEvidenceCommand(), newInspectCommand(), newAuditCommand(), newVerifyCommand(), newAttributionCommand(), newProofCommand(), newConsoleAPICommand(), newContainmentMapCommand())
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the development version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), Version)
			return err
		},
	}
}
