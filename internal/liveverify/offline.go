package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// MaxObservationBytes limits untrusted local evidence before JSON decoding.
const MaxObservationBytes = 2 << 20

// VerifyFile reads a bounded local observation file and applies the immutable
// six-route verification plan. It never accesses Kubernetes or Terraform.
// The evidence is checked for internal consistency, NOT authenticity.
func VerifyFile(inputPath string, profile Profile) (VerificationReport, error) {
	plan, err := NewPlan(profile)
	if err != nil {
		return VerificationReport{}, err
	}
	if strings.TrimSpace(inputPath) == "" {
		return VerificationReport{}, errors.New("--input must name an existing local observation JSON file")
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("open observation file: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() == 0 || stat.Size() > MaxObservationBytes {
		return VerificationReport{}, errors.New("observation input must be a nonempty regular file no larger than 2 MiB")
	}
	contents, err := io.ReadAll(io.LimitReader(file, MaxObservationBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > MaxObservationBytes {
		return VerificationReport{}, errors.New("observation input read failed or exceeded the 2 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var observation SuiteObservation
	if err := decoder.Decode(&observation); err != nil {
		return VerificationReport{}, fmt.Errorf("parse observation JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return VerificationReport{}, errors.New("observation contains trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return VerificationReport{}, fmt.Errorf("observation trailing content: %w", err)
	}
	report, err := Verify(plan, observation)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("observation verification refused: %w", err)
	}
	return report, nil
}

// HumanVerificationReport renders an honest, reviewable result. It intentionally
// states that matching HTTP behavior does not prove policy causality.
func HumanVerificationReport(report VerificationReport) string {
	var b strings.Builder
	fmt.Fprintln(&b, "FIREDOOR v0.3 / Live-observation verification / OFFLINE - READ ONLY")
	fmt.Fprintf(&b, "Cluster: %s | Context: %s | Profile: %s (operator supplied)\n", report.Cluster, report.Context, report.Profile)
	fmt.Fprintf(&b, "Observation: %s to %s\n", report.ObservationStartUTC.Format("2006-01-02T15:04:05Z07:00"), report.ObservationEndUTC.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "Routes: %d | MATCH %d | MISMATCH %d | INCONCLUSIVE %d\n", report.Summary.Total, report.Summary.Match, report.Summary.Mismatch, report.Summary.Inconclusive)
	fmt.Fprintf(&b, "Verdict: %s\n", report.Status)
	for _, route := range report.Routes {
		fmt.Fprintf(&b, "  %-14s %-12s expected=%-11s observed=%-12s attempts=%d\n", route.ID, route.Verdict, route.Expected, route.Observed, route.AttemptCount)
		if route.Verdict != VerdictMatch {
			fmt.Fprintf(&b, "    Reason: %s\n", route.Reason)
		}
	}
	fmt.Fprintln(&b, "Limitations:", report.Limitations)
	return b.String()
}
