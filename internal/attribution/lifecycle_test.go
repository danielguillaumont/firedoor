package attribution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeLifecycle struct {
	events                                                            []string
	initialErr, createErr, inspectErr, deleteErr, goneErr, restoreErr error
	a1                                                                PhaseObservation
	b                                                                 PhaseObservation
	a2                                                                PhaseObservation
	uid                                                               string
	failB                                                             bool
	cancelParent                                                      context.CancelFunc
	preflightCalls                                                    int
	inspectCalls                                                      int
	present                                                           bool
}

func validPhase(phase Phase) PhaseObservation {
	result := PhaseObservation{Phase: phase, Reason: "synthetic deterministic fixture", AttemptCount: 2, Observed: Unreachable}
	if phase == Witness {
		result.Observed = Reachable
		result.AttemptCount = 1
	}
	return result
}
func newFake() *fakeLifecycle {
	return &fakeLifecycle{uid: "test-uid-123", a1: validPhase(Before), b: validPhase(Witness), a2: validPhase(Restored)}
}
func (f *fakeLifecycle) Preflight(ctx context.Context) error {
	f.events = append(f.events, "preflight")
	f.preflightCalls++
	if f.preflightCalls == 1 {
		return f.initialErr
	}
	if f.present {
		return errors.New("witness still exists")
	}
	return f.restoreErr
}
func (f *fakeLifecycle) Probe(ctx context.Context, _ Experiment, phase Phase) (PhaseObservation, error) {
	f.events = append(f.events, string(phase))
	if phase == Before {
		return f.a1, nil
	}
	if phase == Witness {
		if f.cancelParent != nil {
			f.cancelParent()
			return PhaseObservation{}, ctx.Err()
		}
		if f.failB {
			return PhaseObservation{}, errors.New("probe read failure")
		}
		return f.b, nil
	}
	return f.a2, nil
}
func (f *fakeLifecycle) Create(_ context.Context, _ Experiment, _ WitnessObject) (string, error) {
	f.events = append(f.events, "create")
	f.present = true
	return f.uid, f.createErr
}
func (f *fakeLifecycle) Inspect(_ context.Context, _ Experiment, uid string) error {
	f.events = append(f.events, "inspect")
	f.inspectCalls++
	if uid != f.uid {
		return errors.New("UID changed")
	}
	if f.inspectErr != nil {
		return f.inspectErr
	}
	if !f.present {
		return errors.New("not found")
	}
	return nil
}
func (f *fakeLifecycle) Delete(ctx context.Context, _ Experiment, uid string) error {
	f.events = append(f.events, "delete")
	if uid != f.uid {
		return errors.New("UID changed")
	}
	if ctx.Err() != nil {
		return errors.New("cleanup incorrectly inherited cancellation")
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.present = false
	return nil
}
func (f *fakeLifecycle) VerifyGone(ctx context.Context, _ Experiment) error {
	f.events = append(f.events, "absent")
	if f.goneErr != nil {
		return f.goneErr
	}
	if f.present {
		return errors.New("policy still present")
	}
	return nil
}
func saw(events []string, target string) bool {
	for _, v := range events {
		if v == target {
			return true
		}
	}
	return false
}

func TestLifecycleABAAndOrderedCleanup(t *testing.T) {
	f := newFake()
	r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err != nil || r.Status != "aba_matched" || !r.CleanupVerified || !r.RestorationVerified || len(r.Observations) != 3 {
		t.Fatalf("expected complete safe A/B/A: r=%+v err=%v", r, err)
	}
	want := []string{"preflight", "before", "create", "inspect", "witness_present", "inspect", "delete", "absent", "preflight", "restored"}
	if strings.Join(want, "|") != strings.Join(f.events, "|") {
		t.Fatalf("unsafe lifecycle ordering: got %v want %v", f.events, want)
	}
}
func TestLifecycleRejectsUnsafeBeforeWithoutCreation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*fakeLifecycle)
	}{
		{"reachable_without_witness", func(f *fakeLifecycle) { f.a1.Observed = Reachable }},
		{"single_failed_attempt", func(f *fakeLifecycle) { f.a1.AttemptCount = 1 }},
		{"missing_reason", func(f *fakeLifecycle) { f.a1.Reason = "" }},
		{"forged_phase", func(f *fakeLifecycle) { f.a1.Phase = Restored }},
		{"preflight_drift", func(f *fakeLifecycle) { f.initialErr = errors.New("policy drift") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			tc.edit(f)
			r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
			if err == nil || saw(f.events, "create") || r.CleanupAttempted {
				t.Fatalf("unsafe creation: r=%+v err=%v events=%v", r, err, f.events)
			}
		})
	}
}
func TestLifecycleAlwaysAttemptsCleanupAfterCreatedWitness(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(*fakeLifecycle)
		cleanup bool
		status  string
	}{
		{"witness_not_reachable", func(f *fakeLifecycle) { f.b.Observed = Unreachable; f.b.AttemptCount = 2 }, true, "witness_mismatch"},
		{"inconclusive_probe", func(f *fakeLifecycle) { f.failB = true }, true, "witness_inconclusive"},
		{"witness_identity_mismatch", func(f *fakeLifecycle) { f.inspectErr = errors.New("tampered witness") }, false, "manual_recovery_required"},
		{"delete_failure", func(f *fakeLifecycle) { f.deleteErr = errors.New("no authorization") }, false, "manual_recovery_required"},
		{"absence_not_proved", func(f *fakeLifecycle) { f.goneErr = errors.New("read timeout") }, false, "manual_recovery_required"},
		{"restoration_preflight_drift", func(f *fakeLifecycle) { f.restoreErr = errors.New("policy drift") }, true, "restoration_inconclusive"},
		{"restored_traffic_reachable", func(f *fakeLifecycle) { f.a2.Observed = Reachable }, true, "restoration_mismatch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			tc.edit(f)
			r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
			if err == nil || !r.CleanupAttempted || r.CleanupVerified != tc.cleanup || r.Status != tc.status || !saw(f.events, "absent") && tc.status != "manual_recovery_required" {
				t.Fatalf("cleanup not safely reported: r=%+v err=%v events=%v", r, err, f.events)
			}
		})
	}
}
func TestLifecycleCreationUnknownUIDDemandsManualRecovery(t *testing.T) {
	f := newFake()
	f.uid = ""
	f.createErr = errors.New("ambiguous transport timeout")
	r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil || r.Status != "manual_recovery_required" || r.CleanupVerified || saw(f.events, "delete") || !r.CleanupAttempted {
		t.Fatalf("unsafe unknown-UID handling: r=%+v err=%v events=%v", r, err, f.events)
	}
}
func TestLifecycleCleanupIgnoresCanceledParentContext(t *testing.T) {
	f := newFake()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// A fake preflight deliberately permits cancellation to prove that the
	// deferred cleanup receives a fresh independent context; real adapters
	// SHOULD stop before creation when the parent context is canceled.
	if r, err := (Lifecycle{Driver: f}).RunOne(canceledCtx, ApprovedPlan().Experiments[0]); err == nil || r.CreationAttempted {
		t.Fatalf("canceled context unexpectedly created a witness: %+v, %v", r, err)
	}
	// Cancel the parent DURING B, after the witness has been created. The
	// deferred UID-checked cleanup and A2 use independent contexts.
	f = newFake()
	parent, cancelDuringB := context.WithCancel(context.Background())
	f.cancelParent = cancelDuringB
	r, err := (Lifecycle{Driver: f}).RunOne(parent, ApprovedPlan().Experiments[0])
	if err == nil || !r.CleanupVerified || !r.RestorationVerified || f.present {
		t.Fatalf("cleanup inherited canceled parent context: %+v %v events=%v", r, err, f.events)
	}
}
func TestLifecycleRejectsUnknownExperimentAndUnboundedCleanup(t *testing.T) {
	f := newFake()
	bad := ApprovedPlan().Experiments[0]
	bad.Target.Namespace = "default"
	if _, err := (Lifecycle{Driver: f}).RunOne(context.Background(), bad); err == nil || len(f.events) != 0 {
		t.Fatal("foreign experiment reached driver")
	}
	if _, err := (Lifecycle{Driver: f, CleanupTimeout: 31 * time.Second}).RunOne(context.Background(), ApprovedPlan().Experiments[0]); err == nil || len(f.events) != 0 {
		t.Fatal("unbounded cleanup accepted")
	}
}
func TestLifecycleRunAllStopsAfterFirstFailure(t *testing.T) {
	f := newFake()
	f.b.Observed = Unreachable
	f.b.AttemptCount = 2
	rs, err := (Lifecycle{Driver: f}).RunAll(context.Background())
	if err == nil || len(rs) != 1 || !rs[0].CleanupVerified || f.preflightCalls != 2 || strings.Count(strings.Join(f.events, " "), "create") != 1 {
		t.Fatalf("second experiment ran after failure: %+v %v calls=%v", rs, err, f.events)
	}
}

