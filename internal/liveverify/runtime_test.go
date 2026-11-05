package liveverify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type labFake struct {
	wrongContext bool
	unowned      bool
	wrongPodIP   bool
	execFails    bool
	wrongBody    bool
	execCount    int
	calls        []string
}

func encodeTest(v any) []byte { b, _ := json.Marshal(v); return b }

func testLabels(zone string) map[string]string {
	return map[string]string{
		"firedoor.dev/managed-by":   "terraform",
		"firedoor.dev/zone":         zone,
		"app.kubernetes.io/part-of": "firedoor",
		"app.kubernetes.io/name":    "firedoor-" + zone,
	}
}
func testPod(zone string, ip string) any {
	ns := "firedoor-" + zone
	return map[string]any{
		"metadata": map[string]any{"name": ns + "-abc123-xyz99", "namespace": ns, "uid": "uid-" + zone, "labels": testLabels(zone)},
		"spec":     map[string]any{"nodeName": expectedNodeName, "containers": []any{map[string]string{"name": sourceContainer, "image": "busybox:1.36.1"}}},
		"status": map[string]any{"phase": "Running", "podIP": ip,
			"conditions":        []any{map[string]string{"type": "Ready", "status": "True"}},
			"containerStatuses": []any{map[string]any{"name": sourceContainer, "ready": true}}},
	}
}
func (f *labFake) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, name+" "+joined)
	if name == "kind" && joined == "get clusters" {
		return []byte("firedoor-local\n"), nil
	}
	if name != "kubectl" {
		return nil, errors.New("unexpected binary")
	}
	if joined == "config current-context" {
		if f.wrongContext {
			return []byte("production\n"), nil
		}
		return []byte(OwnedContext + "\n"), nil
	}
	if !strings.HasPrefix(joined, "--context "+OwnedContext+" ") {
		return nil, errors.New("wrong kubectl context")
	}
	if joined == "--context "+OwnedContext+" get nodes -o json" {
		return encodeTest(map[string]any{"items": []any{map[string]any{
			"metadata": map[string]string{"name": expectedNodeName},
			"status":   map[string]any{"conditions": []any{map[string]string{"type": "Ready", "status": "True"}}},
		}}}), nil
	}
	for z, ip := range map[string]string{"edge": "10.88.1.10", "app": "10.88.1.11", "data": "10.88.1.12"} {
		ns := "firedoor-" + z
		if joined == "--context "+OwnedContext+" get namespace "+ns+" -o json" {
			labels := testLabels(z)
			if f.unowned && z == "app" {
				labels["firedoor.dev/managed-by"] = "other"
			}
			return encodeTest(map[string]any{"metadata": map[string]any{"name": ns, "labels": labels}, "status": map[string]string{"phase": "Active"}}), nil
		}
		if joined == "--context "+OwnedContext+" -n "+ns+" get deployment "+ns+" -o json" {
			return encodeTest(map[string]any{
				"metadata": map[string]any{"name": ns, "namespace": ns, "labels": testLabels(z)},
				"spec": map[string]any{"replicas": 1, "template": map[string]any{"spec": map[string]any{
					"containers": []any{map[string]string{"name": sourceContainer, "image": "busybox:1.36.1"}},
				}}},
				"status": map[string]int{"readyReplicas": 1, "availableReplicas": 1},
			}), nil
		}
		if joined == "--context "+OwnedContext+" -n "+ns+" get pods -l app.kubernetes.io/name="+ns+" -o json" {
			return encodeTest(map[string]any{"items": []any{testPod(z, ip)}}), nil
		}
		if joined == "--context "+OwnedContext+" -n "+ns+" get pod "+ns+"-abc123-xyz99 -o json" {
			if f.wrongPodIP && z == "app" {
				ip = "10.88.1.100"
			}
			return encodeTest(testPod(z, ip)), nil
		}
	}
	if strings.Contains(joined, " exec ") {
		f.execCount++
		if f.execFails {
			return nil, errors.New("kubectl transport failure")
		}
		if !strings.Contains(joined, "-c "+sourceContainer+" -- sh -c ") ||
			!strings.Contains(joined, "http://10.88.1.") ||
			!strings.Contains(joined, remoteProbeScript) {
			return nil, fmt.Errorf("unapproved exec argv: %s", joined)
		}
		if f.wrongBody {
			return []byte("FD_WRONG_BODY\n"), nil
		}
		source := ""
		for _, z := range []string{"edge", "app", "data"} {
			if strings.Contains(joined, " -n firedoor-"+z+" exec ") {
				source = z
			}
		}
		url := args[len(args)-2]
		dest := ""
		switch url {
		case "http://10.88.1.10:8080/":
			dest = "edge"
		case "http://10.88.1.11:8080/":
			dest = "app"
		case "http://10.88.1.12:8080/":
			dest = "data"
		default:
			return nil, fmt.Errorf("foreign URL: %s", url)
		}
		if (source == "edge" && dest == "data") || (source == "data" && dest == "app") {
			return []byte("FD_UNREACHABLE\n"), nil
		}
		return []byte("FD_CONNECTED\n"), nil
	}
	return nil, fmt.Errorf("unrecognized read argv: %s", joined)
}

