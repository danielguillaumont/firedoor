package validation

import (
	"testing"

	"github.com/danielguillaumont/firedoor/pkg/models"
)

func TestNetworkObservationClassification(t *testing.T) {
	ready := NetworkObservation{Attempted: true, ProbeCompleted: true, DestinationHealthy: true, DNSHealthy: true, CNIEnforcementVerified: true, AllowedControlSucceeded: true}
	cases := []struct {
		name        string
		observation NetworkObservation
		want        models.Status
	}{
		{"untested", NetworkObservation{}, models.NotTested},
		{"crossed", withOutcome(ready, true, false), models.Crossed},
		{"contained", withOutcome(ready, false, true), models.Contained},
		{"contradictory", withOutcome(ready, true, true), models.Inconclusive},
		{"no observed result", ready, models.Inconclusive},
		{"no healthy destination", withOutcome(NetworkObservation{Attempted: true, ProbeCompleted: true, DNSHealthy: true, CNIEnforcementVerified: true, AllowedControlSucceeded: true}, false, true), models.Inconclusive},
		{"no allowed control", withOutcome(NetworkObservation{Attempted: true, ProbeCompleted: true, DestinationHealthy: true, DNSHealthy: true, CNIEnforcementVerified: true}, false, true), models.Inconclusive},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyNetworkObservation(tt.observation); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func withOutcome(o NetworkObservation, reached, denied bool) NetworkObservation {
	o.RestrictedPathReached = reached
	o.RestrictedPathDenied = denied
	return o
}
