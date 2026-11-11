package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielguillaumont/firedoor/internal/recoveryworkspace"
)

func main() {
	repo := flag.String("repo", "", "absolute checkout")
	privateRoot := flag.String("private-root", "", "private staging directory")
	stage := flag.String("stage", "", "new staged workspace")
	flag.Parse()
	if *repo == "" || *privateRoot == "" || *stage == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: fd004recoverystageverify -repo PATH -private-root PATH -stage PATH")
		os.Exit(2)
	}
	report, err := recoveryworkspace.Verify(*repo, *privateRoot, *stage)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 recovery staging rejected:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
