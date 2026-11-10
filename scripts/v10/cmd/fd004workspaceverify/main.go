package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/rehearsalworkspace"
)

func main() {
	repo := flag.String("repo", "", "absolute source repository")
	stage := flag.String("workspace", "", "absolute private workspace")
	flag.Parse()
	if *repo == "" || *stage == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: fd004workspaceverify --repo PATH --workspace PATH")
		os.Exit(2)
	}
	report, err := rehearsalworkspace.Verify(*repo, *stage)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 disposable workspace refused:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
