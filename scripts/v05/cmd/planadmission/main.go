// planadmission prints a reviewed Terraform command specification without
// executing Terraform, Kubernetes, or any other external process.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func main() {
	root := flag.String("root", ".", "Firedoor repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	manifest, err := prooflens.CollectTerraformSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Terraform source inventory unavailable")
		os.Exit(1)
	}
	admission, err := prooflens.PrepareTerraformPlanAdmission(*root, manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Terraform source admission failed")
		os.Exit(1)
	}
	if err := prooflens.VerifyTerraformPlanAdmission(admission); err != nil {
		fmt.Fprintln(os.Stderr, "command specification rejected")
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(admission); err != nil {
		os.Exit(1)
	}
}
