package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
	"github.com/spf13/cobra"
)

// The initial 7E interface is deliberately an offline, operator-triggered
// preflight, NOT a new HTTP handler, report exporter, or evidence collector.
// The v0.6 approved Proof Lens report is not a v0.3 six-route suite.
func newContainmentMapCommand() *cobra.Command {
	group := &cobra.Command{
		Use:   "containment-map",
		Short: "Review fixed-lab network topology and historical observation boundaries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	var input, profile string
	var acknowledge bool
	inspect := &cobra.Command{
		Use:   "inspect-historical",
		Short: "Offline, sanitized preflight of a saved v0.3 six-route observation (not proof of enforcement)",
		Long:  "Explicitly inspect a locally held, already captured v0.3 six-route suite. Internal consistency is checked, but file origin and operator-supplied profile are not independently authenticated. Does not start HTTP, run probes, query Kubernetes, read Terraform state, or produce a public evidence file.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profile != "baseline" && profile != "hardened" {
				return errors.New("select only baseline or hardened for --profile")
			}
			if !acknowledge {
				return errors.New("explicit --acknowledge-historical required; the source and profile are not authenticated")
			}
			m, err := containmentmap.ProjectV03HistoricalFile(input, liveverify.Profile(profile))
			if err != nil {
				return errors.New("saved v0.3 historical observation rejected; no summary or HTTP server created")
			}
			return writeHistoricalContainmentSummary(cmd.OutOrStdout(), m)
		},
	}
	inspect.Flags().StringVar(&input, "input", "", "Operator-selected local saved v0.3 suite JSON (never uploaded or echoed)")
	inspect.Flags().StringVar(&profile, "profile", "", "Operator-confirmed profile: baseline or hardened")
	inspect.Flags().BoolVar(&acknowledge, "acknowledge-historical", false, "Explicitly acknowledge that origin, profile and causality are not independently authenticated")
	_ = inspect.MarkFlagRequired("input")
	_ = inspect.MarkFlagRequired("profile")
	group.AddCommand(inspect)
	return group
}

// writeHistoricalContainmentSummary prints only bounded, canonical route IDs
// and classification fields. Never print raw suite contents, names, IPs,
// source identifiers, timestamps, or private paths from a supplied report.
func writeHistoricalContainmentSummary(w io.Writer, m containmentmap.Map) error {
	if err := m.Validate(); err != nil || m.Source != "historical_projection" || m.CurrentInventory || m.EnforcementVerified {
		return errors.New("historical summary is outside the accepted safety boundary")
	}
	plan, err := liveverify.NewPlan(liveverify.Profile(m.Profile))
	if err != nil || len(plan.Probes) != len(m.Routes) {
		return errors.New("historical summary does not match the fixed route plan")
	}
	var observed int
	for i, route := range m.Routes {
		if route.ID != plan.Probes[i].ID {
			return errors.New("historical summary route order differs from the canonical plan")
		}
		if route.Observed != "not_observed" {
			observed++
		}
	}

	if _, err := fmt.Fprintf(w, "FIREDOOR v0.7 / OFFLINE SAVED NETWORK OBSERVATION / HISTORICAL ONLY\nProfile: %s (operator supplied, not authenticated)\nFixed Firedoor lab: %d zones, %d directed routes\nSaved observed routes: %d | Not observed: %d\n", m.Profile, len(m.Nodes), len(m.Routes), observed, len(m.Routes)-observed); err != nil {
		return err
	}
	for i, route := range m.Routes {
		if _, err := fmt.Fprintf(w, "  %-14s planned=%-11s saved_observed=%-12s assessment=%s\n", route.ID, plan.Probes[i].Expected, route.Observed, route.Assessment); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w, "DECLARED TERRAFORM INTENT: UNSPECIFIED | CURRENT INVENTORY: NO | ENFORCEMENT VERIFIED: NO\nInternal consistency only; no source authentication, policy-causation finding, live cluster read, HTTP server, or evidence export.")
	return err
}
