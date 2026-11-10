// Command fd004recoveryevidence is an offline-only reviewer of three
// previously collected local FD-004 proof files.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func run(args []string) error {
	flags := flag.NewFlagSet("fd004recoveryevidence", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	initial := flags.String("initial", "", "saved initial hardened proof")
	baseline := flags.String("baseline", "", "saved baseline proof")
	restored := flags.String("restored", "", "saved restored hardened proof")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *initial == "" || *baseline == "" || *restored == "" {
		return errors.New("all three distinct -initial, -baseline and -restored JSON file paths are required")
	}
	seen := map[string]bool{}
	proofs := make([]liveverify.FD004LiveProof, 0, 3)
	for _, path := range []string{*initial, *baseline, *restored} {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absolute = filepath.Clean(absolute)
		if seen[absolute] {
			return errors.New("proof file paths must be distinct")
		}
		seen[absolute] = true
		proof, err := liveverify.ReadFD004ProofFile(absolute)
		if err != nil {
			return fmt.Errorf("proof read refused: %w", err)
		}
		proofs = append(proofs, proof)
	}
	result, err := liveverify.ReviewFD004RecoveryEvidence(proofs[0], proofs[1], proofs[2])
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 recovery evidence refused:", err)
		os.Exit(1)
	}
}
