// fd004recoverreview reviews a saved baseline-to-hardened Terraform plan JSON
// from stdin. It does not execute Terraform or authorize any apply.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: terraform show -json PLAN | go run ./scripts/v10/cmd/fd004recoverreview")
		os.Exit(2)
	}
	// Accept one optional Windows UTF-8 BOM at the CLI transport boundary;
	// the recovery validator itself rejects malformed and duplicate-key JSON.
	limit := int64(liveverify.FD004MaxPlanJSONBytes + 4)
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, limit))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		raw = raw[3:]
	}
	report, err := liveverify.ReviewFD004HardenedRecoveryPlanJSON(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 hardened recovery plan rejected:", err)
		os.Exit(2)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(string(out))
}
