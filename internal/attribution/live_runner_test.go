package attribution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/danielguillaumont/firedoor\n\ngo 1.23.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLiveRunnerRequiresFixedExperimentAndDriverBeforeArtifacts(t *testing.T) {
	root := fakeRepo(t)
	_, err := RunOneLiveWithRecovery(context.Background(), root, "outside-route", newFake(), func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("unexpected experiment accepted")
	}
	_, err = RunOneLiveWithRecovery(context.Background(), root, ApprovedPlan().Experiments[0].ID, nil, func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("nil driver accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "evidence")); !os.IsNotExist(err) {
		t.Fatalf("invalid request created recovery artifacts: %v", err)
	}
}

func TestLiveRunnerRecordsAndReleasesOnlyAfterIndependentAudit(t *testing.T) {
	root := fakeRepo(t)
	fake := newFake()
	fake.uid = exampleUID
	auditCalls := 0
	id := ApprovedPlan().Experiments[0].ID
	result, err := RunOneLiveWithRecovery(context.Background(), root, id, fake, func(context.Context) error {
		auditCalls++
		if fake.present {
			return errors.New("witness still present")
		}
		return nil
	})
	if err != nil || result.Report.Status != "aba_matched" || !result.LockReleased || result.RecoveryPending || auditCalls != 1 {
		t.Fatalf("success did not validate all gates: %+v, %v, audits=%d", result, err, auditCalls)
	}
	if _, err := os.Stat(result.LockPath); !os.IsNotExist(err) {
		t.Fatalf("verified run did not release lock: %v", err)
	}
	if _, err := os.Stat(result.JournalPath); err != nil {
		t.Fatalf("missing journal: %v", err)
	}
	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatalf("missing run report: %v", err)
	}
	data, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved LiveRunResult
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.LockReleased || saved.RecoveryPending || saved.Report.Status != "aba_matched" {
		t.Fatalf("saved completion report misstates safety: %+v", saved)
	}
	if _, err := os.Stat(result.ReportPath + ".pending"); !os.IsNotExist(err) {
		t.Fatalf("left pending report: %v", err)
	}
	if result.Journal.PendingManualReview || len(result.Journal.RestorationRecordedFor) != 1 {
		t.Fatalf("unexpected recovery summary: %+v", result.Journal)
	}
	if _, err := RunOneLiveWithRecovery(context.Background(), root, ApprovedPlan().Experiments[1].ID, fakeReadyWithUID(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("subsequent experiment blocked despite fully verified first: %v", err)
	}
}

func fakeReadyWithUID() *fakeLifecycle {
	f := newFake()
	f.uid = exampleUID
	return f
}

func TestLiveRunnerDeleteFailureKeepsLockAndBlocksReexecution(t *testing.T) {
	root := fakeRepo(t)
	f := fakeReadyWithUID()
	f.deleteErr = os.ErrPermission
	id := ApprovedPlan().Experiments[0].ID
	res, err := RunOneLiveWithRecovery(context.Background(), root, id, f, func(context.Context) error { return nil })
	if err == nil || res.LockReleased || !res.RecoveryPending || res.Report.CleanupVerified {
		t.Fatalf("failed cleanup falsely accepted: %+v err=%v", res, err)
	}
	if _, err := os.Stat(res.LockPath); err != nil {
		t.Fatalf("missing crash lock: %v", err)
	}
	f2 := fakeReadyWithUID()
	_, err = RunOneLiveWithRecovery(context.Background(), root, id, f2, func(context.Context) error { return nil })
	if err == nil || saw(f2.events, "create") {
		t.Fatalf("second run escaped crash lock: %v events=%v", err, f2.events)
	}
}

func TestLiveRunnerAuditFailurePreservesLockAndCleanJournal(t *testing.T) {
	root := fakeRepo(t)
	res, err := RunOneLiveWithRecovery(context.Background(), root, ApprovedPlan().Experiments[0].ID, fakeReadyWithUID(), func(context.Context) error { return errors.New("postflight drift") })
	if err == nil || !strings.Contains(err.Error(), "post-run") || res.LockReleased || !res.RecoveryPending {
		t.Fatalf("postflight failed open: %+v err=%v", res, err)
	}
	sum, err := InspectRecoveryJournal(res.JournalPath)
	if err != nil || sum.PendingManualReview {
		t.Fatalf("journal itself should have confirmed restoration: %+v err=%v", sum, err)
	}
	if _, err := os.Stat(res.LockPath); err != nil {
		t.Fatalf("post-audit failure must retain lock: %v", err)
	}
}

func TestLiveRunnerRefusesUnresolvedPastJournal(t *testing.T) {
	root := fakeRepo(t)
	dir := filepath.Join(root, "docs", "evidence", "local")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "v04-live-old.jsonl")
	j, err := NewRecoveryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.AppendEvent(ApprovedPlan().Experiments[0], "create_intent", ""); err != nil {
		t.Fatal(err)
	}
	f := fakeReadyWithUID()
	_, err = RunOneLiveWithRecovery(context.Background(), root, ApprovedPlan().Experiments[1].ID, f, func(context.Context) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "unresolved") || saw(f.events, "create") {
		t.Fatalf("old incomplete journal not respected: %v events=%v", err, f.events)
	}
	if _, err := os.Stat(filepath.Join(dir, liveLockName)); err != nil {
		t.Fatalf("recovery lock was not retained: %v", err)
	}
}

func TestLiveRunnerRejectsExternalOrUnexpectedRepository(t *testing.T) {
	_, err := RunOneLiveWithRecovery(context.Background(), t.TempDir(), ApprovedPlan().Experiments[0].ID, fakeReadyWithUID(), func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("non-Firedoor folder accepted")
	}
}
