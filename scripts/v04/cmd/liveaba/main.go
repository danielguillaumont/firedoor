// Firedoor v0.4 guarded one-at-a-time A/B/A experiment entrypoint.
// Default mode is offline REVIEW ONLY. Live writes require THREE independent
// operator-controlled gates: --execute, an exact acknowledgement, and an
// opt-in environment value. The owning PowerShell review step never sets them.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

const consent = "I_APPROVE_TEMPORARY_NETWORKPOLICY_WRITES_IN_KIND_FIREDOOR_LOCAL"
const liveEnvName = "FIREDOOR_V04_ENABLE_LIVE"
const liveEnvValue = "OWNED_KIND_ONLY"

func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func checkBranch(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "feat/v04-policy-attribution" {
		return errors.New("live v0.4 mutation requires the expected feature branch")
	}
	return nil
}

// Pure validation for flags and environment. Unit tests cannot open a live
// Kubernetes writer; they validate the opt-in boundary without shell commands.
func approvedLiveInput(experiment, acknowledgement, environment string) error {
	if experiment != "witness-edge-to-data" && experiment != "witness-data-to-app" {
		return errors.New("must specify exactly one approved --experiment before enabling mutation")
	}
	if acknowledgement != consent || environment != liveEnvValue {
		return errors.New("live execution refused: explicit approval phrase and environment consent are required")
	}
	return nil
}

func run() error {
	experiment := flag.String("experiment", "", "one approved ID: witness-edge-to-data or witness-data-to-app")
	execute := flag.Bool("execute", false, "enable one real policy experiment (requires two additional explicit gates)")
	ack := flag.String("ack", "", "exact operator approval phrase, required only with --execute")
	flag.Parse()
	if len(flag.Args()) > 0 {
		return errors.New("positional arguments are not accepted")
	}
	if !*execute {
		if *ack != "" {
			return errors.New("approval phrase is only accepted when --execute is present")
		}
		preview, err := attribution.NewAdapterPreview()
		if err != nil {
			return err
		}
		preview.Limitations = []string{
			"This command is in REVIEW_ONLY mode; it cannot create or delete a Kubernetes policy.",
			"The live runner is only available after exact flags, environment consent, branch verification and a fresh pinned-lab preflight.",
			"An interrupted live run may retain a recovery lock and require manual UID-checked recovery.",
		}
		return emit(struct {
			Mode              string                     `json:"mode"`
			LiveWritesEnabled bool                       `json:"live_writes_enabled"`
			Review            attribution.AdapterPreview `json:"review"`
			NextStep          string                     `json:"next_step"`
		}{"REVIEW_ONLY", false, preview, "Live writes are disabled unless --execute, the full acknowledgement, and the opt-in environment variable are all present."})
	}
	if err := approvedLiveInput(*experiment, *ack, os.Getenv(liveEnvName)); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, 7*time.Minute)
	defer cancel()
	if err := checkBranch(ctx); err != nil {
		return err
	}
	// Live reads can occur before a lock or recovery journal is created, but
	// NO policy mutation may occur until the full Lifecycle preflight repeats.
	if _, err := attribution.RunPreflight(ctx, attribution.ReadOnlyCommands{}); err != nil {
		return fmt.Errorf("independent initial read-only admission failed: %w", err)
	}
	reader := attribution.ReadOnlyCommands{}
	driver := attribution.NewWitnessKubeDriver(reader, attribution.ExecRawWriter{})
	finalAudit := func(c context.Context) error {
		_, err := attribution.RunPreflight(c, attribution.ReadOnlyCommands{})
		return err
	}
	result, err := attribution.RunOneLiveWithRecovery(ctx, root, *experiment, driver, finalAudit)
	if printErr := emit(result); printErr != nil {
		return errors.Join(err, printErr)
	}
	if err != nil {
		return fmt.Errorf("live experiment incomplete; stop and inspect retained recovery evidence/lock: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Firedoor v0.4 live adapter:", err)
		os.Exit(1)
	}
}
