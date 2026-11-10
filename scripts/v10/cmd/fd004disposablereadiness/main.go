// Command fd004disposablereadiness reviews only previously collected JSON.
// It cannot query KIND, Docker, Kubernetes or Terraform and cannot authorize
// a change to any network policy.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func run(args []string) error {
	flags := flag.NewFlagSet("fd004disposablereadiness", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	input := flags.String("input", "", "existing offline readiness-inventory JSON file")
	confirm := flags.String("confirm-cluster", "", "exact disposable cluster name selected independently by operator")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *confirm == "" {
		return errors.New("-input and -confirm-cluster are both required; no positional arguments allowed")
	}
	path, err := filepath.Abs(*input)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > liveverify.FD004MaxDisposableReadinessJSON {
		return errors.New("input must be a bounded, regular JSON file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	report, err := liveverify.ReviewFD004DisposableReadinessJSON(raw, *confirm, time.Now().UTC())
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "FD-004 disposable readiness refused:", err)
		os.Exit(1)
	}
}
