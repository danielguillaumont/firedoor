// fd004deploygate reads immutable historical FD-004 evidence; it never issues
// an execution permit or invokes Terraform, Kubernetes, Docker or KIND.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverygate"
)

func main() {
	stage := flag.String("stage", "", "exact private stage directory (path only)")
	intent := flag.String("intent", "", "existing stage intent JSON")
	bootstrap := flag.String("bootstrap", "", "pinned historical bootstrap reconciliation JSON")
	binary := flag.String("binary-plan", "", "existing historical saved binary plan; never apply")
	planJSON := flag.String("plan-json", "", "existing retained decoded plan JSON")
	admission := flag.String("admission", "", "existing independent admission report JSON")
	receipt := flag.String("plan-reconciliation", "", "existing plan reconciliation JSON")
	flag.Parse()
	if *stage == "" || *intent == "" || *bootstrap == "" || *binary == "" ||
		*planJSON == "" || *admission == "" || *receipt == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "all seven private paths are required; this CLI has no deployment action")
		os.Exit(2)
	}
	read := func(label, filename string, max int64) []byte {
		info, err := os.Lstat(filename)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > max {
			fmt.Fprintln(os.Stderr, label, "must be a bounded regular file (no links)")
			os.Exit(1)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, label, "could not be read")
			os.Exit(1)
		}
		return data
	}
	report, err := recoverygate.Audit(recoverygate.Inputs{
		StagePath: *stage, StageIntent: read("stage intent", *intent, 128*1024),
		BootstrapReconciliation: read("bootstrap reconciliation", *bootstrap, 128*1024),
		BinaryPlan:              read("historical binary plan", *binary, 64*1024),
		PlanJSON:                read("decoded plan", *planJSON, 8*1024*1024),
		AdmissionReport:         read("admission", *admission, 128*1024),
		PlanReconciliation:      read("plan reconciliation", *receipt, 128*1024),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "READ-ONLY GATE REJECTED:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "cannot output report:", err)
		os.Exit(1)
	}
}
