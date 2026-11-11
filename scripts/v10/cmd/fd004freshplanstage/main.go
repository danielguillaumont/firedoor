// fd004freshplanstage verifies a new private source-only planning workspace.
// It never executes Terraform, authenticates Kubernetes, or authorizes apply.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/recoveryfreshplan"
	"os"
)

func main() {
	repo := flag.String("repo", "", "absolute repository path")
	stage := flag.String("stage", "", "absolute retained stage path")
	fresh := flag.String("fresh", "", "absolute fresh workspace path")
	flag.Parse()
	report, err := recoveryfreshplan.Verify(*repo, *stage, *fresh, recoveryfreshplan.DefaultPins)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fresh source stage rejected:", err)
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write report")
		os.Exit(1)
	}
}
