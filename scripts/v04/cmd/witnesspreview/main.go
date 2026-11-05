// Preview two compile-time fixed, narrowly scoped Kubernetes ingress witnesses.
// This command does not execute kubectl, contact the cluster, or modify state.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "witnesspreview accepts no arguments")
		os.Exit(2)
	}
	plan, err := attribution.NewWitnessPreview()
	if err != nil {
		fmt.Fprintln(os.Stderr, "witness preview rejected:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(plan); err != nil {
		fmt.Fprintln(os.Stderr, "cannot print witness preview:", err)
		os.Exit(1)
	}
}
