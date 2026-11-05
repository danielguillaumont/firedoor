package prooflens

// Slice 3C1 reads paired, previously saved Firedoor v0.4 A/B/A reports and
// recovery journals. It never starts Kubernetes/Terraform processes, mutates
// NetworkPolicies, or upgrades a historical observation to a security proof.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

const (
	HistoricalWitnessSchema = "firedoor.v05.historical-witness.v1"
	HistoricalWitnessMode   = "HISTORICAL_V04_ABA_LOCAL_READ_ONLY"
	historicalReportMax     = 256 * 1024
	historicalJournalMax    = 1024 * 1024
)

var historicalReportName = regexp.MustCompile(`^v04-live-(edge-to-data|data-to-app)-(\d{8}T\d{6}Z)-([a-f0-9]{16})\.json$`)

type HistoricalWitnessPhase struct {
	Phase    string `json:"phase"`
	Observed string `json:"observed"`
	Attempts int    `json:"attempts"`
}

type HistoricalWitnessRecord struct {
	SchemaVersion           string                   `json:"schema_version"`
	Mode                    string                   `json:"mode"`
	HistoricalOnly          bool                     `json:"historical_only"`
	CapturedUTC             string                   `json:"captured_utc"`
	ExperimentID            string                   `json:"experiment_id"`
	RouteID                 string                   `json:"route_id"`
	Cluster                 string                   `json:"cluster"`
	Context                 string                   `json:"context"`
	Profile                 string                   `json:"profile"`
	ReportFile              string                   `json:"report_file"`
	ReportSHA256            string                   `json:"report_sha256"`
	JournalFile             string                   `json:"journal_file"`
	JournalSHA256           string                   `json:"journal_sha256"`
	Phases                  []HistoricalWitnessPhase `json:"phases"`
	RecoveryEventCount      int                      `json:"recovery_event_count"`
	CleanupVerified         bool                     `json:"cleanup_verified"`
	RestorationVerified     bool                     `json:"restoration_verified"`
	LockReleased            bool                     `json:"lock_released"`
	RecoveryPending         bool                     `json:"recovery_pending"`
	CurrentNetworkProbesRun bool                     `json:"current_network_probes_run"`
	TerraformExecuted       bool                     `json:"terraform_executed"`
	LinkedToV05Receipt      bool                     `json:"linked_to_v05_receipt"`
	ProofVerdict            string                   `json:"proof_verdict"`
	Limitations             []string                 `json:"limitations"`
	RecordSHA256            string                   `json:"record_sha256"`
}

func historicalLimitations() []string {
	return []string{
		"This is a historical v0.4 local A/B/A HTTP observation, not a fresh traffic test or current cluster assessment.",
		"A/B/A HTTP behavior supports a bounded policy-effect inference, not packet-level proof of network causality.",
		"Report and recovery journal SHA256 digests detect local inconsistency; they are not signatures or authenticated attestations.",
		"Historical evidence is not contemporaneous with the v0.5 Terraform plan receipt or Kubernetes observation and is not yet linked to either.",
		"No Terraform apply, Kubernetes mutation, traffic probe, or database write occurs during this inspection.",
	}
}

func historicalDigest(r HistoricalWitnessRecord) (string, error) {
	r.RecordSHA256 = ""
	encoded, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func historicalNameInfo(name string) (string, time.Time, error) {
	m := historicalReportName.FindStringSubmatch(name)
	if len(m) != 4 {
		return "", time.Time{}, errors.New("report filename is not one of the exact approved historical witness forms")
	}
	t, err := time.Parse("20060102T150405Z", m[2])
	if err != nil {
		return "", time.Time{}, errors.New("invalid report filename timestamp")
	}
	return m[1], t.UTC(), nil
}

// Read-only scan returns ONLY filenames. It never decodes saved report content.
// A human must choose an individual report for validation; failed historical
// attempts are not automatically upgraded or hidden by selecting the newest.
func DiscoverHistoricalWitnessReports(dir string) ([]string, error) {
	if err := safeHistoricalDir(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) > 512 {
		return nil, errors.New("historical evidence directory too large")
	}
	names := make([]string, 0)
	for _, ent := range entries {
		name := ent.Name()
		if !historicalReportName.MatchString(name) {
			continue
		}
		if !ent.Type().IsRegular() {
			return nil, errors.New("historical report has unsafe file type")
		}
		names = append(names, name)
		if len(names) > 32 {
			return nil, errors.New("too many historical witness reports")
		}
	}
	return names, nil // os.ReadDir already returns name-sorted entries
}

func safeHistoricalDir(dir string) error {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if filepath.Base(absolute) != "local" || filepath.Base(filepath.Dir(absolute)) != "evidence" ||
		filepath.Base(filepath.Dir(filepath.Dir(absolute))) != "docs" {
		return errors.New("historical reports must be under docs/evidence/local")
	}
	for _, path := range []string{filepath.Dir(filepath.Dir(absolute)), filepath.Dir(absolute), absolute} {
		st, err := os.Lstat(path)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe historical evidence directory")
		}
	}
	return nil
}

