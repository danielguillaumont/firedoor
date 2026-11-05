// planpreview exercises the v0.5 sanitizer using an explicit SYNTHETIC
// Terraform plan and output JSON. It never executes Terraform or reads state.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

// Both inputs deliberately contain sensitive-looking synthetic data to verify
// they are omitted from sanitized output. NEVER publish raw real plan JSON.
const syntheticPlan = `{
 "format_version":"1.2","terraform_version":"1.17.1",
 "complete":true,"applyable":false,
 "configuration":{"root_module":{"resources":[{"expressions":{"password":{"constant_value":"SYNTHETIC_SECRET_DO_NOT_PRINT"}}}]}},
 "planned_values":{"root_module":{"resources":[]}},
 "variables":{"secret":{"value":"SYNTHETIC_SECRET_DO_NOT_PRINT"}},
 "resource_changes":[
   {"address":"module.boundary.kubernetes_network_policy.example","mode":"managed","change":{"actions":["no-op"],"before":{"password":"SYNTHETIC_SECRET_DO_NOT_PRINT"},"after":{"password":"SYNTHETIC_SECRET_DO_NOT_PRINT"}}},
   {"address":"module.workload.kubernetes_deployment.example","mode":"managed","change":{"actions":["update"],"after":{"token":"SYNTHETIC_SECRET_DO_NOT_PRINT"}}}
 ],
 "output_changes":{"example":{"actions":["no-op"],"before":"SYNTHETIC_SECRET_DO_NOT_PRINT"}}
}`
const syntheticOutputs = `{"service":{"sensitive":false,"type":"string","value":"example"},"secret":{"sensitive":true,"type":"string","value":"SYNTHETIC_SECRET_DO_NOT_PRINT"}}`

func main() {
	root := flag.String("root", ".", "repository root for source-only inventory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	manifest, err := prooflens.CollectTerraformSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "source inventory:", err)
		os.Exit(1)
	}
	summary, err := prooflens.SummarizeTerraformPlanJSON([]byte(syntheticPlan), []byte(syntheticOutputs), manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sanitize synthetic plan:", err)
		os.Exit(1)
	}
	if err := prooflens.RecheckTerraformSources(*root, manifest); err != nil {
		fmt.Fprintln(os.Stderr, "source drift:", err)
		os.Exit(1)
	}
	ref, err := prooflens.TerraformPlanSourceSummary(summary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "summary reference:", err)
		os.Exit(1)
	}
	out := struct {
		Mode            string                         `json:"mode"`
		SyntheticOnly   bool                           `json:"synthetic_only"`
		SourceRechecked bool                           `json:"source_rechecked"`
		Summary         prooflens.TerraformPlanSummary `json:"summary"`
		ProofLensSource prooflens.SourceSummary        `json:"prooflens_source"`
	}{Mode: "SYNTHETIC_SANITIZER_PREVIEW", SyntheticOnly: true, SourceRechecked: true, Summary: summary, ProofLensSource: ref}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
