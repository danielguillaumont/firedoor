package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect local binary availability without touching a Kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := doctor.Collect()
			switch strings.ToLower(format) {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			case "human":
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Firedoor development host: %s/%s\n", runtime.GOOS, runtime.GOARCH); err != nil {
					return err
				}
				for _, item := range result.Tools {
					state := "MISSING"
					if item.Found {
						state = "FOUND"
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-11s %-7s %s\n", item.Name, state, item.Path); err != nil {
						return err
					}
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Binary detection only; no cluster/CNI/security controls were verified.")
				return err
			default:
				return fmt.Errorf("invalid format %q: choose human or json", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "human", "Output format: human or json")
	return cmd
}
