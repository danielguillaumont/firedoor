// fd004liveproof runs the explicitly approved non-mutating six-route lab probe.
// It never plans/applies Terraform or creates/changes Kubernetes resources.
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
	profile := flag.String("profile", "", "existing Terraform baseline|hardened profile")
	flag.Parse()
	if flag.NArg() != 0 || (*profile != "baseline" && *profile != "hardened") {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/v10/cmd/fd004liveproof --profile baseline|hardened")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	proof, err := (liveverify.FD004LiveProver{
		Configuration: liveverify.FD004LiveCollector{
			Terraform:  liveverify.FD004LocalCommands{},
			Kubernetes: kubeinspect.Collector{Runner: kubeinspect.CLI{}},
		},
		Traffic: liveverify.Executor{Inspector: liveverify.Inspector{Runner: liveverify.OSCommands{}}},
	}).Run(ctx, liveverify.Profile(*profile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 live proof refused:", err)
		os.Exit(2)
	}
	payload, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 live proof JSON failed:", err)
		os.Exit(2)
	}
	fmt.Println(string(payload))
	// The wrapper treats nonmatching traffic as failure after saving evidence.
	// This command emits a full, honest nonmatching result for diagnosis.
}
