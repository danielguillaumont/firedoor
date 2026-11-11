// Read-only FD-004 E1 state-candidate inspection. No Terraform or Kubernetes
// command is invoked. In particular, a prior planning-only plan is NEVER deployable.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoveryfreshplan"
	"github.com/danielguillaumont/firedoor/internal/recoveryownedstate"
)

func main() {
	run := flag.String("run", "", "pinned existing fresh-plan run")
	fresh := flag.String("fresh", "", "pinned source-only stage")
	session := flag.String("session-sha256", "", "pinned session SHA256")
	plan := flag.String("plan-sha256", "", "pinned planning-only binary SHA256")
	manifest := flag.String("manifest-sha256", "", "pinned source-stage manifest SHA256")
	flag.Parse()
	if *run == "" || *fresh == "" || *session == "" || *plan == "" || *manifest == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "complete original run paths and SHA-256 pins required")
		os.Exit(2)
	}
	if err := recoveryownedstate.CheckOverrides(os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 E1 BLOCKED:", err)
		os.Exit(1)
	}
	previous, err := recoveryfreshplan.ReconcileFreshRun(*run, *fresh, *session, *plan, *manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 E1 old planning evidence BLOCKED:", err)
		os.Exit(1)
	}
	reconciled := previous.SourceOnlyStagePreserved && previous.ResourcesReviewed == 14 &&
		previous.HardenedPoliciesReviewed == 2 && !previous.CurrentStateOwnershipVerified &&
		!previous.PlanBoundToAuthoritativeState && !previous.ApplyAuthorized &&
		!previous.ApplyExecuted && !previous.RecoveryProven
	report, err := recoveryownedstate.Eligible(os.Getenv("LOCALAPPDATA"), os.Environ(), reconciled)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 E1 candidate BLOCKED:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
