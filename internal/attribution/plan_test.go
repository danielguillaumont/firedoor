package attribution

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestApprovedPlanHasOnlyOwnedHardenedExperiments(t *testing.T) {
	p := ApprovedPlan()
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	if p.Cluster != OwnedCluster || p.Context != OwnedContext || p.Profile != RequiredProfile {
		t.Fatal("plan escaped the owned hardened cluster")
	}
	if len(p.Experiments) != 2 {
		t.Fatalf("want precisely two experiments, got %d", len(p.Experiments))
	}
	wantRoutes := []string{"edge-to-data", "data-to-app"}
	for i, x := range p.Experiments {
		if x.RouteID != wantRoutes[i] {
			t.Fatalf("experiment %d has unexpected route %q", i, x.RouteID)
		}
		if x.Target.Namespace != x.Witness.PolicyNamespace || x.Witness.TargetPort != 8080 {
			t.Fatalf("experiment %d has unexpected witness destination", i)
		}
		if len(x.Sequence) != 3 ||
			x.Sequence[0] != (PhaseExpectation{Before, Unreachable}) ||
			x.Sequence[1] != (PhaseExpectation{Witness, Reachable}) ||
			x.Sequence[2] != (PhaseExpectation{Restored, Unreachable}) {
			t.Fatalf("experiment %d must follow unreachable -> reachable -> unreachable", i)
		}
	}
}

func TestTamperingIsRejected(t *testing.T) {
	tests := map[string]func(*Plan){
		"wrong cluster":             func(p *Plan) { p.Cluster = "production" },
		"wrong context":             func(p *Plan) { p.Context = "kind-other" },
		"wrong profile":             func(p *Plan) { p.Profile = "baseline" },
		"unknown schema":            func(p *Plan) { p.Schema = "future" },
		"extra experiment":          func(p *Plan) { p.Experiments = append(p.Experiments, p.Experiments[0]) },
		"missing experiment":        func(p *Plan) { p.Experiments = p.Experiments[:1] },
		"reordered experiment":      func(p *Plan) { p.Experiments[0], p.Experiments[1] = p.Experiments[1], p.Experiments[0] },
		"foreign source":            func(p *Plan) { p.Experiments[0].Source.Namespace = "kube-system" },
		"foreign target":            func(p *Plan) { p.Experiments[0].Target.Namespace = "default" },
		"foreign pod":               func(p *Plan) { p.Experiments[0].Target.AppName = "nginx" },
		"renamed witness":           func(p *Plan) { p.Experiments[0].Witness.PolicyName = "firedoor-app-ingress" },
		"wrong witness namespace":   func(p *Plan) { p.Experiments[0].Witness.PolicyNamespace = "default" },
		"wider source selector":     func(p *Plan) { p.Experiments[0].Witness.SourcePodSelectorKey = "" },
		"wider namespace selector":  func(p *Plan) { p.Experiments[0].Witness.NamespaceSelectorKey = "" },
		"wider target selector":     func(p *Plan) { p.Experiments[0].Witness.TargetPodSelectorKey = "" },
		"all ports":                 func(p *Plan) { p.Experiments[0].Witness.TargetPort = 0 },
		"wrong protocol":            func(p *Plan) { p.Experiments[0].Witness.Protocol = "UDP" },
		"wrong phase sequence":      func(p *Plan) { p.Experiments[0].Sequence[1].Phase = Before },
		"weak baseline expectation": func(p *Plan) { p.Experiments[0].Sequence[0].ExpectedOutcome = Reachable },
		"weak restore expectation":  func(p *Plan) { p.Experiments[0].Sequence[2].ExpectedOutcome = Reachable },
		"unbounded attempts":        func(p *Plan) { p.MaxAttemptsPerPhase = 1000 },
		"unbounded timeout":         func(p *Plan) { p.AttemptTimeoutMS = 60000 },
		"unbounded wait":            func(p *Plan) { p.MaxSettleSeconds = 3600 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			p := ApprovedPlan()
			mutate(&p)
			if err := Validate(p); err == nil {
				t.Fatal("modified plan was accepted")
			}
		})
	}
}

func TestApprovedPlanIsIndependentAndRoundTrips(t *testing.T) {
	first := ApprovedPlan()
	first.Experiments[0].Sequence[0].ExpectedOutcome = Reachable
	if reflect.DeepEqual(first, ApprovedPlan()) {
		t.Fatal("plan mutation leaked into a new plan")
	}
	b, err := JSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Plan
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := Validate(decoded); err != nil {
		t.Fatalf("serialized plan did not validate: %v", err)
	}
}
