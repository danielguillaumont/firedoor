package attribution

// Pure lifecycle orchestration for the Firedoor A/B/A policy experiment.
// No kubectl, Terraform, subprocess, filesystem, or network operation exists
// in this file. Any future Kubernetes adapter MUST be separately reviewed.
// A missing/unknown UID or failed cleanup is never reported as success.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

const LifecycleSchema = "firedoor.v04.aba-lifecycle.v1"

type PhaseObservation struct {
	Phase        Phase   `json:"phase"`
	Observed     Outcome `json:"observed"`
	AttemptCount int     `json:"attempt_count"`
	Reason       string  `json:"reason"`
}

type LifecycleReport struct {
	Schema              string             `json:"schema"`
	ExperimentID        string             `json:"experiment_id"`
	RouteID             string             `json:"route_id"`
	Cluster             string             `json:"cluster"`
	Context             string             `json:"context"`
	Profile             string             `json:"profile"`
	Status              string             `json:"status"`
	Observations        []PhaseObservation `json:"observations"`
	WitnessUID          string             `json:"witness_uid,omitempty"`
	CreationAttempted   bool               `json:"creation_attempted"`
	CleanupAttempted    bool               `json:"cleanup_attempted"`
	CleanupVerified     bool               `json:"cleanup_verified"`
	RestorationVerified bool               `json:"restoration_verified"`
	Errors              []string           `json:"errors,omitempty"`
	Limitations         []string           `json:"limitations"`
}

// LifecycleDriver abstracts the only operations needed by a future live
// adapter. Every operation MUST operate on the exact approved lab, not arbitrary
// user input. Create MUST return the server-assigned UID on success. A failed
// create with uncertain side effects requires operator recovery, never a PASS.
// Inspect MUST validate the UID, ownership labels and complete witness spec.
// Delete MUST recheck UID immediately before deleting and fail on mismatch.
// VerifyGone MUST confirm the exact named witness no longer exists.
// Probe MUST recheck pinned source/target Pod UID, IP and HTTP body, and only
// classify unreachable after two separate failed network attempts.
// Preflight MUST validate the Terraform hardened state, owned nodes/Pods,
// exact policy inventory, and absence of both witnesses.
type LifecycleDriver interface {
	Preflight(context.Context) error
	Probe(context.Context, Experiment, Phase) (PhaseObservation, error)
	Create(context.Context, Experiment, WitnessObject) (string, error)
	Inspect(context.Context, Experiment, string) error
	Delete(context.Context, Experiment, string) error
	VerifyGone(context.Context, Experiment) error
}

type Lifecycle struct {
	Driver LifecycleDriver
	// Independent cleanup is deliberately NOT canceled with the experiment.
	// A later adapter must use process/OS interrupt handling to let this defer run.
	CleanupTimeout time.Duration
}

func phaseValid(got PhaseObservation, phase Phase) error {
	if got.Phase != phase || got.AttemptCount < 1 || got.AttemptCount > ApprovedPlan().MaxAttemptsPerPhase || got.Reason == "" {
		return fmt.Errorf("%s: missing, tampered or unbounded observation", phase)
	}
	if got.Observed != Reachable && got.Observed != Unreachable {
		return fmt.Errorf("%s: inconclusive or unexpected traffic observation", phase)
	}
	if got.Observed == Unreachable && got.AttemptCount != 2 {
		return fmt.Errorf("%s: a denial requires two separate failed observations", phase)
	}
	return nil
}

func approvedExperiment(ex Experiment) error {
	p := ApprovedPlan()
	if err := Validate(p); err != nil {
		return err
	}
	for _, want := range p.Experiments {
		if reflect.DeepEqual(ex, want) {
			return nil
		}
	}
	return errors.New("unapproved A/B/A experiment")
}

