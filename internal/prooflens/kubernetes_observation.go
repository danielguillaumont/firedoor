package prooflens

// Slice 3A uses the existing v0.4 pinned owned-lab preflight as its only
// Kubernetes object reader. A point-in-time observation is not a traffic test.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/danielguillaumont/firedoor/internal/attribution"
)

const (
	KubernetesObservationSchema = "firedoor.v05.kubernetes-observation.v1"
	KubernetesObservationMode   = "OWNED_LAB_KUBERNETES_READ_ONLY"
	KubernetesUnlinkedStatus    = "UNLINKED_SOURCE_DIGEST_ONLY"
)

type ObservedNamespace struct {
	Name                   string `json:"name"`
	PodReady               bool   `json:"pod_ready"`
	PolicyCount            int    `json:"policy_count"`
	ExpectedPolicy         string `json:"expected_policy,omitempty"`
	AllowedSourceNamespace string `json:"allowed_source_namespace,omitempty"`
}

type KubernetesObservation struct {
	SchemaVersion      string              `json:"schema_version"`
	Mode               string              `json:"mode"`
	CapturedUTC        string              `json:"captured_utc"`
	Cluster            string              `json:"cluster"`
	Context            string              `json:"context"`
	Profile            string              `json:"profile"`
	SourceTreeSHA256   string              `json:"source_tree_sha256"`
	NamespaceCount     int                 `json:"namespace_count"`
	ReadyPodCount      int                 `json:"ready_pod_count"`
	PolicyCount        int                 `json:"policy_count"`
	Namespaces         []ObservedNamespace `json:"namespaces"`
	ReceiptCorrelation string              `json:"receipt_correlation"`
	ProofVerdict       string              `json:"proof_verdict"`
	Limitations        []string            `json:"limitations"`
	ObservationSHA256  string              `json:"observation_sha256"`
}

func observationLimitations() []string {
	return []string{
		"Read-only point-in-time Kubernetes inventory, not an A/B/A traffic experiment or proof of packet filtering.",
		"The source-tree SHA256 is a local consistency reference; no stored Terraform receipt has been independently joined.",
		"No Kubernetes objects, raw Terraform state, provider logs, or pod IP addresses are saved in this observation.",
		"Observation SHA256 is not a trusted signature or authentication of the cluster's identity.",
	}
}

func expectedNamespaces() []ObservedNamespace {
	return []ObservedNamespace{
		{Name: "firedoor-edge", PodReady: true, PolicyCount: 0},
		{Name: "firedoor-app", PodReady: true, PolicyCount: 1, ExpectedPolicy: "firedoor-app-ingress", AllowedSourceNamespace: "firedoor-edge"},
		{Name: "firedoor-data", PodReady: true, PolicyCount: 1, ExpectedPolicy: "firedoor-data-ingress", AllowedSourceNamespace: "firedoor-app"},
	}
}

func observationDigest(o KubernetesObservation) (string, error) {
	o.ObservationSHA256 = ""
	data, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// BuildKubernetesObservation rejects even an internally generated preflight if
// it does not match the fixed hardened Firedoor three-namespace baseline.
func BuildKubernetesObservation(preflight attribution.PreflightReport, source TerraformSourceManifest) (KubernetesObservation, error) {
	var empty KubernetesObservation
	if err := VerifyTerraformSourceManifest(source); err != nil {
		return empty, fmt.Errorf("Terraform source manifest rejected: %w", err)
	}
	if preflight.Schema != "firedoor.v04.policy-preflight.v1" ||
		preflight.Cluster != attribution.OwnedCluster ||
		preflight.Context != attribution.OwnedContext ||
		preflight.Profile != attribution.RequiredProfile ||
		preflight.Status != "admissible_for_later_review" ||
		preflight.CapturedAtUTC.IsZero() ||
		preflight.CapturedAtUTC.Location() != time.UTC ||
		len(preflight.Namespaces) != 3 {
		return empty, errors.New("unapproved Kubernetes preflight identity or state")
	}
	plan := attribution.ApprovedPlan()
	if !reflect.DeepEqual(preflight.PlannedExperiments, []string{plan.Experiments[0].ID, plan.Experiments[1].ID}) {
		return empty, errors.New("unexpected experiment IDs in preflight")
	}
	observed := make([]ObservedNamespace, 0, 3)
	for _, n := range preflight.Namespaces {
		observed = append(observed, ObservedNamespace{
			Name: n.Name, PodReady: n.PodReady, PolicyCount: n.PolicyCount,
			ExpectedPolicy: n.ExpectedPolicy, AllowedSourceNamespace: n.SourceNamespace,
		})
	}
	if !reflect.DeepEqual(observed, expectedNamespaces()) {
		return empty, errors.New("Kubernetes namespace/pod/policy snapshot differs from hardened baseline")
	}
	o := KubernetesObservation{
		SchemaVersion: KubernetesObservationSchema, Mode: KubernetesObservationMode,
		CapturedUTC: preflight.CapturedAtUTC.Format(time.RFC3339Nano),
		Cluster:     attribution.OwnedCluster, Context: attribution.OwnedContext,
		Profile:          attribution.RequiredProfile,
		SourceTreeSHA256: source.TreeSHA256,
		NamespaceCount:   3, ReadyPodCount: 3, PolicyCount: 2,
		Namespaces:         observed,
		ReceiptCorrelation: KubernetesUnlinkedStatus,
		ProofVerdict:       Inconclusive, Limitations: observationLimitations(),
	}
	digest, err := observationDigest(o)
	if err != nil {
		return empty, err
	}
	o.ObservationSHA256 = digest
	if err := VerifyKubernetesObservation(o); err != nil {
		return empty, err
	}
	return o, nil
}

// VerifyKubernetesObservation validates the sanitized record's internal shape
// and checksum; it does not authenticate Kubernetes or Terraform provenance.
func VerifyKubernetesObservation(o KubernetesObservation) error {
	if o.SchemaVersion != KubernetesObservationSchema || o.Mode != KubernetesObservationMode ||
		!validUTC(o.CapturedUTC) ||
		o.Cluster != attribution.OwnedCluster || o.Context != attribution.OwnedContext ||
		o.Profile != attribution.RequiredProfile ||
		!validHex(o.SourceTreeSHA256, 64) ||
		o.NamespaceCount != 3 || o.ReadyPodCount != 3 || o.PolicyCount != 2 ||
		!reflect.DeepEqual(o.Namespaces, expectedNamespaces()) ||
		o.ReceiptCorrelation != KubernetesUnlinkedStatus || o.ProofVerdict != Inconclusive ||
		!reflect.DeepEqual(o.Limitations, observationLimitations()) ||
		!validHex(o.ObservationSHA256, 64) {
		return errors.New("Kubernetes observation invalid or unsubstantiated")
	}
	digest, err := observationDigest(o)
	if err != nil || digest != o.ObservationSHA256 {
		return errors.New("Kubernetes observation checksum mismatch")
	}
	return nil
}
