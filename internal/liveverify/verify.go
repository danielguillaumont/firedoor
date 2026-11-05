package liveverify

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Slice 3 verifies consistency between the approved test plan and an already
// captured observation report. It does not access Kubernetes or Terraform.
const VerificationSchema = "firedoor.v03.verification.v1"

// Verdict describes evidence consistency, NOT security compliance or policy causality.
type Verdict string

const (
	VerdictMatch        Verdict = "match"
	VerdictMismatch     Verdict = "mismatch"
	VerdictInconclusive Verdict = "inconclusive"
)

type RouteVerification struct {
	ID           string      `json:"id"`
	Source       Zone        `json:"source"`
	Destination  Zone        `json:"destination"`
	Expected     Expectation `json:"expected"`
	Observed     Observation `json:"observed"`
	Verdict      Verdict     `json:"verdict"`
	Reason       string      `json:"reason"`
	SourcePod    string      `json:"source_pod"`
	TargetPod    string      `json:"target_pod"`
	TargetPodIP  string      `json:"target_pod_ip"`
	AttemptCount int         `json:"attempt_count"`
}

type VerificationSummary struct {
	Total        int `json:"total"`
	Match        int `json:"match"`
	Mismatch     int `json:"mismatch"`
	Inconclusive int `json:"inconclusive"`
}

type VerificationReport struct {
	SchemaVersion       string              `json:"schema_version"`
	SourceSchema        string              `json:"source_schema"`
	Cluster             string              `json:"cluster"`
	Context             string              `json:"context"`
	Profile             Profile             `json:"profile"`
	ProfileProvenance   string              `json:"profile_provenance"`
	ObservationStatus   string              `json:"observation_status"`
	ObservationStartUTC time.Time           `json:"observation_started_at_utc"`
	ObservationEndUTC   time.Time           `json:"observation_finished_at_utc"`
	Summary             VerificationSummary `json:"summary"`
	Routes              []RouteVerification `json:"routes"`
	Status              string              `json:"status"`
	Limitations         string              `json:"limitations"`
}

// ValidateForVerification checks that the saved report is internally consistent
// before any match/mismatch judgment. This is not cryptographic attestation:
// a local file can be altered, and provenance is only an operator label.
func (suite SuiteObservation) ValidateForVerification(plan Plan) error {
	if err := suite.ValidateObservation(plan); err != nil {
		return err
	}
	if suite.ProfileProvenance != "operator-supplied; independently verify Terraform state" {
		return errors.New("unsupported or overstated profile provenance")
	}
	if suite.StartedAtUTC.IsZero() || suite.FinishedAtUTC.IsZero() || suite.FinishedAtUTC.Before(suite.StartedAtUTC) {
		return errors.New("missing or reversed suite timestamps")
	}
	if suite.Inventory.CapturedAtUTC.IsZero() || suite.Inventory.CapturedAtUTC.Before(suite.StartedAtUTC) || suite.Inventory.CapturedAtUTC.After(suite.FinishedAtUTC) {
		return errors.New("inventory timestamp outside observation window")
	}
	if strings.TrimSpace(suite.Limitations) == "" {
		return errors.New("observation limitations are missing")
	}
	// A generous fixed bound allows minor operating-system scheduling overhead;
	// it is not a measurement of packet latency or an enforcement guarantee.
	if suite.FinishedAtUTC.Sub(suite.StartedAtUTC) > time.Duration(plan.Limits.MaxSuiteDurationSeconds+10)*time.Second {
		return errors.New("observation suite exceeded permitted wall-clock window")
	}

	anyInconclusive := false
	lastAt := suite.StartedAtUTC
	for i, route := range suite.Routes {
		probe := plan.Probes[i]
		if strings.TrimSpace(route.Reason) == "" {
			return fmt.Errorf("route %s has no reason", route.ID)
		}
		limit := plan.Limits.MaxAttemptsReachable
		if probe.Expected == ExpectUnreachable {
			limit = plan.Limits.MaxAttemptsUnreachable
		}
		if len(route.Attempts) > limit {
			return fmt.Errorf("route %s exceeds its profile-specific attempt limit", route.ID)
		}
		for j, a := range route.Attempts {
			if a.Number != j+1 || strings.TrimSpace(a.Reason) == "" || a.AtUTC.IsZero() ||
				a.AtUTC.Before(lastAt) || a.AtUTC.After(suite.FinishedAtUTC) ||
				a.DurationMillis < 0 || a.DurationMillis > int64(plan.Limits.MaxSuiteDurationSeconds+10)*1000 {
				return fmt.Errorf("route %s has malformed or out-of-order attempt %d", route.ID, j+1)
			}
			switch a.Observation {
			case ObservedReachable:
				if a.Reason != "destination returned the exact expected HTTP body" {
					return fmt.Errorf("route %s claimed reachable without exact-marker evidence", route.ID)
				}
			case ObservedUnreachable:
				if a.Reason != "HTTP request failed; cause is not attributed to a NetworkPolicy" {
					return fmt.Errorf("route %s claimed unreachable without the expected observation marker", route.ID)
				}
			case ObservedInconclusive:
				// An inconclusive execution never yields a network denial.
			default:
				return fmt.Errorf("route %s has unknown attempt result", route.ID)
			}
			lastAt = a.AtUTC
			if j < len(route.Attempts)-1 && (a.Observation == ObservedReachable || a.Observation == ObservedInconclusive) {
				return fmt.Errorf("route %s continued after terminal attempt", route.ID)
			}
		}
		if err := validateFinalObservation(route, plan.Limits.RequiredConsecutiveFailures); err != nil {
			return err
		}
		if route.Observed == ObservedInconclusive {
			anyInconclusive = true
		}
	}
	if anyInconclusive && suite.Status != "incomplete" {
		return errors.New("inconclusive routes must mark suite incomplete")
	}
	if !anyInconclusive && suite.Status != "completed" {
		return errors.New("fully observed suite must mark status completed")
	}
	return nil
}

