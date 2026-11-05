// Firedoor v0.4 Slice 3B2A: live A1 baseline, read-only Kubernetes API plus
// bounded exec probes. This executable has NO NetworkPolicy write operations.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signals, 5*time.Minute)
	defer cancel()
	report, err := attribution.RunReadOnlyA1OnKind(ctx)
	data, jsonErr := json.MarshalIndent(report, "", "  ")
	if jsonErr != nil {
		fmt.Fprintln(os.Stderr, jsonErr)
		os.Exit(1)
	}
	fmt.Println(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Firedoor v0.4 A1 baseline rejected:", err)
		os.Exit(1)
	}
}
