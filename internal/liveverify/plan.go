// Package liveverify defines the bounded, fixed-scope network verification
// contract for Firedoor's owned local Kubernetes testbed.
//
// Slice 1 is pure Go: it does not read a kubeconfig, execute kubectl, create
// Kubernetes resources, or perform network requests.
package liveverify

import "fmt"

const (
	SchemaVersion = "firedoor.v03.probe-plan.v1"
	OwnedCluster  = "firedoor-local"
	OwnedContext  = "kind-firedoor-local"

	// Direct Pod IPs intentionally bypass Service DNS and Service translation.
	// Pod IPs are discovered and ownership-checked at execution time, not
	// supplied through this plan.
	EndpointModeDirectPodIP = "direct-pod-ip"
	TransportHTTPOverTCP    = "HTTP/TCP"
	TargetPort              = 8080
)

type Zone string

const (
	ZoneEdge Zone = "edge"
	ZoneApp  Zone = "app"
	ZoneData Zone = "data"
)

type Profile string

const (
	ProfileBaseline Profile = "baseline"
	ProfileHardened Profile = "hardened"
)

type Expectation string

const (
	ExpectReachable   Expectation = "reachable"
	ExpectUnreachable Expectation = "unreachable"
)

// Probe identifies a fixed source-to-destination route. It deliberately has
// no user-provided address, namespace, shell command, or executable field.
type Probe struct {
	ID          string      `json:"id"`
	Source      Zone        `json:"source"`
	Destination Zone        `json:"destination"`
	Expected    Expectation `json:"expected"`
}

// Limits bound the future execution engine. The limits are intentionally fixed
// and must not be relaxed through user input in v0.3.
type Limits struct {
	AttemptTimeoutSeconds       int `json:"attempt_timeout_seconds"`
	RetryDelaySeconds           int `json:"retry_delay_seconds"`
	MaxAttemptsReachable        int `json:"max_attempts_reachable"`
	MaxAttemptsUnreachable      int `json:"max_attempts_unreachable"`
	RequiredConsecutiveFailures int `json:"required_consecutive_failures"`
	MaxSuiteDurationSeconds     int `json:"max_suite_duration_seconds"`
}

func DefaultLimits() Limits {
	return Limits{
		AttemptTimeoutSeconds:       3,
		RetryDelaySeconds:           2,
		MaxAttemptsReachable:        2,
		MaxAttemptsUnreachable:      8,
		RequiredConsecutiveFailures: 2,
		MaxSuiteDurationSeconds:     150,
	}
}

// Plan is an expected-behavior declaration, not evidence that the behavior
// occurred, and not proof of the underlying reason for a connection failure.
type Plan struct {
	SchemaVersion string  `json:"schema_version"`
	Cluster       string  `json:"cluster"`
	Context       string  `json:"context"`
	Profile       Profile `json:"profile"`
	EndpointMode  string  `json:"endpoint_mode"`
	Transport     string  `json:"transport"`
	Port          int     `json:"port"`
	Limits        Limits  `json:"limits"`
	Probes        []Probe `json:"probes"`
}

type route struct {
	id          string
	source      Zone
	destination Zone
	hardened    Expectation
}

// These six routes and hardened expectations reproduce the reviewed v0.1
// Terraform connectivity matrix. The baseline expectation is reachable for
// every route. Runtime observation will be implemented in later slices.
var routes = [...]route{
	{"edge-to-app", ZoneEdge, ZoneApp, ExpectReachable},
	{"app-to-data", ZoneApp, ZoneData, ExpectReachable},
	{"edge-to-data", ZoneEdge, ZoneData, ExpectUnreachable},
	{"data-to-app", ZoneData, ZoneApp, ExpectUnreachable},
	{"app-to-edge", ZoneApp, ZoneEdge, ExpectReachable},
	{"data-to-edge", ZoneData, ZoneEdge, ExpectReachable},
}

