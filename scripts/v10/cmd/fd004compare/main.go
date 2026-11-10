package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func main() {
	baseline := flag.String("baseline", "", "path to previously captured FD-004 baseline live-proof JSON")
	hardened := flag.String("hardened", "", "path to previously captured FD-004 hardened live-proof JSON")
	flag.Parse()
	if *baseline == "" || *hardened == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: fd004compare -baseline <baseline-proof.json> -hardened <hardened-proof.json>")
		os.Exit(2)
	}
	b, e1 := filepath.Abs(*baseline)
	h, e2 := filepath.Abs(*hardened)
	if e1 != nil || e2 != nil || b == h {
		fmt.Fprintln(os.Stderr, "FD-004 refused: distinct valid proof input paths required")
		os.Exit(2)
	}
	bp, err := liveverify.ReadFD004ProofFile(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 baseline refused:", err)
		os.Exit(1)
	}
	hp, err := liveverify.ReadFD004ProofFile(h)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 hardened refused:", err)
		os.Exit(1)
	}
	report, err := liveverify.CompareFD004Transition(bp, hp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 comparison refused:", err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
