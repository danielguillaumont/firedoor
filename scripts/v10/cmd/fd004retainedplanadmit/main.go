// fd004retainedplanadmit performs only offline review of three already-captured
// files. It never invokes Terraform, Kubernetes, Docker, kind or writes output.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoveryadmission"
)

func main() {
	stage := flag.String("stage", "", "exact private retained stage directory")
	intent := flag.String("intent", "", "existing private stage intent.json")
	reconciliation := flag.String("reconciliation", "", "existing private bootstrap-reconciliation.v1.json")
	plan := flag.String("plan-json", "", "previously captured Terraform show -json file; never a .tfplan")
	flag.Parse()
	if *stage == "" || *intent == "" || *reconciliation == "" || *plan == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "all four file/stage flags are required; no execution implied")
		os.Exit(2)
	}
	read := func(label, filename string, limit int64) []byte {
		info, err := os.Lstat(filename)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
			fmt.Fprintln(os.Stderr, label, "not a bounded regular file")
			os.Exit(1)
		}
		b, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, label, "unreadable")
			os.Exit(1)
		}
		return b
	}
	intentRaw := read("intent", *intent, 128*1024)
	reconciliationRaw := read("reconciliation", *reconciliation, 128*1024)
	planRaw := read("plan JSON", *plan, 8*1024*1024)
	result, err := recoveryadmission.Review(planRaw, intentRaw, reconciliationRaw,
		*stage, recoveryadmission.TargetCluster, recoveryadmission.ReconciliationSHA256)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 retained plan REJECTED:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