func TestLifecycleStillChecksRestorationWhenWitnessProbeFails(t *testing.T) {
	f := newFake()
	f.failB = true
	r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil || r.Status != "witness_inconclusive" || !r.CleanupVerified || !r.RestorationVerified ||
		!saw(f.events, "restored") || len(r.Observations) != 2 {
		t.Fatalf("did not independently check restoration after failed B: r=%+v err=%v events=%v", r, err, f.events)
	}
}
func TestLifecycleRunAllHappyPath(t *testing.T) {
	f := newFake()
	rs, err := (Lifecycle{Driver: f}).RunAll(context.Background())
	if err != nil || len(rs) != 2 {
		t.Fatalf("sequential owned experiments rejected: %+v %v", rs, err)
	}
	for _, r := range rs {
		if r.Status != "aba_matched" || !r.CleanupVerified || !r.RestorationVerified {
			t.Fatalf("incomplete experiment: %+v", r)
		}
	}
	if f.present {
		t.Fatal("temporary witness left present")
	}
}

func TestLifecycleAmbiguousCreateWithKnownUIDStillCleansUp(t *testing.T) {
	f := newFake()
	f.createErr = errors.New("transport failed after server creation")
	r, err := (Lifecycle{Driver: f}).RunOne(context.Background(), ApprovedPlan().Experiments[0])
	if err == nil || r.Status != "creation_failed" || !r.CleanupVerified || !r.RestorationVerified || f.present || saw(f.events, "witness_present") {
		t.Fatalf("ambiguous server creation did not clean up: %+v %v events=%v", r, err, f.events)
	}
}
