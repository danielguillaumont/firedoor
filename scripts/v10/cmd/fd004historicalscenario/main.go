// Command fd004historicalscenario reviews FD-004 against a hash-pinned,
// user-supplied sanitized G1 projection. It does not open network connections.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/scenarioevaluation"
)

func run() error {
	projection := flag.String("projection", "", "explicit G1 sanitized projection path")
	sha := flag.String("sha256", "", "expected projection SHA-256")
	contract := flag.String("contract", "", "explicit reviewed FD-004 contract path")
	flag.Parse()
	if *projection == "" || *sha == "" || *contract == "" || flag.NArg() != 0 {
		return fmt.Errorf("required: -projection <file> -sha256 <pinned G1 SHA256> -contract scenarios/FD-004/contract.json")
	}
	if !strings.EqualFold(*sha, scenarioevaluation.ProjectionSHA256) {
		return fmt.Errorf("G1 projection hash is not the reviewed historical snapshot")
	}
	c, err := os.ReadFile(*contract)
	if err != nil {
		return fmt.Errorf("cannot read FD-004 contract: %w", err)
	}
	csum := sha256.Sum256(c)
	if !strings.EqualFold(hex.EncodeToString(csum[:]), scenarioevaluation.ContractSHA256) {
		return fmt.Errorf("FD-004 contract checksum mismatch")
	}
	p, err := os.ReadFile(*projection)
	if err != nil {
		return fmt.Errorf("cannot read G1 projection: %w", err)
	}
	psum := sha256.Sum256(p)
	if !strings.EqualFold(hex.EncodeToString(psum[:]), *sha) {
		return fmt.Errorf("G1 projection checksum mismatch")
	}
	result, err := scenarioevaluation.Evaluate(c, p)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "H1 BLOCKED:", err)
		os.Exit(1)
	}
}
