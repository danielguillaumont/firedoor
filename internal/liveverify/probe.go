package liveverify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Observation is a network observation, not a security-control verdict.
type Observation string

const (
	ObservedReachable    Observation = "reachable"
	ObservedUnreachable  Observation = "unreachable"
	ObservedInconclusive Observation = "inconclusive"
)

type Attempt struct {
	Number         int         `json:"number"`
	AtUTC          time.Time   `json:"at_utc"`
	DurationMillis int64       `json:"duration_millis"`
	Observation    Observation `json:"observation"`
	Reason         string      `json:"reason"`
}

type RouteObservation struct {
	ID          string      `json:"id"`
	Source      Zone        `json:"source"`
	Destination Zone        `json:"destination"`
	Expected    Expectation `json:"expected"`
	SourcePod   string      `json:"source_pod"`
	TargetPod   string      `json:"target_pod"`
	TargetPodIP string      `json:"target_pod_ip"`
	Observed    Observation `json:"observed"`
	Reason      string      `json:"reason"`
	Attempts    []Attempt   `json:"attempts"`
}

type SuiteObservation struct {
	SchemaVersion     string             `json:"schema_version"`
	Cluster           string             `json:"cluster"`
	Context           string             `json:"context"`
	Profile           Profile            `json:"profile"`
	ProfileProvenance string             `json:"profile_provenance"`
	StartedAtUTC      time.Time          `json:"started_at_utc"`
	FinishedAtUTC     time.Time          `json:"finished_at_utc"`
	Inventory         Inventory          `json:"inventory"`
	Routes            []RouteObservation `json:"routes"`
	Status            string             `json:"status"`
	Limitations       string             `json:"limitations"`
}

// Shell script is a compile-time constant. The validated, private target IP
// and expected destination marker are passed as positional args, not shell
// fragments. It emits one of four exact markers and does not mutate objects.
const remoteProbeScript = `if ! command -v wget >/dev/null 2>&1; then printf 'FD_TOOL_MISSING\n'; exit 0; fi
body=$(wget -q -T 3 -O - "$1" 2>/dev/null)
rc=$?
if [ "$rc" -ne 0 ]; then printf 'FD_UNREACHABLE\n'; elif [ "$body" = "$2" ]; then printf 'FD_CONNECTED\n'; else printf 'FD_WRONG_BODY\n'; fi`

// Executor enforces the fixed v0.3 plan, refuses unknown Pods, and only execs
// into the three previously verified demo-http containers.
type Executor struct {
	Inspector Inspector
	Now       func() time.Time
	Pause     func(context.Context, time.Duration) error
}

func (e Executor) now() time.Time {
	if e.Now == nil {
		return time.Now().UTC()
	}
	return e.Now().UTC()
}

