// Synthetic recovery-journal preview. Filesystem only. No Kubernetes imports,
// no Kubernetes commands, no live policy writes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	dir, err := os.MkdirTemp("", "firedoor-v04-journal-preview-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "synthetic.jsonl")
	j, err := attribution.NewRecoveryJournal(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, ex := range attribution.ApprovedPlan().Experiments {
		for _, kind := range []string{"create_intent", "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed"} {
			uid := "3f8b3f15-3320-4dd2-a194-420114ead000"
			if kind == "create_intent" {
				uid = ""
			}
			if err := j.AppendEvent(ex, kind, uid); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}
	sum, err := attribution.InspectRecoveryJournal(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := json.MarshalIndent(struct {
		Mode              string                      `json:"mode"`
		LiveWritesEnabled bool                        `json:"live_writes_enabled"`
		Summary           attribution.RecoverySummary `json:"summary"`
	}{Mode: "SYNTHETIC_LOCAL_ONLY", LiveWritesEnabled: false, Summary: sum}, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}
