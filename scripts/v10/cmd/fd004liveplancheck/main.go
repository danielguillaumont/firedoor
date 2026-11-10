// fd004liveplancheck reads a private Terraform plan JSON file and applies the
// fail-closed initial-deployment shape contract. It never executes Terraform.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/deployreview"
)

type Receipt struct {
	SchemaVersion  string              `json:"schema_version"`
	PlanSHA256     string              `json:"plan_sha256"`
	Review         deployreview.Review `json:"review"`
	ApplyExecuted  bool                `json:"apply_executed"`
	RecoveryTested bool                `json:"recovery_tested"`
}

func main() {
	plan := flag.String("plan-json", "", "private JSON output of terraform show -json")
	cluster := flag.String("cluster", "", "verified reserved KIND cluster")
	kubeconfig := flag.String("kubeconfig", "", "verified disposable kubeconfig")
	flag.Parse()
	if flag.NArg() != 0 || *plan == "" || *cluster == "" || *kubeconfig == "" {
		fmt.Fprintln(os.Stderr, "plan-json, cluster and kubeconfig arguments are required")
		os.Exit(2)
	}
	if !filepath.IsAbs(*plan) || !filepath.IsAbs(*kubeconfig) || strings.ContainsAny(*plan, "\r\n\x00") {
		fmt.Fprintln(os.Stderr, "absolute safe paths required")
		os.Exit(2)
	}
	info, err := os.Lstat(*plan)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > deployreview.Limit+3 {
		fmt.Fprintln(os.Stderr, "plan JSON is missing, nonregular or outside size limit")
		os.Exit(1)
	}
	raw, err := os.ReadFile(*plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sum := sha256.Sum256(raw) // exact saved JSON file bytes, including optional PowerShell BOM
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	reviewed, err := deployreview.ReviewJSON(raw, *cluster, *kubeconfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 initial plan REJECTED:", err)
		os.Exit(1)
	}
	receipt := Receipt{
		SchemaVersion:  "firedoor.v10.fd004-disposable-live-plan-review.v1",
		PlanSHA256:     hex.EncodeToString(sum[:]),
		Review:         reviewed,
		ApplyExecuted:  false,
		RecoveryTested: false,
	}
	if err := json.NewEncoder(os.Stdout).Encode(receipt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
