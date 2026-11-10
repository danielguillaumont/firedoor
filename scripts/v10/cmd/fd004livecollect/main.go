// fd004livecollect is an explicitly invoked READ-ONLY inventory collector.
// Run from the Firedoor repository root; it never plans, applies or destroys.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func main() {
	profile := flag.String("profile", "", "baseline or hardened (required)")
	flag.Parse()
	if flag.NArg() != 0 || (*profile != "baseline" && *profile != "hardened") {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/v10/cmd/fd004livecollect --profile baseline|hardened")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	collector := liveverify.FD004LiveCollector{
		Terraform:  liveverify.FD004LocalCommands{},
		Kubernetes: kubeinspect.Collector{Runner: kubeinspect.CLI{}},
	}
	collection, err := collector.Collect(ctx, liveverify.Profile(*profile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-only collection refused:", err)
		os.Exit(2)
	}
	payload, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "collection serialization failed:", err)
		os.Exit(2)
	}
	fmt.Println(string(payload))
	// Exit zero means an inventory was serialized, never a healthy-boundary proof.
}
