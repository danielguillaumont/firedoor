// fd004recoverpreflight checks current hardened FD-004 configuration only.
// It emits two minimal emergency recovery manifests, but never applies them.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/v10/cmd/fd004recoverpreflight")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c := liveverify.FD004LiveCollector{Terraform: liveverify.FD004LocalCommands{}, Kubernetes: kubeinspect.Collector{Runner: kubeinspect.CLI{}}}
	collection, err := c.Collect(ctx, liveverify.ProfileHardened)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-only hardened collector refused:", err)
		os.Exit(2)
	}
	report, err := liveverify.BuildFD004RecoveryPreflight(collection)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hardened recovery preflight refused:", err)
		os.Exit(2)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "preflight JSON encoding refused:", err)
		os.Exit(2)
	}
	fmt.Println(string(out))
}
