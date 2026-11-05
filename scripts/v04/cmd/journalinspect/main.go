// Read-only viewer for a local Firedoor v0.4 recovery journal.
// This program never invokes kubectl or runs a live experiment.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/v04/cmd/journalinspect <local-journal.jsonl>")
		os.Exit(2)
	}
	summary, err := attribution.InspectRecoveryJournal(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "recovery journal rejected; manual inspection required:", err)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if summary.PendingManualReview {
		fmt.Fprintln(os.Stderr, "unresolved journal events: stop; inspect exact-name witnesses on the pinned kind cluster")
		os.Exit(2)
	}
}