func initialLifecycleReport(ex Experiment) LifecycleReport {
	return LifecycleReport{Schema: LifecycleSchema, ExperimentID: ex.ID, RouteID: ex.RouteID,
		Cluster: OwnedCluster, Context: OwnedContext, Profile: RequiredProfile,
		Status: "incomplete", Observations: []PhaseObservation{},
		Limitations: []string{
			"A/B/A network behavior may support a bounded policy-effect inference but is not a packet-level causal proof.",
			"HTTP failure alone does not prove policy denial; provenance depends on Pod and policy checks.",
			"The report is local, unauthenticated experiment evidence and is not a compliance attestation.",
			"Interrupted or failed cleanup MUST be independently resolved before any further experiment.",
		},
	}
}

// RunOne enforces A1 -> create -> inspect -> B -> delete -> absence ->
// renewed preflight -> A2. It NEVER proceeds after a failed A1 observation.
// A defer independently attempts cleanup whenever policy creation may have
// occurred, including failure, cancellation, and unexpected B observations.
// It never lies about a restored state or an unknown resource UID.
func (l Lifecycle) RunOne(ctx context.Context, ex Experiment) (report LifecycleReport, runErr error) {
	report = initialLifecycleReport(ex)
	if err := approvedExperiment(ex); err != nil {
		report.Status = "rejected"
		return report, err
	}
	if l.Driver == nil {
		report.Status = "rejected"
		return report, errors.New("no lifecycle driver")
	}
	obj, err := ManifestFor(ex)
	if err != nil {
		report.Status = "rejected"
		return report, err
	}
	if err := ValidateWitness(ex, obj); err != nil {
		report.Status = "rejected"
		return report, err
	}
	// Cleanup timeout cannot be user-configured above thirty seconds.
	cleanupLimit := l.CleanupTimeout
	if cleanupLimit <= 0 {
		cleanupLimit = 30 * time.Second
	}
	if cleanupLimit > 30*time.Second {
		report.Status = "rejected"
		return report, errors.New("cleanup deadline cannot exceed thirty seconds")
	}

	fail := func(phase string, e error) error {
		report.Errors = append(report.Errors, phase+": "+e.Error())
		return fmt.Errorf("%s: %w", phase, e)
	}
	if err := l.Driver.Preflight(ctx); err != nil {
		report.Status = "preflight_rejected"
		return report, fail("initial preflight", err)
	}
	a1, err := l.Driver.Probe(ctx, ex, Before)
	if err != nil {
		report.Status = "before_inconclusive"
		return report, fail("before probe", err)
	}
	report.Observations = append(report.Observations, a1)
	if err := phaseValid(a1, Before); err != nil {
		report.Status = "before_inconclusive"
		return report, fail("before probe", err)
	}
	if a1.Observed != Unreachable {
		report.Status = "before_mismatch"
		return report, fail("before probe", errors.New("route is not blocked; witness creation refused"))
	}
	if err := ctx.Err(); err != nil {
		report.Status = "interrupted"
		return report, fail("before creation", err)
	}

	// Create can have uncertain side effects even if it returns an error.
	// Once creation begins, deferred recovery always runs. Empty UID is an
	// explicit manual recovery state, never an automatic name-only deletion.
	report.CreationAttempted = true
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupLimit)
		defer cancel()
		report.CleanupAttempted = true
		if report.WitnessUID == "" {
			report.Status = "manual_recovery_required"
			e := errors.New("create outcome has unknown witness UID; inspect exact named policy manually; automatic deletion forbidden")
			report.Errors = append(report.Errors, "cleanup: "+e.Error())
			runErr = errors.Join(runErr, e)
			return
		}
		// Even on successful B, cleanup MUST be verified before A2 is tested.
		// If a previous best-effort deletion was done, rechecking absence is
		// safe; the adapter may treat NotFound as a successful verification.
		if err := l.Driver.Inspect(cleanupCtx, ex, report.WitnessUID); err != nil {
			// Witness may have already been deleted by the main flow. Only
			// accept that condition after a positive absence confirmation.
			if absentErr := l.Driver.VerifyGone(cleanupCtx, ex); absentErr != nil {
				report.Status = "manual_recovery_required"
				both := fmt.Errorf("witness identity uncertain (%v); absence not proved (%w)", err, absentErr)
				report.Errors = append(report.Errors, "cleanup: "+both.Error())
				runErr = errors.Join(runErr, both)
				return
			}
		} else {
			if err := l.Driver.Delete(cleanupCtx, ex, report.WitnessUID); err != nil {
				report.Status = "manual_recovery_required"
				report.Errors = append(report.Errors, "cleanup delete: "+err.Error())
				runErr = errors.Join(runErr, err)
				return
			}
		}
		if err := l.Driver.VerifyGone(cleanupCtx, ex); err != nil {
			report.Status = "manual_recovery_required"
			report.Errors = append(report.Errors, "cleanup absence: "+err.Error())
			runErr = errors.Join(runErr, err)
			return
		}
		report.CleanupVerified = true
		// Even after a failed B phase or canceled parent context, attempt
		// an independently bounded A2 restoration observation. Retain the
		// original error and never upgrade a failed experiment to PASS.
		restoreCtx, stopRestore := context.WithTimeout(context.Background(), 110*time.Second)
		defer stopRestore()
		if err := l.Driver.Preflight(restoreCtx); err != nil {
			report.Status = "restoration_inconclusive"
			report.Errors = append(report.Errors, "restored preflight: "+err.Error())
			runErr = errors.Join(runErr, err)
			return
		}
		a2, err := l.Driver.Probe(restoreCtx, ex, Restored)
		if err != nil {
			report.Status = "restoration_inconclusive"
			report.Errors = append(report.Errors, "restored probe: "+err.Error())
			runErr = errors.Join(runErr, err)
			return
		}
		report.Observations = append(report.Observations, a2)
		if err := phaseValid(a2, Restored); err != nil {
			report.Status = "restoration_inconclusive"
			report.Errors = append(report.Errors, "restored probe: "+err.Error())
			runErr = errors.Join(runErr, err)
			return
		}
		if a2.Observed != Unreachable {
			report.Status = "restoration_mismatch"
			runErr = errors.Join(runErr, errors.New("route remained reachable after witness was deleted"))
			report.Errors = append(report.Errors, "restored probe: expected unreachable")
			return
		}
		report.RestorationVerified = true
		if runErr == nil && report.Status == "restoration_pending" {
			report.Status = "aba_matched"
		}
	}()

	uid, createErr := l.Driver.Create(ctx, ex, obj)
	report.WitnessUID = uid
	if createErr != nil {
		report.Status = "creation_failed"
		return report, fail("witness creation", createErr)
	}
	if uid == "" {
		report.Status = "creation_failed"
		return report, fail("witness creation", errors.New("server UID missing"))
	}
	if err := l.Driver.Inspect(ctx, ex, uid); err != nil {
		report.Status = "witness_mismatch"
		return report, fail("witness inspection", err)
	}
	b, err := l.Driver.Probe(ctx, ex, Witness)
	if err != nil {
		report.Status = "witness_inconclusive"
		return report, fail("witness probe", err)
	}
	report.Observations = append(report.Observations, b)
	if err := phaseValid(b, Witness); err != nil {
		report.Status = "witness_inconclusive"
		return report, fail("witness probe", err)
	}
	if b.Observed != Reachable {
		report.Status = "witness_mismatch"
		return report, fail("witness probe", errors.New("route did not become reachable with witness"))
	}
	report.Status = "restoration_pending"
	// Deferred cleanup and post-deletion A2 always run before return.
	return report, nil
}

// RunAll executes each of the two immutable experiments sequentially, never
// simultaneously. One failed/incomplete experiment blocks the next.
func (l Lifecycle) RunAll(ctx context.Context) ([]LifecycleReport, error) {
	plan := ApprovedPlan()
	if err := Validate(plan); err != nil {
		return nil, err
	}
	reports := make([]LifecycleReport, 0, len(plan.Experiments))
	for _, ex := range plan.Experiments {
		report, err := l.RunOne(ctx, ex)
		reports = append(reports, report)
		if err != nil {
			return reports, err
		}
		if report.Status != "aba_matched" || !report.CleanupVerified || !report.RestorationVerified {
			return reports, errors.New("incomplete prior experiment; subsequent witness refused")
		}
	}
	return reports, nil
}
