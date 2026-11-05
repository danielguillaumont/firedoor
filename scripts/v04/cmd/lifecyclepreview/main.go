// An explicitly simulated A/B/A lifecycle demonstration. NO KUBERNETES CALLS.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielguillaumont/firedoor/internal/attribution"
	"os"
)

type simulatedDriver struct{ present bool }

func (s *simulatedDriver) Preflight(context.Context) error {
	if s.present {
		return fmt.Errorf("simulated witness not absent")
	}
	return nil
}
func (s *simulatedDriver) Probe(_ context.Context, _ attribution.Experiment, phase attribution.Phase) (attribution.PhaseObservation, error) {
	o := attribution.PhaseObservation{Phase: phase, Observed: attribution.Unreachable, AttemptCount: 2, Reason: "SIMULATION ONLY: two synthetic unsuccessful HTTP attempts"}
	if phase == attribution.Witness {
		o.Observed = attribution.Reachable
		o.AttemptCount = 1
		o.Reason = "SIMULATION ONLY: synthetic successful HTTP marker"
	}
	return o, nil
}
func (s *simulatedDriver) Create(_ context.Context, _ attribution.Experiment, _ attribution.WitnessObject) (string, error) {
	s.present = true
	return "simulated-uid", nil
}
func (s *simulatedDriver) Inspect(_ context.Context, _ attribution.Experiment, uid string) error {
	if !s.present || uid != "simulated-uid" {
		return fmt.Errorf("fake UID mismatch")
	}
	return nil
}
func (s *simulatedDriver) Delete(_ context.Context, _ attribution.Experiment, uid string) error {
	if uid != "simulated-uid" {
		return fmt.Errorf("fake UID mismatch")
	}
	s.present = false
	return nil
}
func (s *simulatedDriver) VerifyGone(_ context.Context, _ attribution.Experiment) error {
	if s.present {
		return fmt.Errorf("simulated witness not removed")
	}
	return nil
}
func main() {
	r, err := (attribution.Lifecycle{Driver: &simulatedDriver{}}).RunAll(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := struct {
		Mode          string                        `json:"mode"`
		Cluster       string                        `json:"cluster"`
		LiveMutations bool                          `json:"live_mutations"`
		Reports       []attribution.LifecycleReport `json:"reports"`
	}{
		Mode: "SIMULATED_ONLY", Cluster: attribution.OwnedCluster, LiveMutations: false, Reports: r,
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