func boundedRegularFile(path string, limit int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() <= 0 || st.Size() > limit {
		return nil, errors.New("missing, unsafe or oversized historical artifact")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot open historical artifact")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, errors.New("historical artifact changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || len(data) == 0 || int64(len(data)) > limit {
		return nil, errors.New("historical artifact cannot be read within limits")
	}
	return data, nil
}

func decodeHistoricalReport(raw []byte) (attribution.LiveRunResult, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r attribution.LiveRunResult
	if err := dec.Decode(&r); err != nil {
		return r, errors.New("invalid historical report JSON")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return r, errors.New("historical report has trailing JSON")
	}
	return r, nil
}

// Windows-written paths in old JSON are untrusted *metadata* only. Never open
// them; open exclusively the two validated files inside the supplied directory.
func historicalStoredBase(path string) string {
	return filepath.Base(strings.ReplaceAll(path, "\\", "/"))
}

func phaseSummary(r attribution.LifecycleReport) ([]HistoricalWitnessPhase, error) {
	wantPhases := []attribution.Phase{attribution.Before, attribution.Witness, attribution.Restored}
	wantOutcomes := []attribution.Outcome{attribution.Unreachable, attribution.Reachable, attribution.Unreachable}
	if len(r.Observations) != 3 {
		return nil, errors.New("historical witness does not contain exactly three A/B/A observations")
	}
	out := make([]HistoricalWitnessPhase, 0, 3)
	for i, got := range r.Observations {
		if got.Phase != wantPhases[i] || got.Observed != wantOutcomes[i] ||
			got.AttemptCount < 1 || got.AttemptCount > 2 ||
			(got.Observed == attribution.Unreachable && got.AttemptCount != 2) ||
			len(strings.TrimSpace(got.Reason)) == 0 || len(got.Reason) > 256 {
			return nil, errors.New("historical witness A/B/A phase or attempt count is invalid")
		}
		out = append(out, HistoricalWitnessPhase{Phase: string(got.Phase), Observed: string(got.Observed), Attempts: got.AttemptCount})
	}
	return out, nil
}

func validateJournalUIDs(raw []byte, uid, experimentID string) error {
	// InspectRecoveryJournal already validates sequencing and cross-event UIDs.
	// Compare those event UIDs against the separate lifecycle report as well.
	count := 0
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		var ev attribution.RecoveryEvent
		if dec.Decode(&ev) != nil || ev.ExperimentID != experimentID ||
			(ev.UID != "" && ev.UID != uid) {
			return errors.New("historical journal does not match lifecycle report UID")
		}
		if ev.UID != "" {
			count++
		}
	}
	if count < 4 {
		return errors.New("historical journal missing required UID events")
	}
	return nil
}

