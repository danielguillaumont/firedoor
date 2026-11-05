package attribution

// Offline-only validation of two v0.4 live experiment reports and their
// separately supplied recovery journals. This is consistency validation,
// not authentication or a live Kubernetes inventory check.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const EvidenceBundleSchema = "firedoor.v04.attribution-evidence.v1"
const maxLiveReportBytes = 1024 * 1024

type EvidenceRow struct {
	ExperimentID        string  `json:"experiment_id"`
	RouteID             string  `json:"route_id"`
	Before              Outcome `json:"before"`
	Witness             Outcome `json:"witness_present"`
	Restored            Outcome `json:"restored"`
	Status              string  `json:"status"`
	CleanupVerified     bool    `json:"cleanup_verified"`
	RestorationVerified bool    `json:"restoration_verified"`
	JournalEvents       int     `json:"journal_events"`
}

type EvidenceBundle struct {
	Schema              string        `json:"schema"`
	Cluster             string        `json:"cluster"`
	Context             string        `json:"context"`
	Profile             string        `json:"profile"`
	Status              string        `json:"status"`
	Experiments         int           `json:"experiments"`
	Matched             int           `json:"matched"`
	CleanupVerified     int           `json:"cleanup_verified"`
	RestorationVerified int           `json:"restoration_verified"`
	RecoveryPending     int           `json:"recovery_pending"`
	Rows                []EvidenceRow `json:"rows"`
	Limitations         []string      `json:"limitations"`
}

type EvidencePair struct {
	ReportPath  string
	JournalPath string
}

