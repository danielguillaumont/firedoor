// Package audit assembles evidence-backed configuration findings from an
// already collected, explicitly owned Kubernetes inventory. It does no I/O.
package audit

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/controls"
	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

const Schema = "firedoor.v02.audit.v1"

const limitations = "Configuration inspection only: does not prove CNI enforcement, actual network reachability, effective RBAC authorization, admission protection, or Terraform drift. The profile label is operator supplied and NOT verified against Terraform state. Unknown findings are not passes. Assessed pass rate is not a security/compliance score."

type Summary struct {
	Total                   int     `json:"total"`
	Pass                    int     `json:"pass"`
	Fail                    int     `json:"fail"`
	Unknown                 int     `json:"unknown"`
	NotApplicable           int     `json:"not_applicable"`
	Assessed                int     `json:"assessed"`
	AssessedPassRatePercent float64 `json:"assessed_pass_rate_percent"`
	AssessmentCoveragePct   float64 `json:"assessment_coverage_percent"`
	Status                  string  `json:"status"`
}

type Report struct {
	SchemaVersion string             `json:"schema_version"`
	CapturedAtUTC time.Time          `json:"captured_at_utc"`
	Cluster       string             `json:"cluster"`
	Context       string             `json:"context"`
	Profile       string             `json:"profile"`
	ProfileSource string             `json:"profile_source"`
	Summary       Summary            `json:"summary"`
	Findings      []controls.Finding `json:"findings"`
	Limitations   string             `json:"limitations"`
}

// Build uses only observed inventory fields. It never validates the chosen
// Terraform profile or makes a live cluster call; the CLI collector does that.
func Build(snapshot kubeinspect.Snapshot, operatorProfile string) (Report, error) {
	findings, err := controls.EvaluateSnapshot(snapshot, operatorProfile)
	if err != nil {
		return Report{}, err
	}
	if len(findings) != 18 {
		return Report{}, fmt.Errorf("expected 18 foundation control findings; got %d", len(findings))
	}
	report := Report{
		SchemaVersion: Schema, CapturedAtUTC: snapshot.CapturedAtUTC,
		Cluster: snapshot.Cluster, Context: snapshot.Context,
		Profile: operatorProfile, ProfileSource: "operator_supplied_unverified",
		Findings: findings, Limitations: limitations,
	}
	report.Summary.Total = len(findings)
	for _, f := range findings {
		if err := f.Validate(); err != nil {
			return Report{}, fmt.Errorf("invalid finding %s/%s: %w", f.Scope.Target.Namespace, f.Control.ID, err)
		}
		switch f.Verdict {
		case controls.Pass:
			report.Summary.Pass++
		case controls.Fail:
			report.Summary.Fail++
		case controls.Unknown:
			report.Summary.Unknown++
		case controls.NotApplicable:
			report.Summary.NotApplicable++
		default:
			return Report{}, fmt.Errorf("invalid finding verdict")
		}
	}
	report.Summary.Assessed = report.Summary.Pass + report.Summary.Fail
	if report.Summary.Assessed > 0 {
		report.Summary.AssessedPassRatePercent = roundOne(float64(report.Summary.Pass) * 100 / float64(report.Summary.Assessed))
	}
	if report.Summary.Total > 0 {
		report.Summary.AssessmentCoveragePct = roundOne(float64(report.Summary.Assessed) * 100 / float64(report.Summary.Total))
	}
	switch {
	case report.Summary.Fail > 0:
		report.Summary.Status = "findings_require_review"
	case report.Summary.Unknown > 0 || report.Summary.NotApplicable > 0:
		report.Summary.Status = "incomplete_assessment"
	default:
		report.Summary.Status = "no_issues_in_covered_configuration_checks"
	}
	return report, nil
}

func roundOne(n float64) float64 { return math.Round(n*10) / 10 }

// Human returns a readable representation with explicit uncertainty and
// evidence for failed or unknown findings. It is not a compliance certificate.
func Human(r Report) string {
	var b strings.Builder
	fmt.Fprintln(&b, "FIREDOOR v0.2 / Configuration security audit / READ ONLY")
	fmt.Fprintf(&b, "Cluster: %s | Context: %s | Observed: %s\n", r.Cluster, r.Context, r.CapturedAtUTC.Format(time.RFC3339))
	fmt.Fprintf(&b, "Profile label: %s (operator supplied, NOT verified against Terraform state)\n", r.Profile)
	fmt.Fprintf(&b, "Findings: %d total | PASS %d | FAIL %d | UNKNOWN %d | N/A %d\n",
		r.Summary.Total, r.Summary.Pass, r.Summary.Fail, r.Summary.Unknown, r.Summary.NotApplicable)
	fmt.Fprintf(&b, "Assessed: %d/%d (%.1f%% coverage) | Pass among assessed: %.1f%% (NOT a security score)\n",
		r.Summary.Assessed, r.Summary.Total, r.Summary.AssessmentCoveragePct, r.Summary.AssessedPassRatePercent)
	fmt.Fprintf(&b, "Disposition: %s\n", r.Summary.Status)
	current := ""
	for _, finding := range r.Findings {
		ns := finding.Scope.Target.Namespace
		if ns != current {
			fmt.Fprintf(&b, "\n[%s]\n", ns)
			current = ns
		}
		fmt.Fprintf(&b, "  %-15s %-8s %s\n", finding.Control.ID, strings.ToUpper(string(finding.Verdict)), finding.Control.Name)
		if finding.Verdict != controls.Pass {
			if finding.Reason != "" {
				fmt.Fprintf(&b, "    Reason: %s\n", finding.Reason)
			}
			for _, ev := range finding.Evidence {
				fmt.Fprintf(&b, "    Evidence: %s/%s %s = %s (expected: %s)\n", ev.Object.Kind, ev.Object.Name, ev.FieldPath, ev.Observed, ev.Expected)
			}
		}
	}
	fmt.Fprintf(&b, "\nLimitations: %s\n", r.Limitations)
	return b.String()
}
