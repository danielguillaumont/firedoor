// labprobe is a temporary v0.3 development harness, not the stable CLI.
// It can only inspect and exec within the owned, single-node kind testbed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func main() {
	profile := flag.String("profile", "", "approved profile: baseline or hardened")
	acknowledge := flag.Bool("confirm-owned-lab", false, "acknowledge that a limited kubectl exec probe will run in firedoor-local")
	flag.Parse()
	if !*acknowledge || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "refusing: require --confirm-owned-lab and no extra args")
		os.Exit(2)
	}
	plan, err := liveverify.NewPlan(liveverify.Profile(*profile))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(plan.Limits.MaxSuiteDurationSeconds)*time.Second)
	defer cancel()
	exec := liveverify.Executor{Inspector: liveverify.Inspector{Runner: liveverify.OSCommands{}}}
	result, err := exec.Run(ctx, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAILED CLOSED:", err)
		os.Exit(1)
	}
	if err := result.ValidateObservation(plan); err != nil {
		fmt.Fprintln(os.Stderr, "INVALID OBSERVATIONS:", err)
		os.Exit(1)
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(output))
	if result.Status != "completed" {
		fmt.Fprintln(os.Stderr, "INCOMPLETE: at least one observation is inconclusive; no security verdict inferred")
		os.Exit(3)
	}
}