func readBoundedLiveReport(path string) (LiveRunResult, error) {
	var result LiveRunResult
	if strings.TrimSpace(path) == "" {
		return result, errors.New("explicit report path required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxLiveReportBytes {
		return result, errors.New("live report must be a nonempty bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxLiveReportBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return result, fmt.Errorf("invalid live report: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return result, errors.New("extra JSON after live report")
	}
	return result, nil
}

func samePath(a, b string) bool {
	x, e1 := filepath.Abs(a)
	y, e2 := filepath.Abs(b)
	return e1 == nil && e2 == nil && filepath.Clean(x) == filepath.Clean(y)
}

func verifyEvidencePair(ex Experiment, pair EvidencePair) (EvidenceRow, string, error) {
	var row EvidenceRow
	if pair.ReportPath == "" || pair.JournalPath == "" || samePath(pair.ReportPath, pair.JournalPath) {
		return row, "", errors.New("separate report and journal paths are required")
	}
	r, err := readBoundedLiveReport(pair.ReportPath)
	if err != nil {
		return row, "", fmt.Errorf("%s: %w", ex.ID, err)
	}
	if !samePath(r.ReportPath, pair.ReportPath) || !samePath(r.JournalPath, pair.JournalPath) ||
		!samePath(r.LockPath, filepath.Join(filepath.Dir(pair.ReportPath), liveLockName)) {
		return row, "", errors.New("report/journal/lock paths do not match recorded run identity")
	}
	if r.Schema != LiveRunSchema || r.Cluster != OwnedCluster || r.Context != OwnedContext ||
		r.ExperimentID != ex.ID || r.Status != "aba_matched" || !r.LockReleased || r.RecoveryPending ||
		r.CapturedUTC.IsZero() || !r.CapturedUTC.Equal(r.CapturedUTC.UTC()) {
		return row, "", errors.New("incomplete or untrusted live run identity/status")
	}
	lr := r.Report
	if lr.Schema != LifecycleSchema || lr.ExperimentID != ex.ID || lr.RouteID != ex.RouteID ||
		lr.Cluster != OwnedCluster || lr.Context != OwnedContext || lr.Profile != RequiredProfile ||
		lr.Status != "aba_matched" || !lr.CreationAttempted || !lr.CleanupAttempted ||
		!lr.CleanupVerified || !lr.RestorationVerified || !validUID(lr.WitnessUID) || len(lr.Errors) != 0 ||
		len(lr.Observations) != 3 {
		return row, "", errors.New("lifecycle report is incomplete or inconsistent")
	}
	wantPhases := []Phase{Before, Witness, Restored}
	wantOutcomes := []Outcome{Unreachable, Reachable, Unreachable}
	for i, got := range lr.Observations {
		if err := phaseValid(got, wantPhases[i]); err != nil {
			return row, "", err
		}
		if got.Observed != wantOutcomes[i] {
			return row, "", fmt.Errorf("%s has unexpected phase observation", wantPhases[i])
		}
	}
	// The witness marker must be read from the actual target service; an
	// arbitrary successful command is not sufficient evidence of reachability.
	if lr.Observations[1].Reason != "exact destination HTTP marker returned" {
		return row, "", errors.New("witness phase lacks the exact HTTP marker proof")
	}
	js, err := InspectRecoveryJournal(pair.JournalPath)
	if err != nil {
		return row, "", fmt.Errorf("recovery journal rejected: %w", err)
	}
	if js.PendingManualReview || js.EventCount != 6 || len(js.UnresolvedExperiments) != 0 ||
		len(js.RestorationRecordedFor) != 1 || js.RestorationRecordedFor[0] != ex.ID || !reflect.DeepEqual(js, r.Journal) {
		return row, "", errors.New("journal summary is not an exact completed single-experiment record")
	}
	events, err := readRecoveryEvents(pair.JournalPath)
	if err != nil {
		return row, "", err
	}
	wantEvents := []string{"create_intent", "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed"}
	for i, ev := range events {
		if ev.ExperimentID != ex.ID || ev.Namespace != ex.Witness.PolicyNamespace || ev.PolicyName != ex.Witness.PolicyName || ev.Event != wantEvents[i] {
			return row, "", errors.New("journal event does not belong to the expected experiment sequence")
		}
		if i > 0 && ev.UID != lr.WitnessUID {
			return row, "", errors.New("server-assigned UID changed across journal events")
		}
	}
	row = EvidenceRow{ExperimentID: ex.ID, RouteID: ex.RouteID, Before: lr.Observations[0].Observed,
		Witness: lr.Observations[1].Observed, Restored: lr.Observations[2].Observed,
		Status: lr.Status, CleanupVerified: true, RestorationVerified: true, JournalEvents: len(events)}
	return row, lr.WitnessUID, nil
}

// VerifyEvidenceBundle performs no network calls and never touches live
// Kubernetes policies. Its result means the supplied *local* files are
// consistent, not that they are authenticated or that the cluster is currently safe.
func VerifyEvidenceBundle(edge, data EvidencePair) (EvidenceBundle, error) {
	result := EvidenceBundle{Schema: EvidenceBundleSchema, Cluster: OwnedCluster, Context: OwnedContext,
		Profile: RequiredProfile, Status: "rejected", Rows: []EvidenceRow{},
		Limitations: []string{
			"Offline local files are unauthenticated; an attacker able to edit both a report and journal can forge evidence.",
			"A/B/A observations support a bounded policy-effect inference, not packet-level proof or compliance attestation.",
			"This command does not contact Kubernetes. Independently audit the witness policies and current hardened lab state.",
		}}
	paths := []string{edge.ReportPath, edge.JournalPath, data.ReportPath, data.JournalPath}
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[i] == "" || paths[j] == "" || samePath(paths[i], paths[j]) {
				return result, errors.New("both experiments require four distinct nonempty file paths")
			}
		}
	}
	experiments := ApprovedPlan().Experiments
	first, uid1, err := verifyEvidencePair(experiments[0], edge)
	if err != nil {
		return result, fmt.Errorf("edge-to-data evidence rejected: %w", err)
	}
	second, uid2, err := verifyEvidencePair(experiments[1], data)
	if err != nil {
		return result, fmt.Errorf("data-to-app evidence rejected: %w", err)
	}
	if uid1 == uid2 {
		return result, errors.New("independent experiments reused the same server-assigned witness UID")
	}
	result.Rows = []EvidenceRow{first, second}
	result.Experiments, result.Matched, result.CleanupVerified, result.RestorationVerified = 2, 2, 2, 2
	result.Status = "bounded_policy_effect_observed"
	return result, nil
}

func HumanEvidenceBundle(r EvidenceBundle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "FIREDOOR / v0.4 POLICY ATTRIBUTION (OFFLINE)\nCluster: %s | Profile: %s\n\n", r.Context, r.Profile)
	fmt.Fprintf(&b, "ROUTE         BEFORE       WITNESS      RESTORED     RESULT\n")
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "%-13s %-12s %-12s %-12s %s\n", row.RouteID, row.Before, row.Witness, row.Restored, "MATCH")
	}
	fmt.Fprintf(&b, "\nExperiments: %d | A/B/A matched: %d | Cleanup verified: %d | Restoration verified: %d\n", r.Experiments, r.Matched, r.CleanupVerified, r.RestorationVerified)
	fmt.Fprintf(&b, "RESULT: BOUNDED POLICY EFFECT OBSERVED IN SAVED LAB EVIDENCE\nLIMIT: Offline files are unauthenticated; run the separate live recovery inventory.\n")
	return b.String()
}
