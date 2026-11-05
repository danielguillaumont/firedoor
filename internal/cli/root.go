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
		Long:          "Firedoor links Terraform security intent, Kubernetes observations and bounded validation. The v0.1 evidence command checks previously captured local records; live scenario proofs are future work.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = Version
	root.AddCommand(newVersionCommand(), newDoctorCommand(), newEvidenceCommand())
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
