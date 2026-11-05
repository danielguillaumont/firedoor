package attribution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type baselineReaderFake struct {
	probeResult string
	execError   error
	podDrift    bool
	execCount   int
	podCalls    map[string]int
	verbs       []string
}

func fakeOwnedPodList(ns string, drift bool) []byte {
	zone := strings.TrimPrefix(ns, "firedoor-")
	ip := map[string]string{"firedoor-edge": "10.244.0.11", "firedoor-app": "10.244.0.12", "firedoor-data": "10.244.0.13"}[ns]
	uid := "uid-" + zone
	if drift {
		uid = "replacement-" + uid
	}
	obj := map[string]any{"items": []any{map[string]any{
		"metadata": map[string]any{"name": ns + "-abc", "namespace": ns, "uid": uid,
			"labels": map[string]string{"app.kubernetes.io/name": ns, "firedoor.dev/managed-by": "terraform",
				"firedoor.dev/zone": zone, "app.kubernetes.io/part-of": "firedoor"}},
		"spec": map[string]any{"nodeName": "firedoor-local-control-plane"},
		"status": map[string]any{"phase": "Running", "podIP": ip,
			"conditions":        []any{map[string]any{"type": "Ready", "status": "True"}},
			"containerStatuses": []any{map[string]any{"name": "demo-http", "ready": true}}},
	}}}
	b, _ := json.Marshal(obj)
	return b
}

func (f *baselineReaderFake) Read(ctx context.Context, bin string, args ...string) ([]byte, error) {
	if bin != "kubectl" || len(args) < 7 || args[0] != "--context" || args[1] != OwnedContext || args[2] != "-n" {
		return nil, fmt.Errorf("unexpected reader command %s %v", bin, args)
	}
	ns := args[3]
	f.verbs = append(f.verbs, strings.Join(args, " "))
	switch args[4] {
	case "get":
		if len(args) != 10 || args[9] != "json" || args[8] != "-o" || args[7] != "app.kubernetes.io/name="+ns || args[6] != "-l" || args[5] != "pods" {
			return nil, fmt.Errorf("unexpected Pod get args: %v", args)
		}
		if f.podCalls == nil {
			f.podCalls = map[string]int{}
		}
		f.podCalls[ns]++
		return fakeOwnedPodList(ns, f.podDrift && f.podCalls[ns] > 1), nil
	case "exec":
		f.execCount++
		if len(args) < 12 || args[5] != ns+"-abc" || args[6] != "-c" || args[7] != "demo-http" || args[8] != "--" || args[9] != "sh" || args[10] != "-c" {
			return nil, fmt.Errorf("unapproved exec argv: %v", args)
		}
		if !strings.Contains(args[len(args)-2], "http://10.244.0.") || args[len(args)-1] == "" {
			return nil, errors.New("bad pinned probe arguments")
		}
		if f.execError != nil {
			return nil, f.execError
		}
		return []byte(f.probeResult + "\n"), nil
	}
	return nil, fmt.Errorf("unexpected read-only command: %v", args)
}

func TestRealA1ReadOnlyFakeAdapterConfirmsExactlyTwoDeniedRoutes(t *testing.T) {
	f := &baselineReaderFake{probeResult: "FD_UNREACHABLE"}
	gates := 0
	gate := func(context.Context) error { gates++; return nil }
	report, err := RunReadOnlyA1(context.Background(), f, gate)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "baseline_confirmed" || report.MutationsPerformed || report.Mode != "LIVE_READ_ONLY_A1" || gates != 2 || f.execCount != 4 || len(report.Observations) != 2 {
		t.Fatalf("unsafe or incomplete baseline: %+v gates=%d execs=%d", report, gates, f.execCount)
	}
	for i, obs := range report.Observations {
		if obs.ExperimentID != ApprovedPlan().Experiments[i].ID || obs.Result.Phase != Before || obs.Result.Observed != Unreachable || obs.Result.AttemptCount != 2 {
			t.Fatalf("incomplete route: %+v", obs)
		}
	}
	for _, cmd := range f.verbs {
		if strings.Contains(cmd, " delete ") || strings.Contains(cmd, " create ") || strings.Contains(cmd, " apply ") || strings.Contains(cmd, " patch ") {
			t.Fatalf("forbidden mutation command: %s", cmd)
		}
	}
}

func TestRealA1ReadOnlyRejectsPreflightBeforeExec(t *testing.T) {
	f := &baselineReaderFake{probeResult: "FD_UNREACHABLE"}
	r, err := RunReadOnlyA1(context.Background(), f, func(context.Context) error { return errors.New("drift") })
	if err == nil || r.Status != "preflight_rejected" || f.execCount != 0 || len(f.verbs) != 0 {
		t.Fatalf("failed closed: %+v %v", r, err)
	}
}

func TestRealA1ReadOnlyRejectsPostflight(t *testing.T) {
	f := &baselineReaderFake{probeResult: "FD_UNREACHABLE"}
	gates := 0
	r, err := RunReadOnlyA1(context.Background(), f, func(context.Context) error {
		gates++
		if gates == 2 {
			return errors.New("drift")
		}
		return nil
	})
	if err == nil || r.Status != "postflight_rejected" || len(r.Observations) != 2 {
		t.Fatalf("postflight should fail: %+v %v", r, err)
	}
}

func TestReadOnlyProbeNeverTreatsUntrustedResultAsDenial(t *testing.T) {
	cases := []struct {
		name      string
		marker    string
		execError error
		drift     bool
		status    string
	}{
		{"wrong body", "FD_WRONG_BODY", nil, false, "inconclusive"},
		{"missing wget", "FD_TOOL_MISSING", nil, false, "inconclusive"},
		{"failed kubectl exec", "", errors.New("access denied"), false, "inconclusive"},
		{"unexpected marker", "FD_ANYTHING", nil, false, "inconclusive"},
		{"reachable baseline", "FD_CONNECTED", nil, false, "mismatch"},
		{"replaced pod UID", "FD_UNREACHABLE", nil, true, "inconclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &baselineReaderFake{probeResult: tc.marker, execError: tc.execError, podDrift: tc.drift}
			r, err := RunReadOnlyA1(context.Background(), f, func(context.Context) error { return nil })
			if err == nil || r.Status != tc.status || r.Status == "baseline_confirmed" {
				t.Fatalf("unsafe result: %+v %v", r, err)
			}
		})
	}
}

func TestReadOnlyProbeRejectsUnknownPhaseAndModifiedExperiment(t *testing.T) {
	f := &baselineReaderFake{probeResult: "FD_UNREACHABLE"}
	p := &TrafficProbe{Reader: f}
	ex := ApprovedPlan().Experiments[0]
	_, err := p.Probe(context.Background(), ex, Phase("evil"))
	if err == nil || len(f.verbs) != 0 {
		t.Fatal("unapproved phase reached cluster")
	}
	ex.Source.Namespace = "default"
	_, err = p.Probe(context.Background(), ex, Before)
	if err == nil || len(f.verbs) != 0 {
		t.Fatal("unapproved experiment reached cluster")
	}
}
