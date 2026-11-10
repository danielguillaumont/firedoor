package liveverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

type fd004ProofFakeCollector struct {
	values []FD004LiveCollection
	calls  int
	err    error
}

func (f *fd004ProofFakeCollector) Collect(_ context.Context, _ Profile) (FD004LiveCollection, error) {
	f.calls++
	if f.err != nil {
		return FD004LiveCollection{}, f.err
	}
	if f.calls > len(f.values) {
		return FD004LiveCollection{}, errors.New("too many collections")
	}
	return f.values[f.calls-1], nil
}

type fd004ProofFakeTraffic struct {
	suite SuiteObservation
	calls int
	err   error
}

func (f *fd004ProofFakeTraffic) Run(_ context.Context, _ Plan) (SuiteObservation, error) {
	f.calls++
	if f.err != nil {
		return SuiteObservation{}, f.err
	}
	return f.suite, nil
}

func fd004ProofFixture(t *testing.T) (*fd004ProofFakeCollector, *fd004ProofFakeTraffic) {
	t.Helper()
	plan, suite := verifiedFixture(t)
	create := func(at time.Time) FD004LiveCollection {
		outputs := fixtureFD004Terraform(t, plan.Profile)
		sum := sha256.Sum256(outputs)
		snap := fixtureFD004Snapshot(plan.Profile, at.Add(-time.Second))
		for i := range snap.Zones {
			z := &snap.Zones[i]
			p, err := suite.Inventory.pod(Zone(z.Name))
			if err != nil {
				t.Fatal(err)
			}
			z.Pods = []kubeinspect.Pod{{Name: p.PodName, Phase: "Running", Ready: true, Labels: map[string]string{
				"firedoor.dev/managed-by": "terraform", "firedoor.dev/zone": z.Name,
				"app.kubernetes.io/part-of": "firedoor", "app.kubernetes.io/name": "firedoor-" + z.Name,
			}}}
		}
		return FD004LiveCollection{
			SchemaVersion: FD004LiveCollectionSchema, Scenario: "FD-004",
			Cluster: OwnedCluster, Context: OwnedContext, Profile: plan.Profile,
			CapturedAtUTC: at, TerraformRoot: FD004TerraformRoot,
			TerraformStateAddresses: fd004ExpectedStateAddresses(),
			TerraformOutputsSHA256:  hex.EncodeToString(sum[:]), TerraformOutputs: json.RawMessage(outputs),
			KubernetesSnapshot: snap, PolicyStatus: "consistent", StateAddressInventoryChecked: true,
			TerraformAuthenticated: false, TrafficVerified: false,
			Status: "configuration_consistent_not_behaviorally_verified", Limitations: "read-only fixture",
		}
	}
	return &fd004ProofFakeCollector{values: []FD004LiveCollection{
		create(suite.StartedAtUTC.Add(-10 * time.Second)), create(suite.FinishedAtUTC.Add(10 * time.Second)),
	}}, &fd004ProofFakeTraffic{suite: suite}
}

func TestFD004LiveProofMatchingHardenedRoutes(t *testing.T) {
	c, traffic := fd004ProofFixture(t)
	report, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "observed_behavior_matched_cause_unproven" || !report.TrafficVerified ||
		report.TerraformAuthenticated || report.PolicyCausalityProven ||
		report.Bridge.RoutesMatched != 6 || c.calls != 2 || traffic.calls != 1 ||
		!strings.Contains(report.Limitations, "does not independently prove") {
		t.Fatalf("unsupported proof result: %+v", report)
	}
	payload, err := json.Marshal(report)
	if err != nil || !strings.Contains(string(payload), `"policy_causality_proven":false`) {
		t.Fatal("missing honest provenance in proof JSON")
	}
}

func TestFD004LiveProofRejectsUntrustedInputBeforeAnyProbe(t *testing.T) {
	tests := map[string]func(*fd004ProofFakeCollector){
		"policy drift": func(c *fd004ProofFakeCollector) {
			c.values[0].PolicyStatus = "drift"
			c.values[0].Status = "configuration_drift"
		},
		"foreign cluster": func(c *fd004ProofFakeCollector) { c.values[0].Cluster = "production" },
		"wrong profile":   func(c *fd004ProofFakeCollector) { c.values[0].Profile = ProfileBaseline },
		"missing address": func(c *fd004ProofFakeCollector) {
			c.values[0].TerraformStateAddresses = c.values[0].TerraformStateAddresses[:13]
		},
		"tampered outputs":        func(c *fd004ProofFakeCollector) { c.values[0].TerraformOutputs = []byte(`{}`) },
		"unsupported state claim": func(c *fd004ProofFakeCollector) { c.values[0].TerraformAuthenticated = true },
		"stale pre-collection": func(c *fd004ProofFakeCollector) {
			c.values[0].KubernetesSnapshot.CapturedAtUTC = c.values[0].CapturedAtUTC.Add(-5 * time.Minute)
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c, traffic := fd004ProofFixture(t)
			change(c)
			if _, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), ProfileHardened); err == nil {
				t.Fatal("unsafe pre-probe collection accepted")
			}
			if traffic.calls != 0 || c.calls != 1 {
				t.Fatalf("probed before trustworthy configuration: calls %d %d", c.calls, traffic.calls)
			}
		})
	}
}

