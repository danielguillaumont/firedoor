// fd004livepreflight only evaluates an input snapshot. It never executes infrastructure commands.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverylivegate"
)

func main() {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 256*1024+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read bounded snapshot")
		os.Exit(1)
	}
	snapshot, err := recoverylivegate.Decode(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot rejected:", err)
		os.Exit(1)
	}
	report, err := recoverylivegate.Evaluate(snapshot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-only preflight rejected:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "cannot encode report:", err)
		os.Exit(1)
	}
}
