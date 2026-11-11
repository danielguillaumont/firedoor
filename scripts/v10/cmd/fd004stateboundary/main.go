// fd004stateboundary reads one bounded JSON state-discovery snapshot on stdin.
// The output only reports an inspected local-backend boundary; never apply.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverystate"
)

func main() {
	in, err := io.ReadAll(io.LimitReader(os.Stdin, 32*1024+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read state discovery snapshot")
		os.Exit(1)
	}
	s, err := recoverystate.Decode(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid state snapshot:", err)
		os.Exit(1)
	}
	r, err := recoverystate.Evaluate(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, "state discovery rejected:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, "cannot encode report")
		os.Exit(1)
	}
}
