// driftgate evaluates one OFFLINE Kubernetes policy snapshot.
// Exit codes: 0 MATCH, 10 DRIFT, 11 INCONCLUSIVE, 2 invalid input.
// This command DOES NOT connect to Kubernetes or prove snapshot provenance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/regression"
	"io"
	"os"
)

func run() int {
	fs := flag.NewFlagSet("driftgate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "path to bounded, local drift snapshot JSON")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *input == "" {
		fmt.Fprintln(os.Stderr, "usage: driftgate --input FILE")
		return 2
	}
	f, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, regression.DriftMaxSnapshotBytes+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	obs, err := regression.DecodeDrift(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	report, err := regression.EvaluateDrift(obs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	switch report.Verdict {
	case "MATCH":
		return 0
	case "DRIFT":
		return 10
	default:
		return 11
	}
}
func main() { os.Exit(run()) }
