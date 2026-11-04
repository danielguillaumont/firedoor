// Package validation contains pure evaluation primitives; v0.0 does not run probes.
package validation

import "github.com/danielguillaumont/firedoor/pkg/models"

// NetworkObservation is intentionally explicit: denied connectivity alone never proves containment.
// Actual scenario execution / attribution remains a v0.3 milestone.
type NetworkObservation struct {
	Attempted               bool
	ProbeCompleted          bool
	DestinationHealthy      bool
	DNSHealthy              bool
	CNIEnforcementVerified  bool
	AllowedControlSucceeded bool
	RestrictedPathReached   bool
	RestrictedPathDenied    bool
}

func ClassifyNetworkObservation(o NetworkObservation) models.Status {
	if !o.Attempted {
		return models.NotTested
	}
	if !o.ProbeCompleted || !o.DestinationHealthy || !o.DNSHealthy || !o.CNIEnforcementVerified || !o.AllowedControlSucceeded {
		return models.Inconclusive
	}
	if o.RestrictedPathReached && !o.RestrictedPathDenied {
		return models.Crossed
	}
	if o.RestrictedPathDenied && !o.RestrictedPathReached {
		return models.Contained
	}
	return models.Inconclusive
}