func (e Executor) pause(ctx context.Context, delay time.Duration) error {
	if e.Pause != nil {
		return e.Pause(ctx, delay)
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (e Executor) attempt(ctx context.Context, src, dst OwnedPod, expectedBody string) (Observation, string) {
	if e.Inspector.Runner == nil {
		return ObservedInconclusive, "missing kubectl runner"
	}
	if !validPodIP(dst.IP) {
		return ObservedInconclusive, "target is not a private IPv4 Pod address"
	}
	// Re-validate BOTH endpoints before each exec so stale Pod IPs are never used.
	if err := e.Inspector.Recheck(ctx, src); err != nil {
		return ObservedInconclusive, "source Pod changed or could not be verified: " + err.Error()
	}
	if err := e.Inspector.Recheck(ctx, dst); err != nil {
		return ObservedInconclusive, "destination Pod changed or could not be verified: " + err.Error()
	}
	target := "http://" + dst.IP + ":8080/"
	args := []string{"--context", OwnedContext, "-n", src.Namespace, "exec", src.PodName, "-c", sourceContainer,
		"--", "sh", "-c", remoteProbeScript, "_", target, expectedBody}
	output, err := e.Inspector.run(ctx, "kubectl", args...)
	if err != nil {
		return ObservedInconclusive, "kubectl exec failed or timed out: " + err.Error()
	}
	switch strings.TrimSpace(string(output)) {
	case "FD_CONNECTED":
		return ObservedReachable, "destination returned the exact expected HTTP body"
	case "FD_UNREACHABLE":
		return ObservedUnreachable, "HTTP request failed; cause is not attributed to a NetworkPolicy"
	case "FD_TOOL_MISSING":
		return ObservedInconclusive, "wget is unavailable inside the source container"
	case "FD_WRONG_BODY":
		return ObservedInconclusive, "HTTP response did not match the destination marker"
	default:
		return ObservedInconclusive, "unexpected or malformed exec result (not a network denial)"
	}
}

// Run discovers a fixed lab inventory and runs at most the Slice 1 budget.
// A read, exec, or timeout failure is never turned into a security PASS.
// No rule in this function attributes reachability to a specific policy.
func (e Executor) Run(ctx context.Context, plan Plan) (SuiteObservation, error) {
	if err := plan.Validate(); err != nil {
		return SuiteObservation{}, err
	}
	if e.Inspector.Runner == nil {
		return SuiteObservation{}, errors.New("no command runner configured")
	}
	suiteCtx, cancel := context.WithTimeout(ctx, time.Duration(plan.Limits.MaxSuiteDurationSeconds)*time.Second)
	defer cancel()
	suite := SuiteObservation{
		SchemaVersion: ObservationSchema, Cluster: OwnedCluster, Context: OwnedContext, Profile: plan.Profile,
		ProfileProvenance: "operator-supplied; independently verify Terraform state",
		StartedAtUTC:      e.now(), Status: "incomplete",
		Limitations: "Network observations only; an HTTP timeout or refusal does not prove Calico or NetworkPolicy causality. This executor does not verify Terraform state or effective RBAC.",
	}
	inv, err := e.Inspector.Discover(suiteCtx)
	if err != nil {
		return SuiteObservation{}, fmt.Errorf("no probes executed; discovery rejected: %w", err)
	}
	suite.Inventory = inv
	for _, p := range plan.Probes {
		route := RouteObservation{ID: p.ID, Source: p.Source, Destination: p.Destination, Expected: p.Expected,
			Observed: ObservedInconclusive, Reason: "not attempted", Attempts: []Attempt{}}
		src, err := inv.pod(p.Source)
		if err != nil {
			return SuiteObservation{}, err
		}
		dst, err := inv.pod(p.Destination)
		if err != nil {
			return SuiteObservation{}, err
		}
		route.SourcePod, route.TargetPod, route.TargetPodIP = src.PodName, dst.PodName, dst.IP
		if suiteCtx.Err() != nil {
			route.Reason = "suite deadline elapsed before attempting route"
			suite.Routes = append(suite.Routes, route)
			continue
		}
		marker, _ := ExpectedHTTPBody(p.Destination)
		maxAttempts := plan.Limits.MaxAttemptsReachable
		if p.Expected == ExpectUnreachable {
			maxAttempts = plan.Limits.MaxAttemptsUnreachable
		}
		consecutiveDenied := 0
		for n := 1; n <= maxAttempts; n++ {
			if suiteCtx.Err() != nil {
				route.Reason = "suite time budget exhausted"
				break
			}
			start := e.now()
			aCtx, stop := context.WithTimeout(suiteCtx, time.Duration(plan.Limits.AttemptTimeoutSeconds+4)*time.Second)
			observed, reason := e.attempt(aCtx, src, dst, marker)
			stop()
			elapsed := e.now().Sub(start)
			if elapsed < 0 {
				elapsed = 0
			}
			route.Attempts = append(route.Attempts, Attempt{Number: n, AtUTC: start, DurationMillis: elapsed.Milliseconds(), Observation: observed, Reason: reason})
			if observed == ObservedInconclusive {
				route.Reason = reason
				break
			}
			if observed == ObservedReachable {
				route.Observed, route.Reason = ObservedReachable, "HTTP reachable with exact destination marker"
				break
			}
			consecutiveDenied++
			if consecutiveDenied >= plan.Limits.RequiredConsecutiveFailures {
				route.Observed, route.Reason = ObservedUnreachable, "two consecutive HTTP failures observed (cause unproven)"
				break
			}
			if n < maxAttempts {
				if err := e.pause(suiteCtx, time.Duration(plan.Limits.RetryDelaySeconds)*time.Second); err != nil {
					route.Reason = "retry cancelled or suite deadline exhausted"
					break
				}
			}
		}
		if route.Observed == ObservedInconclusive && route.Reason == "not attempted" {
			route.Reason = "insufficient repeat observations"
		}
		suite.Routes = append(suite.Routes, route)
	}
	suite.FinishedAtUTC = e.now()
	suite.Status = "completed"
	if len(suite.Routes) != len(plan.Probes) {
		suite.Status = "incomplete"
	}
	for _, route := range suite.Routes {
		if route.Observed == ObservedInconclusive {
			suite.Status = "incomplete"
		}
	}
	return suite, nil
}

// ValidateObservation checks source and destination scope and report completeness.
// It deliberately does not assert that observations match policy expectations.
func (suite SuiteObservation) ValidateObservation(plan Plan) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if suite.SchemaVersion != ObservationSchema || suite.Cluster != OwnedCluster || suite.Context != OwnedContext || suite.Profile != plan.Profile {
		return errors.New("suite identity mismatch")
	}
	if err := suite.Inventory.Validate(); err != nil {
		return err
	}
	if len(suite.Routes) != len(plan.Probes) {
		return errors.New("incomplete route count")
	}
	for index, r := range suite.Routes {
		p := plan.Probes[index]
		if r.ID != p.ID || r.Source != p.Source || r.Destination != p.Destination || r.Expected != p.Expected {
			return fmt.Errorf("route %d does not match fixed plan", index)
		}
		src, _ := suite.Inventory.pod(p.Source)
		dst, _ := suite.Inventory.pod(p.Destination)
		if r.SourcePod != src.PodName || r.TargetPod != dst.PodName || r.TargetPodIP != dst.IP {
			return fmt.Errorf("route %d target identity mismatch", index)
		}
		if len(r.Attempts) > plan.Limits.MaxAttemptsUnreachable {
			return fmt.Errorf("route %d too many attempts", index)
		}
	}
	return nil
}
