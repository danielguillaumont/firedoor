// Temporary development harness: print the approved v0.4 experiment contract.
// It performs no Kubernetes, Terraform, or network operations.
package main

import (
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	b, err := attribution.JSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}