func validateFinalObservation(r RouteObservation, requiredFailures int) error {
	attempts := r.Attempts
	switch r.Observed {
	case ObservedReachable:
		if len(attempts) == 0 || attempts[len(attempts)-1].Observation != ObservedReachable {
			return fmt.Errorf("route %s claims reachability without final successful attempt", r.ID)
		}
	case ObservedUnreachable:
		if len(attempts) < requiredFailures {
			return fmt.Errorf("route %s claims unreachability without repeated failures", r.ID)
		}
		for _, a := range attempts[len(attempts)-requiredFailures:] {
			if a.Observation != ObservedUnreachable {
				return fmt.Errorf("route %s claims unreachability without consecutive failed HTTP attempts", r.ID)
			}
		}
	case ObservedInconclusive:
		if len(attempts) > 0 {
			last := attempts[len(attempts)-1].Observation
			if last == ObservedReachable {
				return fmt.Errorf("route %s ignores an observed successful request", r.ID)
			}
			if len(attempts) >= requiredFailures {
				allFailed := true
				for _, a := range attempts[len(attempts)-requiredFailures:] {
					if a.Observation != ObservedUnreachable {
						allFailed = false
					}
				}
				if allFailed {
					return fmt.Errorf("route %s ignores consecutive HTTP failure evidence", r.ID)
				}
			}
		}
	default:
		return fmt.Errorf("route %s has invalid final observation", r.ID)
	}
	return nil
}

// Verify compares a validated saved observation to the fixed profile contract.
// Invalid evidence returns an error and no verification verdict. A valid
// mismatch or inconclusive result is returned transparently in the report.
func Verify(plan Plan, suite SuiteObservation) (VerificationReport, error) {
	if err := suite.ValidateForVerification(plan); err != nil {
		return VerificationReport{}, fmt.Errorf("refusing unsupported observation evidence: %w", err)
	}
	report := VerificationReport{
		SchemaVersion: VerificationSchema, SourceSchema: ObservationSchema,
		Cluster: suite.Cluster, Context: suite.Context, Profile: suite.Profile,
		ProfileProvenance: suite.ProfileProvenance, ObservationStatus: suite.Status,
		ObservationStartUTC: suite.StartedAtUTC, ObservationEndUTC: suite.FinishedAtUTC,
		Routes:      make([]RouteVerification, 0, len(plan.Probes)),
		Limitations: "Match means observed HTTP behavior agrees with the fixed lab plan; it does NOT prove Calico/NetworkPolicy causality, effective RBAC, or compliance. Saved JSON is unauthenticated; Terraform profile must be checked separately and contemporaneously when collecting evidence.",
	}
	for _, r := range suite.Routes {
		verdict := VerdictMatch
		reason := "Observed HTTP behavior agrees with the fixed expectation; policy cause not independently established"
		if r.Observed == ObservedInconclusive {
			verdict = VerdictInconclusive
			reason = "No reliable reachability determination: " + r.Reason
		} else if string(r.Observed) != string(r.Expected) {
			verdict = VerdictMismatch
			reason = fmt.Sprintf("Expected %s, observed %s; cause is not independently attributed", r.Expected, r.Observed)
		}
		report.Routes = append(report.Routes, RouteVerification{
			ID: r.ID, Source: r.Source, Destination: r.Destination,
			Expected: r.Expected, Observed: r.Observed, Verdict: verdict,
			Reason: reason, SourcePod: r.SourcePod, TargetPod: r.TargetPod,
			TargetPodIP: r.TargetPodIP, AttemptCount: len(r.Attempts),
		})
		switch verdict {
		case VerdictMatch:
			report.Summary.Match++
		case VerdictMismatch:
			report.Summary.Mismatch++
		case VerdictInconclusive:
			report.Summary.Inconclusive++
		}
	}
	report.Summary.Total = len(report.Routes)
	report.Status = "matched"
	if report.Summary.Inconclusive > 0 {
		report.Status = "inconclusive"
	}
	// An observed deviation must remain visible even if other routes are unknown.
	if report.Summary.Mismatch > 0 {
		report.Status = "mismatched"
	}
	return report, nil
}
