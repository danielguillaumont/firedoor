// Command fixturegate verifies the v0.9A synthetic boundary-regression contract.
// It never invokes kubectl, Terraform, Docker, or network APIs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/regression"
)

func main() {
	fixture := flag.String("fixture", "", "synthetic fixture: intact, weakened, or missing")
	flag.Parse()
	if flag.NArg() != 0 || *fixture == "" {
		fmt.Fprintln(os.Stderr, "usage: fixturegate --fixture intact|weakened|missing")
		os.Exit(2)
	}
	b, err := regression.Fixture(*fixture)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	o, err := regression.Decode(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report, err := regression.Evaluate(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if report.Verdict != "MATCH" {
		os.Exit(1)
	}
}
