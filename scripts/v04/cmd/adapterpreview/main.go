// Firedoor v0.4 Slice 3B2B1: source-only Kubernetes mutation adapter preview.
// Does not instantiate a writer or connect to Kubernetes.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

func main() {
	preview, err := attribution.NewAdapterPreview()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
