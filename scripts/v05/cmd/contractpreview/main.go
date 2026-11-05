// contractpreview runs entirely offline using a deliberately SYNTHETIC fixture.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func main() {
	b, err := prooflens.Seal(prooflens.SyntheticExample())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := prooflens.VerifyBundle(b)
	output := struct {
		Mode   string                     `json:"mode"`
		RunID  string                     `json:"run_id"`
		Report prooflens.ValidationReport `json:"report"`
	}{Mode: "SYNTHETIC_ONLY", RunID: b.Run.RunID, Report: r}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if r.Integrity != prooflens.IntegrityValid || r.ProofVerdict != prooflens.Inconclusive {
		os.Exit(1)
	}
}
