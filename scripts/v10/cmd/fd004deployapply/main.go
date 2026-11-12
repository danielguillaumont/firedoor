package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverydeployapply"
)

func main() {
	req, err := recoverydeployapply.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid single-apply JSON:", err)
		os.Exit(2)
	}
	decision, err := recoverydeployapply.Evaluate(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "single apply rejected:", err)
		os.Exit(3)
	}
	if err := recoverydeployapply.ValidateDecision(decision); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	if err := json.NewEncoder(os.Stdout).Encode(decision); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
}
