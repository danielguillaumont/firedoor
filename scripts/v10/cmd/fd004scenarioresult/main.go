// Command fd004scenarioresult converts a hash-checked, user-supplied H1
// historical review into an H2 result contract. It performs no network or
// infrastructure activity and cannot certify FD-004.
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
	path := flag.String("review", "", "explicit H1 review JSON file")
	hash := flag.String("sha256", "", "expected SHA-256 for this H1 review")
	flag.Parse()
	if *path == "" || len(*hash) != 64 || flag.NArg() != 0 {
		return fmt.Errorf("required: -review <H1 JSON> -sha256 <64-hex SHA256>")
	}
	if _, err := hex.DecodeString(*hash); err != nil {
		return fmt.Errorf("invalid SHA-256 fingerprint")
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return fmt.Errorf("cannot read H1 review: %w", err)
	}
	sum := sha256.Sum256(b)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), *hash) {
		return fmt.Errorf("H1 review SHA-256 mismatch")
	}
	result, err := scenarioevaluation.FromHistoricalReview(b)
	if err != nil {
		return err
	}
	out := json.NewEncoder(os.Stdout)
	out.SetIndent("", "  ")
	return out.Encode(result)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "H2 BLOCKED:", err)
		os.Exit(1)
	}
}