func TestFD004LiveProofRejectsChangedEnvironmentAfterProbe(t *testing.T) {
	tests := map[string]func(*fd004ProofFakeCollector, *fd004ProofFakeTraffic){
		"post-policy drift": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[1].KubernetesSnapshot.Zones[2].NetworkPolicies = nil
		},
		"terraform changed": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[1].TerraformOutputs = []byte(`{}`)
		},
		"profile changed": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[1].Profile = ProfileBaseline
		},
		"Pod replaced": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[1].KubernetesSnapshot.Zones[0].Pods[0].Name = "firedoor-edge-altered-xyz12"
		},
		"stale before": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[0].CapturedAtUTC = c.values[0].CapturedAtUTC.Add(-3 * time.Minute)
			c.values[0].KubernetesSnapshot.CapturedAtUTC = c.values[0].KubernetesSnapshot.CapturedAtUTC.Add(-3 * time.Minute)
		},
		"stale after": func(c *fd004ProofFakeCollector, _ *fd004ProofFakeTraffic) {
			c.values[1].CapturedAtUTC = c.values[1].CapturedAtUTC.Add(3 * time.Minute)
			c.values[1].KubernetesSnapshot.CapturedAtUTC = c.values[1].KubernetesSnapshot.CapturedAtUTC.Add(3 * time.Minute)
		},
		"foreign traffic inventory": func(_ *fd004ProofFakeCollector, p *fd004ProofFakeTraffic) {
			p.suite.Inventory.Pods[0].PodName = "unknown-pod"
		},
		"forged route marker": func(_ *fd004ProofFakeCollector, p *fd004ProofFakeTraffic) {
			p.suite.Routes[0].Attempts[0].Reason = "untrusted"
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c, traffic := fd004ProofFixture(t)
			change(c, traffic)
			if _, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), ProfileHardened); err == nil {
				t.Fatal("stale or altered environment accepted")
			}
		})
	}
}

func TestFD004LiveProofMismatchNeverBecomesSuccess(t *testing.T) {
	c, traffic := fd004ProofFixture(t)
	r := &traffic.suite.Routes[2]
	r.Observed = ObservedReachable
	r.Reason = "HTTP reachable with exact destination marker"
	r.Attempts = r.Attempts[:1]
	r.Attempts[0].Observation = ObservedReachable
	r.Attempts[0].Reason = "destination returned the exact expected HTTP body"
	result, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), ProfileHardened)
	if err != nil || result.Status != "behavior_mismatch" || result.TrafficVerified || result.Bridge.RoutesMismatch != 1 {
		t.Fatalf("mismatch not preserved: %+v %v", result, err)
	}
}

func TestFD004LiveProofInconclusiveNeverBecomesSuccess(t *testing.T) {
	c, traffic := fd004ProofFixture(t)
	p, err := NewPlan(ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := verificationFakeExecutor(&labFake{execFails: true}).Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	traffic.suite = obs
	result, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), ProfileHardened)
	if err != nil || result.Status != "behavior_inconclusive" || result.TrafficVerified || result.Bridge.RoutesInconclusive != 6 {
		t.Fatalf("inconclusive became success: %+v %v", result, err)
	}
}

func TestFD004LiveProofRefusesMissingAdapters(t *testing.T) {
	c, traffic := fd004ProofFixture(t)
	if _, err := (FD004LiveProver{Configuration: c}).Run(context.Background(), ProfileHardened); err == nil {
		t.Fatal("missing traffic adapter accepted")
	}
	if _, err := (FD004LiveProver{Traffic: traffic}).Run(context.Background(), ProfileHardened); err == nil {
		t.Fatal("missing configuration adapter accepted")
	}
	if _, err := (FD004LiveProver{Configuration: c, Traffic: traffic}).Run(context.Background(), Profile("production")); err == nil {
		t.Fatal("foreign profile accepted")
	}
}
