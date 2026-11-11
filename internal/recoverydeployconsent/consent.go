// Package recoverydeployconsent validates FD-004's human review-only intent.
// A valid receipt is never an authorization to terraform apply or destroy.
package recoverydeployconsent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	RequestSchema        = "firedoor.v10.fd004-deployment-review-consent-request.v1"
	ReceiptSchema        = "firedoor.v10.fd004-deployment-review-consent.v1"
	Cluster              = "firedoor-rh-3c138b60a075"
	Context              = "kind-" + Cluster
	BackendLeaf          = "20261111-205144544-a6591fdd632b"
	PlanSHA256           = "0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7"
	SessionSHA256        = "965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123"
	BackendReceiptSHA256 = "0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4"
	ApprovalPhrase       = "REVIEW-firedoor-rh-3c138b60a075-PLAN-0200CE0E-NO-APPLY"
)

type Request struct {
	SchemaVersion           string `json:"schema_version"`
	Cluster                 string `json:"cluster"`
	Context                 string `json:"context"`
	BackendRunLeaf          string `json:"backend_run_leaf"`
	PlanSHA256              string `json:"saved_plan_sha256"`
	SessionSHA256           string `json:"session_sha256"`
	BackendReceiptSHA256    string `json:"backend_receipt_sha256"`
	GitBase                 string `json:"git_base"`
	FreshPreflightPassed    bool   `json:"fresh_preflight_passed"`
	ExistingResourceState   bool   `json:"existing_resource_state"`
	ProtectedClusterTouched bool   `json:"protected_cluster_touched"`
	ConsentPhrase           string `json:"consent_phrase"`
	IssuedUTC               string `json:"issued_utc"`
}

type Receipt struct {
	SchemaVersion                    string `json:"schema_version"`
	Status                           string `json:"status"`
	Cluster                          string `json:"cluster"`
	Context                          string `json:"context"`
	BackendRunLeaf                   string `json:"backend_run_leaf"`
	SavedPlanSHA256                  string `json:"saved_plan_sha256"`
	SessionSHA256                    string `json:"session_sha256"`
	BackendReceiptSHA256             string `json:"backend_receipt_sha256"`
	GitBase                          string `json:"git_base"`
	IssuedUTC                        string `json:"issued_utc"`
	FreshPreflightPassed             bool   `json:"fresh_preflight_passed"`
	ReviewConsentRecorded            bool   `json:"review_consent_recorded"`
	ConsentScope                     string `json:"consent_scope"`
	ManagedResourceOwnershipVerified bool   `json:"managed_resource_ownership_verified"`
	ApplyAuthorized                  bool   `json:"apply_authorized"`
	ApplyExecuted                    bool   `json:"apply_executed"`
	RecoveryProven                   bool   `json:"recovery_proven"`
}

func Evaluate(r Request) (Receipt, error) {
	if r.SchemaVersion != RequestSchema || r.Cluster != Cluster || r.Context != Context {
		return Receipt{}, errors.New("invalid review schema or cluster identity")
	}
	if r.BackendRunLeaf != BackendLeaf || r.PlanSHA256 != PlanSHA256 ||
		r.SessionSHA256 != SessionSHA256 || r.BackendReceiptSHA256 != BackendReceiptSHA256 ||
		r.GitBase != "072e670" {
		return Receipt{}, errors.New("review evidence is not the pinned E3/E4 source")
	}
	if !r.FreshPreflightPassed || r.ExistingResourceState || r.ProtectedClusterTouched {
		return Receipt{}, errors.New("live readiness or state safety boundary failed")
	}
	if r.ConsentPhrase != ApprovalPhrase {
		return Receipt{}, errors.New("review-only consent phrase does not match")
	}
	instant, err := time.Parse(time.RFC3339Nano, r.IssuedUTC)
	if err != nil || instant.Location() != time.UTC || !strings.HasSuffix(r.IssuedUTC, "Z") ||
		instant.IsZero() {
		return Receipt{}, fmt.Errorf("invalid UTC review timestamp")
	}
	return Receipt{
		SchemaVersion: ReceiptSchema,
		Status:        "review_consent_recorded_not_apply_authorized",
		Cluster:       Cluster, Context: Context, BackendRunLeaf: BackendLeaf,
		SavedPlanSHA256: PlanSHA256, SessionSHA256: SessionSHA256,
		BackendReceiptSHA256: BackendReceiptSHA256, GitBase: "072e670", IssuedUTC: r.IssuedUTC,
		FreshPreflightPassed: true, ReviewConsentRecorded: true,
		ConsentScope:                     "review_only_no_terraform_apply",
		ManagedResourceOwnershipVerified: false,
		ApplyAuthorized:                  false, ApplyExecuted: false, RecoveryProven: false,
	}, nil
}

func Decode(r io.Reader) (Request, error) {
	input := bufio.NewReader(io.LimitReader(r, 1<<20))
	if first, err := input.Peek(3); err == nil && bytes.Equal(first, []byte{0xef, 0xbb, 0xbf}) {
		if _, err := input.Discard(3); err != nil {
			return Request{}, err
		}
	}
	dec := json.NewDecoder(input)
	dec.DisallowUnknownFields()
	var value Request
	if err := dec.Decode(&value); err != nil {
		return Request{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Request{}, errors.New("trailing review data")
	}
	return value, nil
}
