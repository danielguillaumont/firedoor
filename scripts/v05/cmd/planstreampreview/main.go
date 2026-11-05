// planstreampreview exercises a synthetic Terraform -json UI event stream.
// It does not execute Terraform, inspect state, or access Kubernetes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const testStream = `{"type":"version","terraform":"1.17.1","ui":"1.2","@level":"info","@message":"SYNTHETIC_SECRET_DO_NOT_PRINT"}
{"type":"planned_change","change":{"resource":{"addr":"SYNTHETIC_SECRET_DO_NOT_PRINT"},"action":"update"}}
{"type":"outputs","outputs":{"password":{"value":"SYNTHETIC_SECRET_DO_NOT_PRINT","sensitive":true}}}
{"type":"change_summary","changes":{"add":0,"change":1,"remove":0,"operation":"plan"}}
`

func main() {
	root := flag.String("root", ".", "Firedoor repository root for read-only Terraform source inventory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional argument")
		os.Exit(2)
	}
	manifest, err := prooflens.CollectTerraformSources(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "source inventory failed")
		os.Exit(1)
	}
	summary, err := prooflens.SummarizeTerraformUIStream(strings.NewReader(testStream), manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "synthetic stream validation failed")
		os.Exit(1)
	}
	if err := prooflens.RecheckTerraformSources(*root, manifest); err != nil {
		fmt.Fprintln(os.Stderr, "source tree changed during synthetic preview")
		os.Exit(1)
	}
	output := struct {
		Mode             string                           `json:"mode"`
		SyntheticOnly    bool                             `json:"synthetic_only"`
		LivePlanExecuted bool                             `json:"live_plan_executed"`
		SourcesRechecked bool                             `json:"sources_rechecked"`
		Summary          prooflens.TerraformUIPlanSummary `json:"summary"`
	}{Mode: "SYNTHETIC_UI_STREAM_ONLY", SyntheticOnly: true, LivePlanExecuted: false, SourcesRechecked: true, Summary: summary}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, "summary encoding failed")
		os.Exit(1)
	}
}
