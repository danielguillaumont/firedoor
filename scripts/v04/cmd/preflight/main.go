// Firedoor v0.4 read-only policy preflight. No Kubernetes writes or probes.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "preflight accepts no arguments; only the compiled owned lab is supported")
		os.Exit(2)
	}
	report, err := attribution.RunPreflight(context.Background(), attribution.ReadOnlyCommands{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FIREDOOR preflight refused admission:", err)
		fmt.Fprintln(os.Stderr, "No Kubernetes mutations or network probes were attempted.")
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "cannot encode preflight report:", err)
		os.Exit(1)
	}
}
