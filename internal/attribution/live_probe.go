package attribution

// Live baseline observation is intentionally read-only. It reuses the same
// fixed probe markers as v0.3 but checks Pod UID/IP stability across attempts.
// No code here can create, patch, apply, or delete a Kubernetes resource.

import (
	"context"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

const BaselineSchema = "firedoor.v04.live-a1-baseline.v1"

const baselineProbeScript = `if ! command -v wget >/dev/null 2>&1; then printf 'FD_TOOL_MISSING\n'; exit 0; fi
body=$(wget -q -T 3 -O - "$1" 2>/dev/null)
rc=$?
if [ "$rc" -ne 0 ]; then printf 'FD_UNREACHABLE\n'; elif [ "$body" = "$2" ]; then printf 'FD_CONNECTED\n'; else printf 'FD_WRONG_BODY\n'; fi`

type pinnedPod struct {
	name string
	uid  string
	ip   string
}

// TrafficProbe receives no shell or argv parameters from outside ApprovedPlan.
// Pod identities are pinned from the first successful discovery and rechecked
// before every exec; a replacement or failed read is inconclusive, not denied.
type TrafficProbe struct {
	Reader CommandReader
	pinned map[string]pinnedPod
}

func (p *TrafficProbe) ownedPod(ctx context.Context, ns string) (podObject, error) {
	if ns != "firedoor-edge" && ns != "firedoor-app" && ns != "firedoor-data" {
		return podObject{}, errors.New("foreign Pod namespace refused")
	}
	raw, err := mustRead(ctx, p.Reader, "kubectl", "--context", OwnedContext,
		"-n", ns, "get", "pods", "-l", applicationLabel+"="+ns, "-o", "json")
	if err != nil {
		return podObject{}, err
	}
	pod, err := readyOwnedPod(raw, ns)
	if err != nil {
		return podObject{}, err
	}
	if p.pinned == nil {
		p.pinned = make(map[string]pinnedPod)
	}
	current := pinnedPod{name: pod.Metadata.Name, uid: pod.Metadata.UID, ip: pod.Status.PodIP}
	if old, seen := p.pinned[ns]; seen && old != current {
		return podObject{}, fmt.Errorf("%s Pod UID, name or IP changed during bounded experiment", ns)
	}
	p.pinned[ns] = current
	return pod, nil
}

func expectedMarker(ex Experiment) (string, error) {
	if err := approvedExperiment(ex); err != nil {
		return "", err
	}
	switch ex.Target.Namespace {
	case "firedoor-app":
		return liveverify.ExpectedHTTPBody(liveverify.ZoneApp)
	case "firedoor-data":
		return liveverify.ExpectedHTTPBody(liveverify.ZoneData)
	default:
		return "", errors.New("foreign probe target")
	}
}

func (p *TrafficProbe) attempt(ctx context.Context, ex Experiment, marker string) (Outcome, string, error) {
	if p.Reader == nil {
		return "", "", errors.New("missing read-only Kubernetes command adapter")
	}
	src, err := p.ownedPod(ctx, ex.Source.Namespace)
	if err != nil {
		return "", "", fmt.Errorf("source Pod identity invalid: %w", err)
	}
	dst, err := p.ownedPod(ctx, ex.Target.Namespace)
	if err != nil {
		return "", "", fmt.Errorf("destination Pod identity invalid: %w", err)
	}
	// The target IP is verified as an owned private IPv4 by readyOwnedPod.
	targetURL := "http://" + dst.Status.PodIP + ":8080/"
	// The shell program is constant; addresses and markers are positional
	// values, never dynamically interpolated shell fragments.
	child, cancel := context.WithTimeout(ctx, time.Duration(ApprovedPlan().AttemptTimeoutMS)*time.Millisecond)
	defer cancel()
	raw, err := p.Reader.Read(child, "kubectl", "--context", OwnedContext, "-n", ex.Source.Namespace,
		"exec", src.Metadata.Name, "-c", "demo-http", "--", "sh", "-c", baselineProbeScript, "_", targetURL, marker)
	if err != nil {
		return "", "", fmt.Errorf("kubectl exec failure (inconclusive): %w", err)
	}
	if len(raw) > 128 {
		return "", "", errors.New("oversized probe output (inconclusive)")
	}
	switch strings.TrimSpace(string(raw)) {
	case "FD_CONNECTED":
		return Reachable, "exact destination HTTP marker returned", nil
	case "FD_UNREACHABLE":
		return Unreachable, "HTTP request failed; cause not attributed", nil
	case "FD_TOOL_MISSING":
		return "", "", errors.New("source container lacks wget (inconclusive)")
	case "FD_WRONG_BODY":
		return "", "", errors.New("unexpected HTTP body (inconclusive)")
	default:
		return "", "", errors.New("unrecognized probe response (inconclusive)")
	}
}

// Probe is reusable by a future operator-approved live driver. It does not
// assume each HTTP failure is a policy denial, and it emits Unreachable only
// after two distinct successful execs both report a failed HTTP attempt.
func (p *TrafficProbe) Probe(ctx context.Context, ex Experiment, phase Phase) (PhaseObservation, error) {
	got := PhaseObservation{Phase: phase}
	if err := approvedExperiment(ex); err != nil {
		return got, err
	}
	if phase != Before && phase != Witness && phase != Restored {
		return got, errors.New("unknown or unapproved experiment phase")
	}
	marker, err := expectedMarker(ex)
	if err != nil {
		return got, err
	}
	for i := 0; i < ApprovedPlan().MaxAttemptsPerPhase; i++ {
		o, why, err := p.attempt(ctx, ex, marker)
		if err != nil {
			return got, err
		}
		got.AttemptCount++
		if o == Reachable {
			got.Observed = Reachable
			got.Reason = why
			return got, nil
		}
		if o != Unreachable {
			return got, errors.New("untrusted HTTP outcome")
		}
		if i+1 < ApprovedPlan().MaxAttemptsPerPhase {
			// Distinct attempts: no cached output is interpreted twice.
			select {
			case <-ctx.Done():
				return got, ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}
	}
	got.Observed = Unreachable
	got.Reason = "two separate failed HTTP attempts; policy causality not established"
	return got, nil
}

type BaselineObservation struct {
	ExperimentID string           `json:"experiment_id"`
	RouteID      string           `json:"route_id"`
	Result       PhaseObservation `json:"result"`
}

type BaselineReport struct {
	Schema             string                `json:"schema"`
	Cluster            string                `json:"cluster"`
	Context            string                `json:"context"`
	Profile            string                `json:"profile"`
	Mode               string                `json:"mode"`
	CapturedAtUTC      time.Time             `json:"captured_at_utc"`
	Status             string                `json:"status"`
	MutationsPerformed bool                  `json:"mutations_performed"`
	Observations       []BaselineObservation `json:"observations"`
	Limitations        []string              `json:"limitations"`
}

// BaselineGate is injected for offline tests; the real entry point wires it to
// the full live, read-only owned-lab preflight before and after the probes.
type BaselineGate func(context.Context) error

// RunReadOnlyA1 gathers real before-phase observations only. A failing
// preflight prevents any exec. A failing/mismatched route stops the run.
func RunReadOnlyA1(ctx context.Context, reader CommandReader, gate BaselineGate) (BaselineReport, error) {
	plan := ApprovedPlan()
	report := BaselineReport{Schema: BaselineSchema, Cluster: OwnedCluster, Context: OwnedContext,
		Profile: RequiredProfile, Mode: "LIVE_READ_ONLY_A1", CapturedAtUTC: time.Now().UTC(),
		Status: "incomplete", MutationsPerformed: false, Observations: []BaselineObservation{},
		Limitations: []string{
			"A1 establishes observed HTTP behavior, not NetworkPolicy causality.",
			"HTTP failures are not proof of packet filtering or effective RBAC.",
			"No temporary allow policy was created; B and A2 remain untested.",
			"Saved local evidence is unauthenticated and must remain Git-ignored.",
		}}
	if err := Validate(plan); err != nil {
		return report, err
	}
	if reader == nil || gate == nil {
		return report, errors.New("missing read-only adapter or preflight")
	}
	if err := gate(ctx); err != nil {
		report.Status = "preflight_rejected"
		return report, fmt.Errorf("initial preflight: %w", err)
	}
	p := &TrafficProbe{Reader: reader}
	for _, ex := range plan.Experiments {
		result, err := p.Probe(ctx, ex, Before)
		if err != nil {
			report.Status = "inconclusive"
			return report, fmt.Errorf("%s: %w", ex.ID, err)
		}
		report.Observations = append(report.Observations, BaselineObservation{ExperimentID: ex.ID, RouteID: ex.RouteID, Result: result})
		if result.Phase != Before || result.Observed != Unreachable || result.AttemptCount != 2 {
			report.Status = "mismatch"
			return report, fmt.Errorf("%s was not confirmed unreachable before witness", ex.ID)
		}
	}
	if err := gate(ctx); err != nil {
		report.Status = "postflight_rejected"
		return report, fmt.Errorf("postflight: %w", err)
	}
	report.Status = "baseline_confirmed"
	return report, nil
}

// RunReadOnlyA1OnKind is the only live entry point for this slice. Neither it
// nor any transitive caller in this package can write Kubernetes resources.
func RunReadOnlyA1OnKind(ctx context.Context) (BaselineReport, error) {
	gate := func(c context.Context) error { _, err := RunPreflight(c, ReadOnlyCommands{}); return err }
	return RunReadOnlyA1(ctx, ReadOnlyCommands{}, gate)
}
