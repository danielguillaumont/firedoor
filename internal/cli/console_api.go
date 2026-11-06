package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/danielguillaumont/firedoor/internal/api"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"github.com/spf13/cobra"
)

// newConsoleAPICommand introduces only a fixed loopback HTTP server. There are
// no bind-address, file-path, Terraform, kube-context, or mutation flags.
func newConsoleAPICommand() *cobra.Command {
	group := &cobra.Command{
		Use: "console-api", Short: "Local read-only HTTP API for the future Firedoor console",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var enableApprovedProof bool
	var consent string
	serve := &cobra.Command{
		Use: "serve", Short: "Serve offline contracts and optional approved detached proof on http://127.0.0.1:8787 (Ctrl+C to stop)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return serveConsoleAPI(cmd, enableApprovedProof, consent) },
	}
	serve.Flags().BoolVar(&enableApprovedProof, "enable-approved-proof", false, "Opt in to one previously saved, verified, Git-ignored detached JSON summary (no collection)")
	serve.Flags().StringVar(&consent, "consent", "", "Exact operator approval for importing the fixed local detached report")
	// Inspection is an explicit operator action. It validates the same fixed,
	// ignored detached JSON as serve, without binding any socket or exporting it.
	var inspectionConsent string
	inspect := &cobra.Command{
		Use: "inspect-approved-proof", Short: "Validate the fixed approved detached JSON without starting an HTTP server",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return inspectConsoleApprovedProof(cmd, inspectionConsent) },
	}
	inspect.Flags().StringVar(&inspectionConsent, "consent", "", "Exact operator approval for checking the fixed local detached report")
	group.AddCommand(serve, inspect)
	return group
}

// inspectConsoleApprovedProof intentionally returns a classification only;
// it never prints paths, evidence content, report hashes or local errors.
// This is consistency validation, NOT an authentication or origin check.
func inspectConsoleApprovedProof(cmd *cobra.Command, consent string) error {
	if consent != prooflens.ConsoleApprovalConsent {
		return errors.New("explicit local inspection consent required; no report was loaded")
	}
	summary, err := prooflens.LoadApprovedConsoleProof(".", consent)
	if err != nil {
		return errors.New("approved detached Proof Lens report unavailable or rejected; no HTTP server started")
	}
	if summary.Verdict != prooflens.Inconclusive || summary.ProofChainEstablished || summary.IndependentlyAttested || !summary.LocalConsistencyOnly {
		return errors.New("unexpected detached proof classification; no HTTP server started")
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "FIREDOOR v0.6 / DETACHED PROOF PREVIEW CHECK\nPASS: fixed, ignored JSON accepted by local consistency checks.\nVerdict: INCONCLUSIVE / historical context only.\nNo HTTP server started. No original evidence authenticated.")
	return err
}

func serveConsoleAPI(cmd *cobra.Command, enableApprovedProof bool, consent string) error {
	var handler http.Handler = api.NewHandler()
	if !enableApprovedProof && consent != "" {
		return errors.New("consent requires explicit --enable-approved-proof")
	}
	if enableApprovedProof {
		// No arbitrary path flag. Import only the fixed ignored source JSON;
		// fail closed BEFORE starting the HTTP listener.
		snapshot, err := prooflens.LoadApprovedConsoleProof(".", consent)
		if err != nil {
			return errors.New("approved detached Proof Lens report rejected; API not started")
		}
		handler = api.NewHandlerWithApprovedProof(&snapshot)
	}
	// tcp4 + an explicit loopback IP prevent wildcard/listen-all binding.
	listener, err := net.Listen("tcp4", api.ListenAddress)
	if err != nil {
		return fmt.Errorf("console API localhost listener unavailable: %w", err)
	}
	defer listener.Close()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       15 * time.Second,
		MaxHeaderBytes:    8 * 1024,
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	banner := fmt.Sprintf("FIREDOOR v0.6 / LOCAL READ-ONLY CONSOLE API\nGET http://%s/api/v1/health\nGET http://%s/api/v1/capabilities\nGET http://%s/api/v1/scenarios\n", api.ListenAddress, api.ListenAddress, api.ListenAddress)
	if enableApprovedProof {
		banner += fmt.Sprintf("GET http://%s/api/v1/proof-summary (approved detached historical summary only)\n", api.ListenAddress)
	}
	banner += "No Terraform, Kubernetes, SQLite, or proof collection. Ctrl+C to stop.\n"
	if _, err := fmt.Fprint(cmd.OutOrStdout(), banner); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		<-done
		return err
	}
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-done
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("console API server failed: %w", err)
		}
		return nil
	case err := <-done:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("console API server failed: %w", err)
		}
		return nil
	}
}
