// fd004destroyreview performs OFFLINE cleanup-plan shape validation only.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/destroyreview"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	plan := flag.String("plan-json", "", "absolute private terraform show -json destroy plan path")
	cluster := flag.String("cluster", "", "disposable reserved cluster identity")
	kubeconfig := flag.String("kubeconfig", "", "disposable private kubeconfig")
	flag.Parse()
	if flag.NArg() != 0 || *plan == "" || *cluster == "" || *kubeconfig == "" || !filepath.IsAbs(*plan) || !filepath.IsAbs(*kubeconfig) || strings.ContainsAny(*plan, "\r\n\x00") {
		fmt.Fprintln(os.Stderr, "absolute private paths and cluster identity required")
		os.Exit(2)
	}
	st, err := os.Lstat(*plan)
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > destroyreview.Limit+3 {
		fmt.Fprintln(os.Stderr, "missing, unsafe or oversized plan JSON")
		os.Exit(1)
	}
	raw, err := os.ReadFile(*plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	digest := sha256.Sum256(raw)
	reviewed, err := destroyreview.ReviewJSON(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), *cluster, *kubeconfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 isolated cleanup plan REJECTED:", err)
		os.Exit(1)
	}
	receipt := struct {
		Schema            string               `json:"schema_version"`
		PlanJSONSHA256    string               `json:"plan_json_sha256"`
		Review            destroyreview.Review `json:"review"`
		TerraformExecuted bool                 `json:"terraform_executed"`
	}{"firedoor.v10.fd004-isolated-destroy-review.v1", hex.EncodeToString(digest[:]), reviewed, false}
	if err := json.NewEncoder(os.Stdout).Encode(receipt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
