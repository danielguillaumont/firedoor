// Package recoverydeployapply validates the narrowly scoped FD-004 first apply
// request. It never executes Terraform. Authorization is single-use intent for
// exactly one human-confirmed, live-preflighted saved-plan attempt.
package recoverydeployapply

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	Schema               = "firedoor.v10.fd004-single-apply-request.v1"
	ResultSchema         = "firedoor.v10.fd004-single-apply-decision.v1"
	Cluster              = "firedoor-rh-3c138b60a075"
	Context              = "kind-" + Cluster
	GitBase              = "d9e70cc"
	BackendLeaf          = "20261111-205144544-a6591fdd632b"
	BackendType          = "local"
	BackendStatePath     = "firedoor-owned.tfstate"
	PlanSHA256           = "0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7"
	SessionSHA256        = "965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123"
	BackendReceiptSHA256 = "0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4"
	ReviewReceiptSHA256  = "39BF011B45448A06780E669E05C1E31DF9061A05E2F172261D2B5416FAA0CB35"
	Phrase               = "APPLY-firedoor-rh-3c138b60a075-PLAN-0200CE0E-14-CREATE-ONLY"
)

type Request struct {
	SchemaVersion            string `json:"schema_version"`
	Cluster                  string `json:"cluster"`
	Context                  string `json:"context"`
	GitBase                  string `json:"git_base"`
	BackendRunLeaf           string `json:"backend_run_leaf"`
	BackendType              string `json:"backend_type"`
	BackendStatePath         string `json:"backend_state_path"`
	Workspace                string `json:"workspace"`
	SavedPlanSHA256          string `json:"saved_plan_sha256"`
	SessionSHA256            string `json:"session_sha256"`
	BackendReceiptSHA256     string `json:"backend_receipt_sha256"`
	ReviewReceiptSHA256      string `json:"review_receipt_sha256"`
	ReviewReceiptReviewOnly  bool   `json:"review_receipt_review_only"`
	FreshLivePreflightPassed bool   `json:"fresh_live_preflight_passed"`
	PlanCreates              int    `json:"plan_creates"`
	PlanChanges              int    `json:"plan_changes"`
	PlanDestroys             int    `json:"plan_destroys"`
	HardenedIngressPolicies  int    `json:"hardened_ingress_policies"`
	StateOrLockPresent       bool   `json:"state_or_lock_present"`
	PriorApplyAttemptPresent bool   `json:"prior_apply_attempt_present"`
	ProtectedClusterTouched  bool   `json:"protected_cluster_touched"`
	ExplicitApplySwitch      bool   `json:"explicit_apply_switch"`
	TypedConsent             string `json:"typed_consent"`
}

type Decision struct {
	SchemaVersion        string `json:"schema_version"`
	Status               string `json:"status"`
	Cluster              string `json:"cluster"`
	Context              string `json:"context"`
	SavedPlanSHA256      string `json:"saved_plan_sha256"`
	ReviewReceiptSHA256  string `json:"review_receipt_sha256"`
	SingleAttemptAllowed bool   `json:"single_attempt_allowed"`
	TerraformApplyRan    bool   `json:"terraform_apply_ran"`
	ResourcesVerified    bool   `json:"resources_verified"`
	RecoveryProven       bool   `json:"recovery_proven"`
}

func Evaluate(r Request) (Decision, error) {
	if r.SchemaVersion != Schema || r.Cluster != Cluster || r.Context != Context || r.GitBase != GitBase {
		return Decision{}, errors.New("wrong request schema, cluster, context or Git base")
	}
	if r.BackendRunLeaf != BackendLeaf || r.BackendType != BackendType ||
		r.BackendStatePath != BackendStatePath || r.Workspace != "default" {
		return Decision{}, errors.New("wrong initialized local backend identity")
	}
	if r.SavedPlanSHA256 != PlanSHA256 || r.SessionSHA256 != SessionSHA256 ||
		r.BackendReceiptSHA256 != BackendReceiptSHA256 || r.ReviewReceiptSHA256 != ReviewReceiptSHA256 {
		return Decision{}, errors.New("saved plan, backend or review receipt does not match approved evidence")
	}
	if !r.ReviewReceiptReviewOnly || !r.FreshLivePreflightPassed {
		return Decision{}, errors.New("missing independently validated review receipt or fresh F1 preflight")
	}
	if r.PlanCreates != 14 || r.PlanChanges != 0 || r.PlanDestroys != 0 || r.HardenedIngressPolicies != 2 {
		return Decision{}, errors.New("unexpected Terraform planned resource actions")
	}
	if r.StateOrLockPresent || r.PriorApplyAttemptPresent || r.ProtectedClusterTouched {
		return Decision{}, errors.New("state, prior attempt or protected cluster boundary violated")
	}
	if !r.ExplicitApplySwitch || r.TypedConsent != Phrase {
		return Decision{}, errors.New("separate explicit apply intent missing")
	}
	return Decision{
		SchemaVersion: ResultSchema, Status: "one_explicit_apply_attempt_eligible",
		Cluster: Cluster, Context: Context, SavedPlanSHA256: PlanSHA256,
		ReviewReceiptSHA256: ReviewReceiptSHA256, SingleAttemptAllowed: true,
		TerraformApplyRan: false, ResourcesVerified: false, RecoveryProven: false,
	}, nil
}

func Decode(reader io.Reader) (Request, error) {
	limited := bufio.NewReader(io.LimitReader(reader, 1<<20))
	if prefix, err := limited.Peek(3); err == nil && bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
		if _, err := limited.Discard(3); err != nil {
			return Request{}, err
		}
	}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var req Request
	if err := decoder.Decode(&req); err != nil {
		return Request{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Request{}, fmt.Errorf("unexpected trailing request data")
	}
	return req, nil
}

func ValidateDecision(d Decision) error {
	if d.SchemaVersion != ResultSchema || d.Status != "one_explicit_apply_attempt_eligible" ||
		d.Cluster != Cluster || d.Context != Context || d.SavedPlanSHA256 != PlanSHA256 ||
		d.ReviewReceiptSHA256 != ReviewReceiptSHA256 || !d.SingleAttemptAllowed ||
		d.TerraformApplyRan || d.ResourcesVerified || d.RecoveryProven {
		return fmt.Errorf("untrusted single-apply decision")
	}
	return nil
}

// DescribeScope is text only; never interpret a F2 review as permission to apply.
func DescribeScope() string {
	return strings.Join([]string{Cluster, "one_attempt", "14_create_only", "no_destroy"}, ":")
}
