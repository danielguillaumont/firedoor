// Command fd004deploymentpreflight evaluates a read-only observed cluster snapshot.
// Output is not a deployment permission or a Terraform apply token.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoverydeploypreflight"
)

func main() {
	o, err := recoverydeploypreflight.Decode(os.Stdin)
	if err == nil {
		var r recoverydeploypreflight.Report
		r, err = recoverydeploypreflight.Evaluate(o)
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(r)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deployment preflight rejected:", err)
		os.Exit(1)
	}
}