// InspectHistoricalWitnessPair deliberately accepts only a basename, never an
// absolute or user-selected arbitrary path. No writing or live CLI calls occur.
func InspectHistoricalWitnessPair(dir, reportName string) (HistoricalWitnessRecord, error) {
	var empty HistoricalWitnessRecord
	if err := safeHistoricalDir(dir); err != nil {
		return empty, err
	}
	route, nameTime, err := historicalNameInfo(reportName)
	if err != nil {
		return empty, err
	}
	journalName := strings.TrimSuffix(reportName, ".json") + ".jsonl"
	lock := filepath.Join(dir, ".v04-live-aba.lock")
	if _, err := os.Lstat(lock); err == nil {
		return empty, errors.New("v0.4 recovery lock exists; historical review refused")
	} else if !os.IsNotExist(err) {
		return empty, errors.New("v0.4 recovery lock state unreadable")
	}
	rawReport, err := boundedRegularFile(filepath.Join(dir, reportName), historicalReportMax)
	if err != nil {
		return empty, err
	}
	rawJournal, err := boundedRegularFile(filepath.Join(dir, journalName), historicalJournalMax)
	if err != nil {
		return empty, err
	}
	r, err := decodeHistoricalReport(rawReport)
	if err != nil {
		return empty, err
	}
	var approved *attribution.Experiment
	for _, ex := range attribution.ApprovedPlan().Experiments {
		if ex.RouteID == route && ex.ID == r.ExperimentID {
			chosen := ex
			approved = &chosen
			break
		}
	}
	if approved == nil || r.Schema != attribution.LiveRunSchema || r.Cluster != attribution.OwnedCluster ||
		r.Context != attribution.OwnedContext || r.Status != "aba_matched" || r.RecoveryPending || !r.LockReleased ||
		r.Report.Schema != attribution.LifecycleSchema || r.Report.Cluster != attribution.OwnedCluster ||
		r.Report.Context != attribution.OwnedContext || r.Report.Profile != attribution.RequiredProfile ||
		r.Report.ExperimentID != approved.ID || r.Report.RouteID != approved.RouteID ||
		r.Report.Status != "aba_matched" || !r.Report.CreationAttempted || !r.Report.CleanupAttempted ||
		!r.Report.CleanupVerified || !r.Report.RestorationVerified || len(r.Report.Errors) != 0 ||
		len(r.Report.Limitations) != 4 || len(r.Limitations) != 4 ||
		r.CapturedUTC.IsZero() || r.CapturedUTC.Location() != time.UTC ||
		!r.CapturedUTC.Truncate(time.Second).Equal(nameTime) ||
		historicalStoredBase(r.ReportPath) != reportName || historicalStoredBase(r.JournalPath) != journalName ||
		historicalStoredBase(r.LockPath) != ".v04-live-aba.lock" {
		return empty, errors.New("historical A/B/A report is incomplete or has mismatched identity/cleanup")
	}
	phases, err := phaseSummary(r.Report)
	if err != nil {
		return empty, err
	}
	summary, err := attribution.InspectRecoveryJournal(filepath.Join(dir, journalName))
	if err != nil || summary.PendingManualReview || summary.EventCount != 6 ||
		!reflect.DeepEqual(summary.RestorationRecordedFor, []string{r.ExperimentID}) ||
		!reflect.DeepEqual(summary, r.Journal) {
		return empty, errors.New("historical recovery journal differs from final report or restoration incomplete")
	}
	if err := validateJournalUIDs(rawJournal, r.Report.WitnessUID, r.ExperimentID); err != nil {
		return empty, err
	}
	a := sha256.Sum256(rawReport)
	b := sha256.Sum256(rawJournal)
	out := HistoricalWitnessRecord{
		SchemaVersion: HistoricalWitnessSchema, Mode: HistoricalWitnessMode,
		HistoricalOnly: true, CapturedUTC: r.CapturedUTC.Format(time.RFC3339Nano),
		ExperimentID: r.ExperimentID, RouteID: r.Report.RouteID,
		Cluster: r.Cluster, Context: r.Context, Profile: r.Report.Profile,
		ReportFile: reportName, ReportSHA256: hex.EncodeToString(a[:]),
		JournalFile: journalName, JournalSHA256: hex.EncodeToString(b[:]),
		Phases: phases, RecoveryEventCount: summary.EventCount,
		CleanupVerified: r.Report.CleanupVerified, RestorationVerified: r.Report.RestorationVerified,
		LockReleased: r.LockReleased, RecoveryPending: r.RecoveryPending,
		CurrentNetworkProbesRun: false, TerraformExecuted: false, LinkedToV05Receipt: false,
		ProofVerdict: Inconclusive, Limitations: historicalLimitations(),
	}
	out.RecordSHA256, err = historicalDigest(out)
	if err != nil {
		return empty, err
	}
	if err := VerifyHistoricalWitnessRecord(out); err != nil {
		return empty, err
	}
	return out, nil
}

func VerifyHistoricalWitnessRecord(r HistoricalWitnessRecord) error {
	route, t, err := historicalNameInfo(r.ReportFile)
	if err != nil {
		return err
	}
	if r.SchemaVersion != HistoricalWitnessSchema || r.Mode != HistoricalWitnessMode || !r.HistoricalOnly ||
		!validUTC(r.CapturedUTC) || r.RouteID != route ||
		!strings.HasSuffix(r.ExperimentID, route) ||
		r.Cluster != attribution.OwnedCluster || r.Context != attribution.OwnedContext ||
		r.Profile != attribution.RequiredProfile || r.JournalFile != strings.TrimSuffix(r.ReportFile, ".json")+".jsonl" ||
		!validHex(r.ReportSHA256, 64) || !validHex(r.JournalSHA256, 64) ||
		r.RecoveryEventCount != 6 || !r.CleanupVerified || !r.RestorationVerified || !r.LockReleased || r.RecoveryPending ||
		r.CurrentNetworkProbesRun || r.TerraformExecuted || r.LinkedToV05Receipt || r.ProofVerdict != Inconclusive ||
		!reflect.DeepEqual(r.Limitations, historicalLimitations()) || len(r.Phases) != 3 || !validHex(r.RecordSHA256, 64) {
		return errors.New("invalid or overstated historical witness record")
	}
	parsed, err := time.Parse(time.RFC3339Nano, r.CapturedUTC)
	if err != nil || !parsed.Truncate(time.Second).Equal(t) {
		return errors.New("historical filename/timestamp mismatch")
	}
	for i, phase := range r.Phases {
		expectedPhase := []string{string(attribution.Before), string(attribution.Witness), string(attribution.Restored)}[i]
		expectedOutcome := []string{string(attribution.Unreachable), string(attribution.Reachable), string(attribution.Unreachable)}[i]
		if phase.Phase != expectedPhase || phase.Observed != expectedOutcome || phase.Attempts < 1 || phase.Attempts > 2 ||
			(phase.Observed == string(attribution.Unreachable) && phase.Attempts != 2) {
			return errors.New("invalid historical phase sequence")
		}
	}
	digest, err := historicalDigest(r)
	if err != nil || digest != r.RecordSHA256 {
		return fmt.Errorf("historical witness record checksum mismatch")
	}
	return nil
}
