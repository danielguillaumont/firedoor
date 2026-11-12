package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverytrafficproof"
)

func main() {
	o, err := recoverytrafficproof.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "F5 invalid observation:", err)
		os.Exit(1)
	}
	r, err := recoverytrafficproof.Evaluate(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
