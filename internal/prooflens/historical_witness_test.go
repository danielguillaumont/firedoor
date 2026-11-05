package prooflens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

const historicalTestUID = "3f8b3f15-3320-4dd2-a194-420114ead000"

func historicalFixture(t *testing.T, index int) (string, string, attribution.LiveRunResult) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "docs", "evidence", "local")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ex := attribution.ApprovedPlan().Experiments[index]
	now := time.Now().UTC()
	stem := "v04-live-" + ex.RouteID + "-" + now.Format("20060102T150405Z") + "-0123456789abcdef"
	name := stem + ".json"
	journalName := stem + ".jsonl"
	journal, err := attribution.NewRecoveryJournal(filepath.Join(dir, journalName))
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range []string{"create_intent", "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed"} {
		uid := historicalTestUID
		if ev == "create_intent" {
			uid = ""
		}
		if err := journal.AppendEvent(ex, ev, uid); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := attribution.InspectRecoveryJournal(filepath.Join(dir, journalName))
	if err != nil {
		t.Fatal(err)
	}
	observations := []attribution.PhaseObservation{
		{Phase: attribution.Before, Observed: attribution.Unreachable, AttemptCount: 2, Reason: "two failed requests"},
		{Phase: attribution.Witness, Observed: attribution.Reachable, AttemptCount: 1, Reason: "target marker"},
		{Phase: attribution.Restored, Observed: attribution.Unreachable, AttemptCount: 2, Reason: "two failed requests"},
	}
	limitations := []string{"a", "b", "c", "d"}
	run := attribution.LiveRunResult{
		Schema: attribution.LiveRunSchema, Cluster: attribution.OwnedCluster, Context: attribution.OwnedContext,
		ExperimentID: ex.ID, CapturedUTC: now, Status: "aba_matched",
		JournalPath:  `C:\Users\Local\Projects\firedoor\docs\evidence\local\` + journalName,
		ReportPath:   `C:\Users\Local\Projects\firedoor\docs\evidence\local\` + name,
		LockPath:     `C:\Users\Local\Projects\firedoor\docs\evidence\local\.v04-live-aba.lock`,
		LockReleased: true, RecoveryPending: false,
		Report: attribution.LifecycleReport{
			Schema: attribution.LifecycleSchema, ExperimentID: ex.ID, RouteID: ex.RouteID,
			Cluster: attribution.OwnedCluster, Context: attribution.OwnedContext, Profile: attribution.RequiredProfile,
			Status: "aba_matched", Observations: observations, WitnessUID: historicalTestUID,
			CreationAttempted: true, CleanupAttempted: true, CleanupVerified: true, RestorationVerified: true,
			Limitations: limitations,
		},
		Journal: sum, Limitations: limitations,
	}
	writeHistoricalFixture(t, dir, name, run)
	return dir, name, run
}

func writeHistoricalFixture(t *testing.T, dir, name string, run attribution.LiveRunResult) {
	t.Helper()
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalWitnessPairedReportAndJournal(t *testing.T) {
	for index := range attribution.ApprovedPlan().Experiments {
		dir, name, _ := historicalFixture(t, index)
		list, err := DiscoverHistoricalWitnessReports(dir)
		if err != nil || len(list) != 1 || list[0] != name {
			t.Fatalf("scan: %v %v", list, err)
		}
		record, err := InspectHistoricalWitnessPair(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyHistoricalWitnessRecord(record); err != nil {
			t.Fatal(err)
		}
		if record.ProofVerdict != Inconclusive || !record.HistoricalOnly || record.LinkedToV05Receipt || record.CurrentNetworkProbesRun || record.TerraformExecuted ||
			record.Phases[0].Observed != "unreachable" || record.Phases[1].Observed != "reachable" || record.Phases[2].Observed != "unreachable" ||
			record.RecoveryEventCount != 6 || !record.RestorationVerified {
			t.Fatalf("unsafe historical summary: %+v", record)
		}
	}
}

func TestHistoricalWitnessRefusesIncompleteAndChangedEvidence(t *testing.T) {
	cases := map[string]func(t *testing.T, dir, name string, r *attribution.LiveRunResult){
		"wrong_context": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Context = "foreign"
			writeHistoricalFixture(t, dir, name, *r)
		},
		"false_restoration": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Report.RestorationVerified = false
			writeHistoricalFixture(t, dir, name, *r)
		},
		"wrong_phase": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Report.Observations[1].Observed = attribution.Unreachable
			writeHistoricalFixture(t, dir, name, *r)
		},
		"bad_attempt_count": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Report.Observations[0].AttemptCount = 1
			writeHistoricalFixture(t, dir, name, *r)
		},
		"unresolved": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.RecoveryPending = true
			writeHistoricalFixture(t, dir, name, *r)
		},
		"unreleased": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.LockReleased = false
			writeHistoricalFixture(t, dir, name, *r)
		},
		"different_report_path": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.ReportPath = `C:\other.json`
			writeHistoricalFixture(t, dir, name, *r)
		},
		"journal_summary_changed": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Journal.EventCount = 5
			writeHistoricalFixture(t, dir, name, *r)
		},
		"unknown_json_field": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			f := filepath.Join(dir, name)
			raw, _ := os.ReadFile(f)
			raw = append(raw[:len(raw)-1], []byte(`,"password":"should-never-print"}`)...)
			if err := os.WriteFile(f, raw, 0600); err != nil {
				t.Fatal(err)
			}
		},
		"journal_uid_mismatch": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			r.Report.WitnessUID = "86e88faa-2c49-437c-a411-aeb10aa781af"
			writeHistoricalFixture(t, dir, name, *r)
		},
		"missing_journal": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			if err := os.Remove(filepath.Join(dir, strings.TrimSuffix(name, ".json")+".jsonl")); err != nil {
				t.Fatal(err)
			}
		},
		"lock_retained": func(t *testing.T, dir, name string, r *attribution.LiveRunResult) {
			if err := os.WriteFile(filepath.Join(dir, ".v04-live-aba.lock"), []byte("pending"), 0600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for label, edit := range cases {
		t.Run(label, func(t *testing.T) {
			dir, name, r := historicalFixture(t, 0)
			edit(t, dir, name, &r)
			if _, err := InspectHistoricalWitnessPair(dir, name); err == nil {
				t.Fatal("unsafe historical evidence accepted")
			}
		})
	}
}

func TestHistoricalWitnessRejectsPathTraversalAndProofUpgrade(t *testing.T) {
	dir, name, _ := historicalFixture(t, 0)
	for _, candidate := range []string{"../" + name, "C:\\Windows\\secrets.json", ".v04-live-aba.lock", "fake.json"} {
		if _, err := InspectHistoricalWitnessPair(dir, candidate); err == nil {
			t.Fatalf("accepted unsafe path %q", candidate)
		}
	}
	got, err := InspectHistoricalWitnessPair(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	got.ProofVerdict = "supported"
	if err := VerifyHistoricalWitnessRecord(got); err == nil {
		t.Fatal("security verdict upgraded")
	}
	got.ProofVerdict = Inconclusive
	got.LinkedToV05Receipt = true
	if err := VerifyHistoricalWitnessRecord(got); err == nil {
		t.Fatal("historical evidence falsely linked to receipt")
	}
}