func testExecutor(f *labFake) Executor {
	return Executor{Inspector: Inspector{Runner: f},
		Now:   func() time.Time { return time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC) },
		Pause: func(context.Context, time.Duration) error { return nil }}
}

func TestDiscoveryScopesSingleKindNodeAndOwnedPods(t *testing.T) {
	f := &labFake{}
	inv, err := (Inspector{Runner: f}).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := inv.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(inv.Pods) != 3 {
		t.Fatal("expected three Pods")
	}
	for _, p := range inv.Pods {
		if p.UID == "" || !validPodIP(p.IP) {
			t.Fatal("missing Pod identity")
		}
	}
}

func TestDiscoveryRejectsProductionAndUnownedObjects(t *testing.T) {
	for name, f := range map[string]*labFake{"production-context": {wrongContext: true}, "unowned-namespace": {unowned: true}} {
		t.Run(name, func(t *testing.T) {
			_, err := (Inspector{Runner: f}).Discover(context.Background())
			if err == nil {
				t.Fatal("must fail closed")
			}
			if f.execCount != 0 {
				t.Fatal("exec before successful discovery")
			}
		})
	}
}

func TestHardenedSixRoutesCaptureRealObservationSemantics(t *testing.T) {
	f := &labFake{}
	p, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	result, err := testExecutor(f).Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("unexpected status: %v", result.Status)
	}
	if err := result.ValidateObservation(p); err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 6 {
		t.Fatalf("routes=%d", len(result.Routes))
	}
	for _, route := range result.Routes {
		want := ObservedReachable
		attempts := 1
		if route.Expected == ExpectUnreachable {
			want = ObservedUnreachable
			attempts = 2
		}
		if route.Observed != want || len(route.Attempts) != attempts {
			t.Fatalf("%s: %+v", route.ID, route)
		}
		for _, a := range route.Attempts {
			if a.Reason == "" {
				t.Fatal("missing attempt reason")
			}
		}
	}
	if f.execCount != 8 {
		t.Fatalf("exec count=%d; expected 8", f.execCount)
	}
}

func TestExecOrReadFailuresAreInconclusiveNotDenials(t *testing.T) {
	p, _ := NewPlan(ProfileHardened)
	for name, f := range map[string]*labFake{"exec-failure": {execFails: true}, "wrong-HTTP-body": {wrongBody: true}, "stale-target-Pod": {wrongPodIP: true}} {
		t.Run(name, func(t *testing.T) {
			result, err := testExecutor(f).Run(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "incomplete" {
				t.Fatalf("must report incomplete, got %s", result.Status)
			}
			found := false
			for _, r := range result.Routes {
				if r.Observed == ObservedInconclusive {
					found = true
				}
			}
			if !found {
				t.Fatal("must preserve inconclusive route")
			}
			if name == "stale-target-Pod" && f.execCount >= 8 {
				t.Fatal("stale Pod must prevent affected exec")
			}
		})
	}
}

func TestInventoryAndPlanValidationRejectsForeignTargets(t *testing.T) {
	f := &labFake{}
	inv, err := (Inspector{Runner: f}).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	inv.Pods[0].IP = "8.8.8.8"
	if inv.Validate() == nil {
		t.Fatal("foreign public IP accepted")
	}
	p, _ := NewPlan(ProfileHardened)
	p.Port = 443
	_, err = testExecutor(f).Run(context.Background(), p)
	if err == nil || f.execCount != 0 {
		t.Fatal("invalid plan executed")
	}
}
