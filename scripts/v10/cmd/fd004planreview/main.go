// fd004planreview reviews Terraform show -json output from stdin and never
// executes Terraform or authorizes an apply.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// readPlanInput allows exactly one optional UTF-8 transport BOM inserted by
// a Windows native-command/standard-input boundary. It does not normalize or
// edit Terraform JSON. The strict security plan validator runs unchanged.
func readPlanInput(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, int64(liveverify.FD004MaxPlanJSONBytes+len(utf8BOM)+1)))
	if err != nil {
		return nil, fmt.Errorf("read plan input: %w", err)
	}
	if bytes.HasPrefix(raw, utf8BOM) {
		raw = raw[len(utf8BOM):]
	}
	if len(raw) == 0 || len(raw) > liveverify.FD004MaxPlanJSONBytes {
		return nil, errors.New("plan input missing or too large")
	}
	return raw, nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: terraform show -json PLAN | go run ./scripts/v10/cmd/fd004planreview")
		os.Exit(2)
	}
	raw, err := readPlanInput(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report, err := liveverify.ReviewFD004BaselinePlanJSON(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 Terraform plan rejected:", err)
		os.Exit(2)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(string(out))
}
