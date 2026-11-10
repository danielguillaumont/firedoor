// fd004bundleverify checks only private local recovery-preflight artifacts.
// It cannot execute any recovery or Terraform/Kubernetes change.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/danielguillaumont/firedoor/internal/recoverybundle"
)

func main() {
	bundle := flag.String("bundle", "", "absolute private FD-004 recovery bundle directory")
	state := flag.String("state", "", "current local Terraform state file")
	flag.Parse()
	if flag.NArg() != 0 || *bundle == "" || *state == "" {
		fmt.Fprintln(os.Stderr, "usage: fd004bundleverify -bundle ABS_DIR -state ABS_STATE")
		os.Exit(2)
	}
	r, err := recoverybundle.Verify(*bundle, *state, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 recovery bundle rejected:", err)
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
