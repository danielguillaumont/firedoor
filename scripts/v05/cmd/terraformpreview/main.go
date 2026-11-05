// terraformpreview prints a safe, read-only fingerprint of the fixed Firedoor
// Terraform source directories. It does NOT call Terraform, kubectl, or SQLite.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func main() {
	root := flag.String("root", ".", "Firedoor repository root containing infrastructure/terraform")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	manifest, err := prooflens.CollectTerraformSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Terraform source inventory:", err)
		os.Exit(1)
	}
	if err := prooflens.RecheckTerraformSources(*root, manifest); err != nil {
		fmt.Fprintln(os.Stderr, "Terraform recheck:", err)
		os.Exit(1)
	}
	source, err := prooflens.TerraformSourceSummary(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Terraform source summary:", err)
		os.Exit(1)
	}
	out := struct {
		Manifest  prooflens.TerraformSourceManifest `json:"manifest"`
		Source    prooflens.SourceSummary           `json:"prooflens_source"`
		Rechecked bool                              `json:"source_rechecked"`
	}{Manifest: manifest, Source: source, Rechecked: true}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
