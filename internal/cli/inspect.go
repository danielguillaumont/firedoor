package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Read Firedoor-owned local Kubernetes resources without changing them",
		Long:  "v0.2 inventory: reads only three owned namespaces and selected resource types. Outputs observed configuration, NOT passing security findings or proof of policy enforcement. Requires current context kind-firedoor-local.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "human" && format != "json" {
				return fmt.Errorf("invalid format %q: choose human or json", format)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			snapshot, err := (kubeinspect.Collector{Runner: kubeinspect.CLI{}}).Collect(ctx)
			if err != nil {
				return fmt.Errorf("owned-cluster inspection refused: %w", err)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(snapshot)
			}
			var b strings.Builder
			fmt.Fprintln(&b, "FIREDOOR v0.2 / Kubernetes configuration inventory / READ ONLY")
			fmt.Fprintf(&b, "Cluster: %s | Context: %s | Timestamp (UTC): %s\n", snapshot.Cluster, snapshot.Context, snapshot.CapturedAtUTC.Format(time.RFC3339))
			for _, z := range snapshot.Zones {
				fmt.Fprintf(&b, "\n[%s] deployment=%s ready=%d/%d service-account=%s pods=%d policies=%d roles=%d rolebindings=%d\n",
					z.Namespace, z.Deployment.Name, z.Deployment.ReadyReplicas, z.Deployment.DesiredReplicas,
					z.Deployment.ServiceAccountName, len(z.Pods), len(z.NetworkPolicies), len(z.Roles), len(z.RoleBindings))
				fmt.Fprintf(&b, "  Pod token automount: %s | ServiceAccount token automount: %s\n", boolState(z.Deployment.AutomountToken), boolState(z.ServiceAccount.AutomountToken))
				for _, ctr := range z.Deployment.Containers {
					fmt.Fprintf(&b, "  Container %s: runAsNonRoot=%s allowPrivilegeEscalation=%s\n", ctr.Name, boolState(ctr.Security.RunAsNonRoot), boolState(ctr.Security.AllowPrivilegeEscalation))
				}
				for _, p := range z.NetworkPolicies {
					fmt.Fprintf(&b, "  NetworkPolicy %s: types=%v ingress-rules=%d egress-rules=%d\n", p.Name, p.PolicyTypes, len(p.Ingress), len(p.Egress))
				}
			}
			fmt.Fprintln(&b, "Limit: "+snapshot.Limitations)
			_, err = fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
	cmd.Flags().StringVar(&format, "format", "human", "Output format: human or json")
	return cmd
}

func boolState(v *bool) string {
	if v == nil {
		return "unset/inherited/unknown"
	}
	if *v {
		return "true"
	}
	return "false"
}
