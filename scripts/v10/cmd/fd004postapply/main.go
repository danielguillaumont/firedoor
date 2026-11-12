package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverypostapply"
)

func main() {
	observation, err := recoverypostapply.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "F4 malformed observation:", err)
		os.Exit(1)
	}
	result, err := recoverypostapply.Evaluate(observation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
