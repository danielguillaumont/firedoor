// historicalwitness reads only paired local Firedoor v0.4 A/B/A reports and
// journals. It never uses kubectl, Terraform, SQLite, or mutation APIs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func main() {
	if err := run(); err != nil {
		// Do not print file contents, old report diagnostic strings or paths.
		fmt.Fprintln(os.Stderr, "Firedoor historical witness inspection refused; check approved report pair and recovery state")
		os.Exit(1)
	}
}

func run() error {
	rootFlag := flag.String("root", ".", "Firedoor repository root")
	scan := flag.Bool("scan", false, "List approved historical report basenames only")
	inspectFile := flag.String("inspect-file", "", "Exact v0.4 report basename inside docs/evidence/local")
	flag.Parse()
	if flag.NArg() != 0 || (*scan && *inspectFile != "") {
		return errors.New("invalid mode")
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if !*scan && *inspectFile == "" {
		return enc.Encode(struct {
			Mode               string `json:"mode"`
			LocalFilesOpened   bool   `json:"local_files_opened"`
			TerraformExecuted  bool   `json:"terraform_executed"`
			KubernetesAccessed bool   `json:"kubernetes_accessed"`
			SQLiteOpened       bool   `json:"sqlite_opened"`
			ProofVerdict       string `json:"proof_verdict"`
		}{"REVIEW_ONLY_NO_HISTORICAL_ACCESS", false, false, false, false, prooflens.Inconclusive})
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	cmd.Stderr = nil
	branch, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(branch)) != "feat/v05-evidence" {
		return errors.New("unapproved feature branch")
	}
	dir := filepath.Join(root, "docs", "evidence", "local")
	if *scan {
		names, err := prooflens.DiscoverHistoricalWitnessReports(dir)
		if err != nil {
			return err
		}
		return enc.Encode(struct {
			Mode         string   `json:"mode"`
			ReportCount  int      `json:"report_count"`
			ReportFiles  []string `json:"report_files"`
			ContentsRead bool     `json:"contents_read"`
			ProofVerdict string   `json:"proof_verdict"`
		}{"HISTORICAL_V04_BASENAME_SCAN_ONLY", len(names), names, false, prooflens.Inconclusive})
	}
	record, err := prooflens.InspectHistoricalWitnessPair(dir, *inspectFile)
	if err != nil {
		return err
	}
	return enc.Encode(record)
}
