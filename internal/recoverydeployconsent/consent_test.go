package recoverydeployconsent

import (
	"encoding/json"
	"strings"
	"testing"
)

func validRequest() Request {
	return Request{
		SchemaVersion: RequestSchema, Cluster: Cluster, Context: Context,
		BackendRunLeaf: BackendLeaf, PlanSHA256: PlanSHA256,
		SessionSHA256: SessionSHA256, BackendReceiptSHA256: BackendReceiptSHA256,
		GitBase: "072e670", FreshPreflightPassed: true,
		ConsentPhrase: ApprovalPhrase, IssuedUTC: "2026-11-11T22:30:00Z",
	}
}
func TestValidReviewNeverAuthorizesApply(t *testing.T) {
	r, err := Evaluate(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !r.ReviewConsentRecorded || !r.FreshPreflightPassed || r.ApplyAuthorized ||
		r.ApplyExecuted || r.RecoveryProven || r.ManagedResourceOwnershipVerified ||
		r.ConsentScope != "review_only_no_terraform_apply" {
		t.Fatalf("unexpected authority: %+v", r)
	}
}
func TestRejectTamperedReview(t *testing.T) {
	cases := map[string]func(*Request){
		"wrong cluster":     func(r *Request) { r.Cluster = "firedoor-local" },
		"wrong context":     func(r *Request) { r.Context = "default" },
		"schema":            func(r *Request) { r.SchemaVersion = "tampered" },
		"wrong backend":     func(r *Request) { r.BackendRunLeaf = "new-backend" },
		"wrong plan":        func(r *Request) { r.PlanSHA256 = strings.Repeat("0", 64) },
		"wrong session":     func(r *Request) { r.SessionSHA256 = strings.Repeat("0", 64) },
		"wrong receipt":     func(r *Request) { r.BackendReceiptSHA256 = strings.Repeat("0", 64) },
		"wrong source":      func(r *Request) { r.GitBase = "0000000" },
		"stale preflight":   func(r *Request) { r.FreshPreflightPassed = false },
		"existing state":    func(r *Request) { r.ExistingResourceState = true },
		"protected touched": func(r *Request) { r.ProtectedClusterTouched = true },
		"unapproved":        func(r *Request) { r.ConsentPhrase = "no" },
		"lowercase phrase":  func(r *Request) { r.ConsentPhrase = strings.ToLower(ApprovalPhrase) },
		"nonUTC timestamp":  func(r *Request) { r.IssuedUTC = "2026-11-11T17:30:00-05:00" },
		"bad timestamp":     func(r *Request) { r.IssuedUTC = "today" },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			f(&r)
			if _, err := Evaluate(r); err == nil {
				t.Fatal("unsafe review accepted")
			}
		})
	}
}
func TestStrictJSONAndUTF8BOM(t *testing.T) {
	data, err := json.Marshal(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{string(data), "\ufeff" + string(data)} {
		req, err := Decode(strings.NewReader(s))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(req); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"{}", "[]", "{\"unexpected\":true}", string(data) + " true",
		"\ufeff\ufeff" + string(data), " \ufeff" + string(data)} {
		decoded, err := Decode(strings.NewReader(s))
		if err == nil {
			_, err = Evaluate(decoded)
		}
		if err == nil {
			t.Fatalf("accepted malformed: %q", s)
		}
	}
}
