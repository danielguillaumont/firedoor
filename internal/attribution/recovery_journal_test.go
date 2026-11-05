package attribution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const exampleUID = "3f8b3f15-3320-4dd2-a194-420114ead000"

func newTestJournal(t *testing.T) (*RecoveryJournal, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recovery.jsonl")
	j, err := NewRecoveryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	return j, path
}

func TestRecoveryJournalWrapperPassesSimulatedABA(t *testing.T) {
	j, path := newTestJournal(t)
	fake := newFake()
	fake.uid = exampleUID
	wrapped, err := NewJournaledLifecycleDriver(fake, j)
	if err != nil {
		t.Fatal(err)
	}
	report, err := (Lifecycle{Driver: wrapped}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err != nil || report.Status != "aba_matched" || !report.CleanupVerified || !report.RestorationVerified {
		t.Fatalf("unexpected lifecycle report: %+v err=%v", report, err)
	}
	events, err := readRecoveryEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create_intent", "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed"}
	if len(events) != len(want) {
		t.Fatalf("events: %#v", events)
	}
	for i, ev := range events {
		if ev.Event != want[i] {
			t.Errorf("event %d: got %s want %s", i, ev.Event, want[i])
		}
	}
	sum, err := InspectRecoveryJournal(path)
	if err != nil || sum.PendingManualReview || len(sum.RestorationRecordedFor) != 1 {
		t.Fatalf("summary %+v err %v", sum, err)
	}
}

func TestRecoveryJournalRefusesExistingPathAndMalformedEvents(t *testing.T) {
	j, path := newTestJournal(t)
	if _, err := NewRecoveryJournal(path); err == nil {
		t.Fatal("existing journal overwritten")
	}
	ex := ApprovedPlan().Experiments[0]
	if err := j.AppendEvent(ex, "created_uid", exampleUID); err == nil {
		t.Fatal("UID without intent admitted")
	}
	if err := j.AppendEvent(ex, "create_intent", ""); err != nil {
		t.Fatal(err)
	}
	if err := j.AppendEvent(ex, "restoration_observed", exampleUID); err == nil {
		t.Fatal("forged restoration admitted")
	}
	if err := j.AppendEvent(ex, "created_uid", "not-a-uid"); err == nil {
		t.Fatal("fake UID admitted")
	}
	if err := os.WriteFile(path, []byte(`{"incomplete":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectRecoveryJournal(path); err == nil {
		t.Fatal("partial log accepted")
	}
	if err := j.AppendEvent(ex, "created_uid", exampleUID); err == nil {
		t.Fatal("append to damaged log admitted")
	}
}

func TestRecoveryJournalWriteAheadRefusesMissingJournalBeforeCreate(t *testing.T) {
	j, path := newTestJournal(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	fake := newFake()
	fake.uid = exampleUID
	wrapped, err := NewJournaledLifecycleDriver(fake, j)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Lifecycle{Driver: wrapped}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil || saw(fake.events, "create") {
		t.Fatalf("write-ahead failure allowed creation: %v events=%v", err, fake.events)
	}
}

func TestRecoveryJournalFailedDeleteStaysUnresolved(t *testing.T) {
	j, path := newTestJournal(t)
	fake := newFake()
	fake.uid = exampleUID
	fake.deleteErr = os.ErrPermission
	wrapped, _ := NewJournaledLifecycleDriver(fake, j)
	report, err := (Lifecycle{Driver: wrapped}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil || report.CleanupVerified || report.RestorationVerified {
		t.Fatalf("false success: %+v err=%v", report, err)
	}
	sum, err := InspectRecoveryJournal(path)
	if err != nil || !sum.PendingManualReview || len(sum.UnresolvedExperiments) != 1 {
		t.Fatalf("failed delete not flagged: %+v err=%v", sum, err)
	}
}

func TestRecoveryJournalUnknownCreateNeverInventsUID(t *testing.T) {
	j, path := newTestJournal(t)
	fake := newFake()
	fake.uid = ""
	fake.createErr = os.ErrDeadlineExceeded
	wrapped, _ := NewJournaledLifecycleDriver(fake, j)
	_, err := (Lifecycle{Driver: wrapped}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil {
		t.Fatal("ambiguous create reported success")
	}
	events, err := readRecoveryEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Event != "create_intent" || events[1].Event != "create_uncertain" || events[1].UID != "" {
		t.Fatalf("unknown create not preserved as uncertain: %+v", events)
	}
}

func TestRecoveryJournalRejectsUnauthorizedIdentityAndCorruption(t *testing.T) {
	j, path := newTestJournal(t)
	ex := ApprovedPlan().Experiments[0]
	bad := ex
	bad.Witness.PolicyName = "other-name"
	if err := j.AppendEvent(bad, "create_intent", ""); err == nil {
		t.Fatal("unapproved policy logged")
	}
	if err := j.AppendEvent(ex, "create_intent", ""); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(string(original), OwnedContext, "foreign-context", 1)
	if err := os.WriteFile(path, []byte(modified), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectRecoveryJournal(path); err == nil {
		t.Fatal("altered context accepted")
	}
}

func TestRecoveryJournalTwoSeparateExperiments(t *testing.T) {
	j, path := newTestJournal(t)
	for _, ex := range ApprovedPlan().Experiments {
		for _, kind := range []string{"create_intent", "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed"} {
			uid := exampleUID
			if kind == "create_intent" {
				uid = ""
			}
			if err := j.AppendEvent(ex, kind, uid); err != nil {
				t.Fatal(err)
			}
		}
	}
	sum, err := InspectRecoveryJournal(path)
	if err != nil || sum.PendingManualReview || len(sum.RestorationRecordedFor) != 2 {
		t.Fatalf("two experiment summary %+v err=%v", sum, err)
	}
}
