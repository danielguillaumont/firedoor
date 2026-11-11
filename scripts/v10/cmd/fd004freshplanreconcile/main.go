// Offline, read-only FD-004 fresh-plan reconciliation. Does not execute
// terraform, Kubernetes or any infrastructure operation, or write evidence.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/recoveryfreshplan"
	"os"
)

func main() {
	run := flag.String("run", "", "exact private fresh-plan run")
	stage := flag.String("fresh", "", "exact private source-only workspace")
	session := flag.String("session-sha256", "", "expected session receipt SHA-256")
	plan := flag.String("plan-sha256", "", "expected saved plan SHA-256")
	manifest := flag.String("manifest-sha256", "", "expected source manifest SHA-256")
	flag.Parse()
	if *run == "" || *stage == "" || *session == "" || *plan == "" || *manifest == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "exact private paths and 3 SHA-256 pins required")
		os.Exit(2)
	}
	out, err := recoveryfreshplan.ReconcileFreshRun(*run, *stage, *session, *plan, *manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 fresh plan reconciliation BLOCKED:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
