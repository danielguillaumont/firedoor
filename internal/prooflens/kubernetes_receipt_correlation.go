package prooflens

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"time"
)

const (
	ReceiptCorrelationSchema = "firedoor.v05.receipt-observation-correlation.v1"
	ReceiptCorrelationMode   = "OWNED_LAB_READ_ONLY_RECEIPT_CORRELATION"
	// Digest equality links two local records. It does not establish Terraform
	// apply, resource causality, packet filtering, or a trusted attestation.
	ReceiptCorrelationStatus = "MATCHING_SOURCE_DIGEST_NOT_CAUSAL_PROOF"
)

type ReceiptObservationCorrelation struct {
	SchemaVersion      string                   `json:"schema_version"`
	Mode               string                   `json:"mode"`
	ReceiptID          string                   `json:"receipt_id"`
	ReceiptSHA256      string                   `json:"receipt_sha256"`
	PlanCapturedUTC    string                   `json:"plan_captured_utc"`
	PlanChanges        TerraformUIChangeSummary `json:"plan_changes"`
	ObservationSHA256  string                   `json:"observation_sha256"`
	ObservationUTC     string                   `json:"observation_captured_utc"`
	SourceTreeSHA256   string                   `json:"source_tree_sha256"`
	Cluster            string                   `json:"cluster"`
	Context            string                   `json:"context"`
	Profile            string                   `json:"profile"`
	NamespaceCount     int                      `json:"namespace_count"`
	ReadyPodCount      int                      `json:"ready_pod_count"`
	PolicyCount        int                      `json:"policy_count"`
	ReceiptCorrelation string                   `json:"receipt_correlation"`
	PlanApplied        bool                     `json:"plan_applied"`
	NetworkProbesRun   bool                     `json:"network_probes_run"`
	ProofVerdict       string                   `json:"proof_verdict"`
	Limitations        []string                 `json:"limitations"`
	CorrelationSHA256  string                   `json:"correlation_sha256"`
}

func correlationLimitations() []string {
	return []string{
		"Only an existing local Terraform speculative-plan receipt and a fresh read-only Kubernetes inventory are joined.",
		"Equal source-tree SHA256 values and plan counts do not establish Terraform apply or causation of Kubernetes objects.",
		"No live traffic probes, network-policy mutations, or v0.4 A/B/A witness replay occurs in this slice.",
		"Local SHA256 checks detect inconsistency, not forgery; the security proof verdict remains inconclusive.",
	}
}

func correlationDigest(c ReceiptObservationCorrelation) (string, error) {
	c.CorrelationSHA256 = ""
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// CorrelatePlanReceiptWithObservation accepts only validated records captured
// in a sensible order. No security proof is upgraded by this link.
func CorrelatePlanReceiptWithObservation(p PlanReceipt, o KubernetesObservation) (ReceiptObservationCorrelation, error) {
	if err := VerifyLivePlanReceipt(p); err != nil {
		return ReceiptObservationCorrelation{}, errors.New("stored Terraform receipt failed validation")
	}
	if err := VerifyKubernetesObservation(o); err != nil {
		return ReceiptObservationCorrelation{}, errors.New("Kubernetes observation failed validation")
	}
	if p.SourceTreeSHA256 != o.SourceTreeSHA256 {
		return ReceiptObservationCorrelation{}, errors.New("Terraform receipt and Kubernetes observation reference different source digests")
	}
	planTime, pErr := time.Parse(time.RFC3339Nano, p.RecordedUTC)
	obsTime, oErr := time.Parse(time.RFC3339Nano, o.CapturedUTC)
	if pErr != nil || oErr != nil || obsTime.Before(planTime) {
		return ReceiptObservationCorrelation{}, errors.New("Kubernetes observation predates the stored Terraform receipt")
	}
	c := ReceiptObservationCorrelation{
		SchemaVersion: ReceiptCorrelationSchema, Mode: ReceiptCorrelationMode,
		ReceiptID: p.ReceiptID, ReceiptSHA256: p.ReceiptSHA256,
		PlanCapturedUTC: p.SanitizedPlanSummary.CapturedUTC, PlanChanges: p.SanitizedPlanSummary.Changes,
		ObservationSHA256: o.ObservationSHA256, ObservationUTC: o.CapturedUTC,
		SourceTreeSHA256: p.SourceTreeSHA256,
		Cluster:          o.Cluster, Context: o.Context, Profile: o.Profile,
		NamespaceCount: o.NamespaceCount, ReadyPodCount: o.ReadyPodCount, PolicyCount: o.PolicyCount,
		ReceiptCorrelation: ReceiptCorrelationStatus,
		PlanApplied:        false, NetworkProbesRun: false,
		ProofVerdict: Inconclusive, Limitations: correlationLimitations(),
	}
	digest, err := correlationDigest(c)
	if err != nil {
		return ReceiptObservationCorrelation{}, err
	}
	c.CorrelationSHA256 = digest
	if err := VerifyReceiptObservationCorrelation(c); err != nil {
		return ReceiptObservationCorrelation{}, err
	}
	return c, nil
}

func VerifyReceiptObservationCorrelation(c ReceiptObservationCorrelation) error {
	if c.SchemaVersion != ReceiptCorrelationSchema || c.Mode != ReceiptCorrelationMode ||
		!validText(c.ReceiptID, 128) || !validHex(c.ReceiptSHA256, 64) ||
		!validHex(c.ObservationSHA256, 64) || !validHex(c.SourceTreeSHA256, 64) ||
		!validUTC(c.PlanCapturedUTC) || !validUTC(c.ObservationUTC) ||
		c.Cluster != "firedoor-local" || c.Context != "kind-firedoor-local" || c.Profile != "hardened" ||
		c.NamespaceCount != 3 || c.ReadyPodCount != 3 || c.PolicyCount != 2 ||
		c.PlanChanges.Add < 0 || c.PlanChanges.Change < 0 || c.PlanChanges.Remove < 0 ||
		c.ReceiptCorrelation != ReceiptCorrelationStatus || c.PlanApplied || c.NetworkProbesRun ||
		c.ProofVerdict != Inconclusive || !reflect.DeepEqual(c.Limitations, correlationLimitations()) ||
		!validHex(c.CorrelationSHA256, 64) {
		return errors.New("unsafe or invalid receipt-observation correlation")
	}
	digest, err := correlationDigest(c)
	if err != nil || digest != c.CorrelationSHA256 {
		return errors.New("receipt-observation correlation SHA256 mismatch")
	}
	return nil
}
