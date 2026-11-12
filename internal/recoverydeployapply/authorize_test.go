package recoverydeployapply

import (
	"encoding/json"
	"strings"
	"testing"
)

func approved() Request {
	return Request{
		SchemaVersion: Schema, Cluster: Cluster, Context: Context, GitBase: GitBase,
		BackendRunLeaf: BackendLeaf, BackendType: BackendType, BackendStatePath: BackendStatePath,
		Workspace: "default", SavedPlanSHA256: PlanSHA256,
		SessionSHA256: SessionSHA256, BackendReceiptSHA256: BackendReceiptSHA256,
		ReviewReceiptSHA256: ReviewReceiptSHA256, ReviewReceiptReviewOnly: true,
		FreshLivePreflightPassed: true, PlanCreates: 14, HardenedIngressPolicies: 2,
		ExplicitApplySwitch: true, TypedConsent: Phrase,
	}
}

func TestExplicitSingleAttempt(t *testing.T) {
	decision, err := Evaluate(approved())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDecision(decision); err != nil {
		t.Fatal(err)
	}
	if decision.TerraformApplyRan || decision.ResourcesVerified || decision.RecoveryProven {
		t.Fatal("authorization confused with execution or proof")
	}
}

func TestRejectUnsafe(t *testing.T) {
	cases := map[string]func(*Request){
		"protected":         func(r *Request) { r.Cluster = "firedoor-local" },
		"context":           func(r *Request) { r.Context = "kind-firedoor-local" },
		"schema":            func(r *Request) { r.SchemaVersion = "wrong" },
		"git":               func(r *Request) { r.GitBase = "072e670" },
		"backend":           func(r *Request) { r.BackendType = "s3" },
		"backend leaf":      func(r *Request) { r.BackendRunLeaf = "other" },
		"state path":        func(r *Request) { r.BackendStatePath = "terraform.tfstate" },
		"workspace":         func(r *Request) { r.Workspace = "other" },
		"plan":              func(r *Request) { r.SavedPlanSHA256 = strings.Repeat("0", 64) },
		"session":           func(r *Request) { r.SessionSHA256 = strings.Repeat("0", 64) },
		"backend receipt":   func(r *Request) { r.BackendReceiptSHA256 = strings.Repeat("0", 64) },
		"review receipt":    func(r *Request) { r.ReviewReceiptSHA256 = strings.Repeat("0", 64) },
		"review scope":      func(r *Request) { r.ReviewReceiptReviewOnly = false },
		"old preflight":     func(r *Request) { r.FreshLivePreflightPassed = false },
		"extra create":      func(r *Request) { r.PlanCreates = 15 },
		"update":            func(r *Request) { r.PlanChanges = 1 },
		"destroy":           func(r *Request) { r.PlanDestroys = 1 },
		"policy":            func(r *Request) { r.HardenedIngressPolicies = 1 },
		"existing state":    func(r *Request) { r.StateOrLockPresent = true },
		"previous attempt":  func(r *Request) { r.PriorApplyAttemptPresent = true },
		"protected changed": func(r *Request) { r.ProtectedClusterTouched = true },
		"no apply switch":   func(r *Request) { r.ExplicitApplySwitch = false },
		"F2 review phrase":  func(r *Request) { r.TypedConsent = "REVIEW-firedoor-rh-3c138b60a075-PLAN-0200CE0E-NO-APPLY" },
		"no typed consent":  func(r *Request) { r.TypedConsent = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := approved()
			change(&r)
			if _, err := Evaluate(r); err == nil {
				t.Fatal("unsafe attempt authorized")
			}
		})
	}
}

func TestStrictJSONAndOneBOM(t *testing.T) {
	valid, err := json.Marshal(approved())
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{string(valid), "\ufeff" + string(valid)} {
		req, err := Decode(strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Evaluate(req); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []string{
		"", "{}", "null", "{\"unknown\":true}", string(valid) + " {}", "\ufeff\ufeff" + string(valid),
	} {
		req, err := Decode(strings.NewReader(bad))
		if err == nil {
			_, err = Evaluate(req)
		}
		if err == nil {
			t.Fatalf("unexpected JSON acceptance: %q", bad)
		}
	}
}

func TestRejectForgedPositiveDecision(t *testing.T) {
	d, _ := Evaluate(approved())
	d.TerraformApplyRan = true
	if err := ValidateDecision(d); err == nil {
		t.Fatal("forged executed decision")
	}
}
