package liveverify

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// FD004RecoveryPreflightSchema is a strictly bounded, read-only review of an
// already-hardened owned testbed. A preflight is NOT a recovery test.
const FD004RecoveryPreflightSchema = "firedoor.v10.fd004-recovery-preflight.v1"

// FD004RecoveryManifest contains a minimal independent emergency Kubernetes
// NetworkPolicy definition. Producing it does not apply or authorize it.
type FD004RecoveryManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		PodSelector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"podSelector"`
		PolicyTypes []string `json:"policyTypes"`
		Ingress     []struct {
			From []struct {
				NamespaceSelector struct {
					MatchLabels map[string]string `json:"matchLabels"`
				} `json:"namespaceSelector"`
			} `json:"from"`
			Ports []struct {
				Protocol string `json:"protocol"`
				Port     int    `json:"port"`
			} `json:"ports"`
		} `json:"ingress"`
	} `json:"spec"`
}

type FD004RecoveryPreflight struct {
	SchemaVersion          string                  `json:"schema_version"`
	Scenario               string                  `json:"scenario"`
	Cluster                string                  `json:"cluster"`
	Context                string                  `json:"context"`
	Profile                string                  `json:"profile"`
	CapturedAtUTC          time.Time               `json:"captured_at_utc"`
	StateAddressCount      int                     `json:"state_address_count"`
	TerraformOutputsSHA256 string                  `json:"terraform_outputs_sha256"`
	RecoveryManifests      []FD004RecoveryManifest `json:"recovery_manifests"`
	Status                 string                  `json:"status"`
	ApplyAuthorized        bool                    `json:"apply_authorized"`
	RecoveryTested         bool                    `json:"recovery_tested"`
	TerraformAuthenticated bool                    `json:"terraform_authenticated"`
	PolicyCausalityProven  bool                    `json:"policy_causality_proven"`
	Limitations            string                  `json:"limitations"`
}

// BuildFD004RecoveryPreflight refuses any non-hardened, foreign, drifted or
// incomplete configuration and emits only two fixed owned policy manifests.
// It has no external commands or mutation side effects.
func BuildFD004RecoveryPreflight(c FD004LiveCollection) (FD004RecoveryPreflight, error) {
	var empty FD004RecoveryPreflight
	if err := validateFD004Collection(c, ProfileHardened); err != nil {
		return empty, fmt.Errorf("hardened recovery preflight rejected: %w", err)
	}
	if len(c.KubernetesSnapshot.Zones) != 3 {
		return empty, errors.New("recovery preflight requires exactly three owned zones")
	}
	zoneNames := make([]string, 0, 3)
	for _, zone := range c.KubernetesSnapshot.Zones {
		zoneNames = append(zoneNames, zone.Name)
		if zone.Namespace != "firedoor-"+zone.Name || zone.Deployment.ReadyReplicas != 1 || zone.Deployment.DesiredReplicas != 1 || len(zone.Pods) != 1 || !zone.Pods[0].Ready || zone.Pods[0].Phase != "Running" {
			return empty, fmt.Errorf("recovery preflight workload not ready in zone %q", zone.Name)
		}
		if zone.Pods[0].Labels["app.kubernetes.io/name"] != "firedoor-"+zone.Name || zone.Pods[0].Labels["firedoor.dev/managed-by"] != "terraform" {
			return empty, fmt.Errorf("recovery preflight workload ownership invalid in zone %q", zone.Name)
		}
	}
	sort.Strings(zoneNames)
	if zoneNames[0] != "app" || zoneNames[1] != "data" || zoneNames[2] != "edge" {
		return empty, errors.New("recovery preflight found duplicate or unknown zones")
	}
	manifests := []FD004RecoveryManifest{fd004HardenedManifest("app", "edge"), fd004HardenedManifest("data", "app")}
	return FD004RecoveryPreflight{
		SchemaVersion: FD004RecoveryPreflightSchema, Scenario: "FD-004", Cluster: OwnedCluster, Context: OwnedContext,
		Profile: "hardened", CapturedAtUTC: c.CapturedAtUTC, StateAddressCount: len(c.TerraformStateAddresses),
		TerraformOutputsSHA256: c.TerraformOutputsSHA256, RecoveryManifests: manifests,
		Status: "hardened_preflight_consistent_recovery_not_exercised", ApplyAuthorized: false, RecoveryTested: false,
		TerraformAuthenticated: false, PolicyCausalityProven: false,
		Limitations: "Local hardened configuration is consistent at capture time. Private state backup and exported manifests still need verification by the Windows runner. No recovery command was executed, no automatic rollback exists, and recovery/causality/provenance are NOT proven. Never treat this report as approval to weaken or apply infrastructure.",
	}, nil
}

func fd004HardenedManifest(zone, allowedSource string) FD004RecoveryManifest {
	var m FD004RecoveryManifest
	m.APIVersion = "networking.k8s.io/v1"
	m.Kind = "NetworkPolicy"
	m.Metadata.Name = "firedoor-" + zone + "-ingress"
	m.Metadata.Namespace = "firedoor-" + zone
	m.Metadata.Labels = map[string]string{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}
	m.Spec.PodSelector.MatchLabels = map[string]string{"app.kubernetes.io/name": "firedoor-" + zone}
	m.Spec.PolicyTypes = []string{"Ingress"}
	ingress := struct {
		From []struct {
			NamespaceSelector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"namespaceSelector"`
		} `json:"from"`
		Ports []struct {
			Protocol string `json:"protocol"`
			Port     int    `json:"port"`
		} `json:"ports"`
	}{}
	// Initialize the typed nested structs without relying on user-controlled strings.
	ingress.From = make([]struct {
		NamespaceSelector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"namespaceSelector"`
	}, 1)
	ingress.From[0].NamespaceSelector.MatchLabels = map[string]string{"firedoor.dev/zone": allowedSource}
	ingress.Ports = make([]struct {
		Protocol string `json:"protocol"`
		Port     int    `json:"port"`
	}, 1)
	ingress.Ports[0].Protocol = "TCP"
	ingress.Ports[0].Port = 8080
	m.Spec.Ingress = append(m.Spec.Ingress, ingress)
	return m
}
