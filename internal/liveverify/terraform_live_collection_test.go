package liveverify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

type fd004TerraformFake struct {
	state  []byte
	output []byte
	fail   bool
	calls  []string
}

func (f *fd004TerraformFake) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	command := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, command)
	if f.fail {
		return nil, errors.New("offline simulated CLI failure")
	}
	if command == "terraform -chdir="+FD004TerraformRoot+" state list" {
		return f.state, nil
	}
	if command == "terraform -chdir="+FD004TerraformRoot+" output -json" {
		return f.output, nil
	}
	return nil, fmt.Errorf("unapproved test command %s", command)
}

type fd004KubernetesFake struct {
	snapshot kubeinspect.Snapshot
	err      error
}

func (f fd004KubernetesFake) Collect(_ context.Context) (kubeinspect.Snapshot, error) {
	return f.snapshot, f.err
}
func fd004GoodCollector(t *testing.T, profile Profile) (FD004LiveCollector, *fd004TerraformFake) {
	t.Helper()
	now := time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)
	state := []byte(strings.Join(fd004ExpectedStateAddresses(), "\r\n") + "\r\n")
	runner := &fd004TerraformFake{state: state, output: fixtureFD004Terraform(t, profile)}
	collector := FD004LiveCollector{Terraform: runner, Kubernetes: fd004KubernetesFake{snapshot: fixtureFD004Snapshot(profile, now)}, Now: func() time.Time { return now }}
	return collector, runner
}
func TestFD004LiveCollectionReadOnlyBaselineAndHardened(t *testing.T) {
	for _, profile := range []Profile{ProfileBaseline, ProfileHardened} {
		t.Run(string(profile), func(t *testing.T) {
			c, runner := fd004GoodCollector(t, profile)
			report, err := c.Collect(context.Background(), profile)
			if err != nil {
				t.Fatal(err)
			}
			if report.Status != "configuration_consistent_not_behaviorally_verified" || !report.StateAddressInventoryChecked || report.TerraformAuthenticated || report.TrafficVerified || report.PolicyStatus != "consistent" || len(report.TerraformStateAddresses) != 14 || len(report.TerraformOutputsSHA256) != 64 || report.SchemaVersion != FD004LiveCollectionSchema {
				t.Fatalf("unsupported live security claim or invalid state: %+v", report)
			}
			if len(runner.calls) != 2 || !strings.Contains(runner.calls[0], "state list") || !strings.Contains(runner.calls[1], "output -json") {
				t.Fatalf("unexpected commands: %v", runner.calls)
			}
		})
	}
}
func TestFD004LiveCollectionRefusesUnownedOrMissingState(t *testing.T) {
	cases := []struct {
		name   string
		change func(*FD004LiveCollector, *fd004TerraformFake)
	}{
		{"foreign address", func(_ *FD004LiveCollector, r *fd004TerraformFake) {
			r.state = append(r.state, []byte("kubernetes_secret.prod\n")...)
		}},
		{"missing address", func(_ *FD004LiveCollector, r *fd004TerraformFake) {
			r.state = []byte(strings.Join(fd004ExpectedStateAddresses()[:13], "\n"))
		}},
		{"duplicate address", func(_ *FD004LiveCollector, r *fd004TerraformFake) {
			r.state = append(r.state, []byte(fd004ExpectedStateAddresses()[0]+"\n")...)
		}},
		{"empty state", func(_ *FD004LiveCollector, r *fd004TerraformFake) { r.state = nil }},
		{"oversized state", func(_ *FD004LiveCollector, r *fd004TerraformFake) {
			r.state = []byte(strings.Repeat("a", maxFD004StateList+1))
		}},
		{"profile mismatch", func(_ *FD004LiveCollector, r *fd004TerraformFake) {
			r.output = []byte(`{"active_profile":{"value":"baseline"}}`)
		}},
		{"unavailable Terraform", func(_ *FD004LiveCollector, r *fd004TerraformFake) { r.fail = true }},
		{"wrong cluster", func(c *FD004LiveCollector, _ *fd004TerraformFake) {
			s := c.Kubernetes.(fd004KubernetesFake).snapshot
			s.Cluster = "production"
			c.Kubernetes = fd004KubernetesFake{snapshot: s}
		}},
		{"stale snapshot", func(c *FD004LiveCollector, _ *fd004TerraformFake) {
			s := c.Kubernetes.(fd004KubernetesFake).snapshot
			s.CapturedAtUTC = s.CapturedAtUTC.Add(-time.Hour)
			c.Kubernetes = fd004KubernetesFake{snapshot: s}
		}},
		{"unavailable Kubernetes", func(c *FD004LiveCollector, _ *fd004TerraformFake) {
			c.Kubernetes = fd004KubernetesFake{err: errors.New("denied")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, r := fd004GoodCollector(t, ProfileHardened)
			tc.change(&c, r)
			if _, err := c.Collect(context.Background(), ProfileHardened); err == nil {
				t.Fatal("unsafe or incomplete live collection accepted")
			}
		})
	}
}
func TestFD004LiveCollectionDriftIsNotHealthy(t *testing.T) {
	c, _ := fd004GoodCollector(t, ProfileHardened)
	s := c.Kubernetes.(fd004KubernetesFake).snapshot
	s.Zones[2].NetworkPolicies = nil
	c.Kubernetes = fd004KubernetesFake{snapshot: s}
	report, err := c.Collect(context.Background(), ProfileHardened)
	if err != nil || report.Status != "configuration_drift" || report.PolicyStatus != "drift" || report.TrafficVerified || report.TerraformAuthenticated {
		t.Fatalf("drift falsely healthy: %+v %v", report, err)
	}
}
func TestFD004LocalCommandAllowlist(t *testing.T) {
	runner := FD004LocalCommands{}
	for _, args := range [][]string{
		{"-chdir=" + FD004TerraformRoot, "apply", "-auto-approve"},
		{"-chdir=" + FD004TerraformRoot, "destroy", "-auto-approve"},
		{"-chdir=" + FD004TerraformRoot, "state", "rm", "secret"},
		{"-chdir=production", "output", "-json"},
		{"-chdir=" + FD004TerraformRoot, "plan", "-out=x"},
		{"-chdir=" + FD004TerraformRoot, "output", "-raw", "secret"},
	} {
		if _, err := runner.Run(context.Background(), "terraform", args...); err == nil {
			t.Fatalf("unsafe command allowed: %v", args)
		}
	}
	if _, err := runner.Run(context.Background(), "kubectl", "get", "secret"); err == nil {
		t.Fatal("kubectl command accepted")
	}
}
