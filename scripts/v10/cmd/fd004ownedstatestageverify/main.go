package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/recoveryownedstate"
	"os"
	"strings"
)

func main() {
	fresh := flag.String("fresh", "", "immutable source stage")
	owned := flag.String("owned", "", "new source-only state candidate")
	hash := flag.String("manifest-sha256", "", "source-stage manifest fingerprint")
	flag.Parse()
	if *fresh == "" || *owned == "" || len(*hash) != 64 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "complete paths and SHA-256 pin required")
		os.Exit(2)
	}
	report, err := recoveryownedstate.VerifyStaged(os.Getenv("LOCALAPPDATA"), *fresh, *owned, strings.ToLower(*hash), os.Environ())
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 E2 BLOCKED:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
