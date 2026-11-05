// labverify is a temporary, offline v0.3 development harness.
// It only reads an existing local JSON observation file. It does not execute
// kubectl, access the cluster, or change Terraform resources or state.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

const maxEvidenceBytes = 2 << 20

func main() {
	input := flag.String("input", "", "path to locally captured v0.3 live observation JSON")
	profile := flag.String("profile", "", "approved profile: baseline or hardened")
	format := flag.String("format", "human", "human or json")
	flag.Parse()
	if *input == "" || flag.NArg() != 0 || (*format != "human" && *format != "json") {
		fmt.Fprintln(os.Stderr, "require --input PATH --profile baseline|hardened --format human|json; no extra arguments")
		os.Exit(2)
	}
	plan, err := liveverify.NewPlan(liveverify.Profile(*profile))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	file, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open observation JSON:", err)
		os.Exit(2)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || stat.Size() > maxEvidenceBytes {
		fmt.Fprintln(os.Stderr, "observation file missing or exceeds 2 MiB limit")
		os.Exit(2)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxEvidenceBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxEvidenceBytes {
		fmt.Fprintln(os.Stderr, "cannot read bounded observation JSON")
		os.Exit(2)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var observation liveverify.SuiteObservation
	if err := decoder.Decode(&observation); err != nil {
		fmt.Fprintln(os.Stderr, "invalid observation JSON:", err)
		os.Exit(2)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		fmt.Fprintln(os.Stderr, "invalid observation: trailing JSON content")
		os.Exit(2)
	} else if !errors.Is(err, io.EOF) {
		fmt.Fprintln(os.Stderr, "invalid observation: trailing content:", err)
		os.Exit(2)
	}
	report, err := liveverify.Verify(plan, observation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verification refused:", err)
		os.Exit(2)
	}
	if *format == "json" {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot serialize verification report:", err)
			os.Exit(2)
		}
		fmt.Println(string(out))
		return
	}
	fmt.Printf("FIREDOOR v0.3 / OFFLINE observation verification\nCluster: %s | Context: %s | Profile: %s (operator supplied)\n", report.Cluster, report.Context, report.Profile)
	fmt.Printf("Routes: %d | match: %d | mismatch: %d | inconclusive: %d\nStatus: %s\n", report.Summary.Total, report.Summary.Match, report.Summary.Mismatch, report.Summary.Inconclusive, report.Status)
	for _, route := range report.Routes {
		fmt.Printf("%-14s expected=%-11s observed=%-12s verdict=%-12s attempts=%d\n", route.ID, route.Expected, route.Observed, route.Verdict, route.AttemptCount)
	}
	fmt.Println("Limitations:", report.Limitations)
	// Exit zero means a syntactically valid report was produced, NOT all paths
	// matched. Consumers MUST check report.Status and all route verdicts.
}
