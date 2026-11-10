// Command livegate evaluates ONE typed observation emitted by Firedoor's
// disposable kind/Calico driver. It never touches the network or Kubernetes.
// Exit codes: 0 MATCH, 10 REGRESSION, 11 INCONCLUSIVE, 2 invalid input.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/regression"
)

func run() int {
	file := flag.String("input", "", "absolute path to a local, bounded live observation JSON")
	flag.Parse()
	if flag.NArg() != 0 || *file == "" {
		fmt.Fprintln(os.Stderr, "usage: livegate --input FILE")
		return 2
	}
	f, err := os.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, regression.MaxLiveObservationBytes+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	o, err := regression.DecodeLive(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	report, err := regression.EvaluateLive(o)
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
	case "REGRESSION":
		return 10
	default:
		return 11
	}
}
func main() { os.Exit(run()) }
