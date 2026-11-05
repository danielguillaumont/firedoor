package attribution

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBundleEvidence(t *testing.T) (EvidencePair, EvidencePair) {
	t.Helper()
	root := fakeRepo(t)
	makeRun := func(id, uid string) EvidencePair {
		f := newFake()
		f.uid = uid
		f.b.Reason = "exact destination HTTP marker returned"
		result, err := RunOneLiveWithRecovery(context.Background(), root, id, f, func(context.Context) error {
			if f.present {
				t.Fatal("witness still present at final audit")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("fixture run failed: %v", err)
		}
		return EvidencePair{ReportPath: result.ReportPath, JournalPath: result.JournalPath}
	}
	return makeRun("witness-edge-to-data", "3f8b3f15-3320-4dd2-a194-420114ead000"),
		makeRun("witness-data-to-app", "3f8b3f15-3320-4dd2-a194-420114ead001")
}

func TestV04EvidenceImporterAcceptsTwoSeparateCompletedRuns(t *testing.T) {
	edge, data := testBundleEvidence(t)
	got, err := VerifyEvidenceBundle(edge, data)
	if err != nil {
		t.Fatalf("valid saved evidence rejected: %v", err)
	}
	if got.Status != "bounded_policy_effect_observed" || got.Experiments != 2 || got.CleanupVerified != 2 || got.RestorationVerified != 2 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if got.Rows[0].RouteID != "edge-to-data" || got.Rows[1].RouteID != "data-to-app" ||
		!strings.Contains(HumanEvidenceBundle(got), "BOUNDED POLICY EFFECT") {
		t.Fatalf("unexpected human output: %+v", got)
	}
}

func mutateReport(t *testing.T, path string, change func(*LiveRunResult)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r LiveRunResult
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	change(&r)
	b, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestV04EvidenceImporterRejectsIncompleteOrTamperedRuns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*LiveRunResult)
	}{
		{"cleanup false", func(r *LiveRunResult) { r.Report.CleanupVerified = false }},
		{"restoration false", func(r *LiveRunResult) { r.Report.RestorationVerified = false }},
		{"lock retained", func(r *LiveRunResult) { r.LockReleased = false }},
		{"recovery pending", func(r *LiveRunResult) { r.RecoveryPending = true }},
		{"wrong identity", func(r *LiveRunResult) { r.ExperimentID = "witness-data-to-app" }},
		{"wrong cluster", func(r *LiveRunResult) { r.Context = "production" }},
		{"wrong phase", func(r *LiveRunResult) { r.Report.Observations[1].Observed = Unreachable }},
		{"wrong marker", func(r *LiveRunResult) { r.Report.Observations[1].Reason = "success exit code" }},
		{"wrong UID", func(r *LiveRunResult) { r.Report.WitnessUID = "3f8b3f15-3320-4dd2-a194-420114ead099" }},
		{"wrong journal summary", func(r *LiveRunResult) { r.Journal.EventCount = 3 }},
		{"wrong journal file path", func(r *LiveRunResult) { r.JournalPath = "another.jsonl" }},
		{"wrong report file path", func(r *LiveRunResult) { r.ReportPath = "another.json" }},
		{"missing observation", func(r *LiveRunResult) { r.Report.Observations = r.Report.Observations[:2] }},
		{"error recorded", func(r *LiveRunResult) { r.Report.Errors = []string{"cleanup failed"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edge, data := testBundleEvidence(t)
			mutateReport(t, edge.ReportPath, tc.change)
			if _, err := VerifyEvidenceBundle(edge, data); err == nil {
				t.Fatal("unsafe report was accepted")
			}
		})
	}
}

func TestV04EvidenceImporterRejectsInvalidJournalsAndPaths(t *testing.T) {
	for _, scenario := range []string{"duplicate", "swapped", "truncate", "tamperUID", "missingJournal", "extraJSON"} {
		t.Run(scenario, func(t *testing.T) {
			edge, data := testBundleEvidence(t)
			switch scenario {
			case "duplicate":
				data.ReportPath = edge.ReportPath
			case "swapped":
				edge, data = data, edge
			case "truncate":
				b, _ := os.ReadFile(edge.JournalPath)
				index := strings.LastIndex(strings.TrimSpace(string(b)), "\n")
				if index < 0 {
					t.Fatal("journal fixture too short")
				}
				if err := os.WriteFile(edge.JournalPath, b[:index+1], 0600); err != nil {
					t.Fatal(err)
				}
			case "tamperUID":
				b, _ := os.ReadFile(edge.JournalPath)
				a := strings.Replace(string(b), "3f8b3f15-3320-4dd2-a194-420114ead000", "3f8b3f15-3320-4dd2-a194-420114ead066", 1)
				if err := os.WriteFile(edge.JournalPath, []byte(a), 0600); err != nil {
					t.Fatal(err)
				}
			case "missingJournal":
				edge.JournalPath = filepath.Join(t.TempDir(), "missing.jsonl")
			case "extraJSON":
				f, err := os.OpenFile(edge.ReportPath, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString("{}\n")
				_ = f.Close()
			}
			if _, err := VerifyEvidenceBundle(edge, data); err == nil {
				t.Fatal("unsafe evidence was accepted")
			}
		})
	}
}