// NewPlan emits only the six allowed pod-to-pod HTTP/TCP routes.
func NewPlan(profile Profile) (Plan, error) {
	if profile != ProfileBaseline && profile != ProfileHardened {
		return Plan{}, fmt.Errorf("unsupported profile %q", profile)
	}
	p := Plan{
		SchemaVersion: SchemaVersion,
		Cluster:       OwnedCluster,
		Context:       OwnedContext,
		Profile:       profile,
		EndpointMode:  EndpointModeDirectPodIP,
		Transport:     TransportHTTPOverTCP,
		Port:          TargetPort,
		Limits:        DefaultLimits(),
		Probes:        make([]Probe, 0, len(routes)),
	}
	for _, r := range routes {
		expected := ExpectReachable
		if profile == ProfileHardened {
			expected = r.hardened
		}
		p.Probes = append(p.Probes, Probe{ID: r.id, Source: r.source, Destination: r.destination, Expected: expected})
	}
	if err := p.Validate(); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// Validate refuses a different cluster, unexpected path, modified expectations,
// altered execution limits, or a partial/repeated matrix. Any future live
// executor must call this before contacting Kubernetes and must independently
// establish observed Pod ownership and the actual Terraform profile.
func (p Plan) Validate() error {
	if p.SchemaVersion != SchemaVersion || p.Cluster != OwnedCluster || p.Context != OwnedContext {
		return fmt.Errorf("plan is not scoped to the expected Firedoor schema, cluster, and context")
	}
	if p.Profile != ProfileBaseline && p.Profile != ProfileHardened {
		return fmt.Errorf("unsupported profile %q", p.Profile)
	}
	if p.EndpointMode != EndpointModeDirectPodIP || p.Transport != TransportHTTPOverTCP || p.Port != TargetPort {
		return fmt.Errorf("unsupported endpoint mode, transport, or port")
	}
	if p.Limits != DefaultLimits() {
		return fmt.Errorf("probe execution limits were modified")
	}
	if len(p.Probes) != len(routes) {
		return fmt.Errorf("expected %d fixed probes; got %d", len(routes), len(p.Probes))
	}
	allowed := make(map[string]Probe, len(routes))
	for _, r := range routes {
		expectation := ExpectReachable
		if p.Profile == ProfileHardened {
			expectation = r.hardened
		}
		allowed[r.id] = Probe{ID: r.id, Source: r.source, Destination: r.destination, Expected: expectation}
	}
	seen := make(map[string]bool, len(routes))
	for _, probe := range p.Probes {
		expected, exists := allowed[probe.ID]
		if !exists || seen[probe.ID] || probe != expected {
			return fmt.Errorf("unknown, duplicate, or altered probe %q", probe.ID)
		}
		seen[probe.ID] = true
	}
	for id := range allowed {
		if !seen[id] {
			return fmt.Errorf("required probe %q is missing", id)
		}
	}
	// At most 24 attempts in the hardened profile and 12 in baseline.
	// Reserve time for attempt timeouts and inter-attempt delays; the actual
	// executor must additionally apply a wall-clock context deadline.
	maxAttempts := 0
	for _, probe := range p.Probes {
		if probe.Expected == ExpectReachable {
			maxAttempts += p.Limits.MaxAttemptsReachable
		} else {
			maxAttempts += p.Limits.MaxAttemptsUnreachable
		}
	}
	upperBoundSeconds := maxAttempts * (p.Limits.AttemptTimeoutSeconds + p.Limits.RetryDelaySeconds)
	if upperBoundSeconds > p.Limits.MaxSuiteDurationSeconds {
		return fmt.Errorf("probe timeout budget exceeds maximum suite duration")
	}
	return nil
}

// ExpectedHTTPBody is the exact readiness marker returned by the owned v0.1
// demo-http workloads. A successful HTTP status without this body is not proof
// of reaching the intended destination.
func ExpectedHTTPBody(destination Zone) (string, error) {
	switch destination {
	case ZoneEdge:
		return "FIREDOOR_EDGE_OK", nil
	case ZoneApp:
		return "FIREDOOR_APP_OK", nil
	case ZoneData:
		return "FIREDOOR_DATA_OK", nil
	default:
		return "", fmt.Errorf("unowned destination zone %q", destination)
	}
}
