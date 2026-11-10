package liveverify

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Create a baseline -> hardened fixture by reversing only the same two policy
// values and profile output in the existing hardened -> baseline test fixture.
func fd004RecoveryFakePlan() map[string]any {
	p := fd004ReviewFakePlan()
	p["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "hardened"
	for _, zone := range []string{"app", "data"} {
		change := fd004ReviewPolicyChange(p, zone)
		change["before"], change["after"] = change["after"], change["before"]
	}
	output := p["output_changes"].(map[string]any)["active_profile"].(map[string]any)
	output["before"], output["after"] = output["after"], output["before"]
	return p
}

func TestFD004HardenedRecoveryPlanValid(t *testing.T) {
	p := fd004RecoveryFakePlan()
	report, err := ReviewFD004HardenedRecoveryPlanJSON(fd004ReviewJSON(p))
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != FD004RecoveryPlanReviewSchema || report.FromProfile != "baseline" || report.ToProfile != "hardened" || report.InPlacePolicyUpdates != 2 || report.OtherChanges != 0 || report.TrackedResources != 14 || report.ApplyAuthorized || report.TerraformAuthenticated || report.PolicyCausalityProven || report.Status != "recovery_plan_shape_reviewed_no_apply_authorization" {
		t.Fatalf("invalid recovery report: %+v", report)
	}
	if len(report.ReviewedAddresses) != 2 {
		t.Fatal("missing recovery policy addresses")
	}
	// Nested output change is permitted by Terraform JSON but must be exact.
	p = fd004RecoveryFakePlan()
	output := p["output_changes"].(map[string]any)["active_profile"].(map[string]any)
	p["output_changes"].(map[string]any)["active_profile"] = map[string]any{"change": output}
	if _, err := ReviewFD004HardenedRecoveryPlanJSON(fd004ReviewJSON(p)); err != nil {
		t.Fatal(err)
	}
}

func TestFD004HardenedRecoveryPlanRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"wrong_target_profile", func(p map[string]any) {
			p["variables"].(map[string]any)["security_profile"].(map[string]any)["value"] = "baseline"
		}},
		{"wrong_context", func(p map[string]any) {
			p["variables"].(map[string]any)["kube_context"].(map[string]any)["value"] = "production"
		}},
		{"extra_variable", func(p map[string]any) { p["variables"].(map[string]any)["extra"] = map[string]any{"value": "x"} }},
		{"wrong_from_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["active_profile"].(map[string]any)["before"] = "hardened"
		}},
		{"wrong_to_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["active_profile"].(map[string]any)["after"] = "baseline"
		}},
		{"missing_output", func(p map[string]any) { delete(p, "output_changes") }},
		{"added_output", func(p map[string]any) {
			p["output_changes"].(map[string]any)["other"] = map[string]any{"actions": []any{"update"}}
		}},
		{"resource_drift", func(p map[string]any) { p["resource_drift"] = []any{map[string]any{"address": "bad"}} }},
		{"extra_resource", func(p map[string]any) {
			p["resource_changes"] = append(p["resource_changes"].([]any), map[string]any{"address": "foreign"})
		}},
		{"missing_resource", func(p map[string]any) { p["resource_changes"] = p["resource_changes"].([]any)[:13] }},
		{"replacement", func(p map[string]any) { fd004ReviewPolicyChange(p, "app")["actions"] = []any{"delete", "create"} }},
		{"deleted_policy", func(p map[string]any) { fd004ReviewPolicyChange(p, "data")["actions"] = []any{"delete"} }},
		{"replaced_paths", func(p map[string]any) { fd004ReviewPolicyChange(p, "app")["replace_paths"] = []any{[]any{"spec"}} }},
		{"wrong_after_selector", func(p map[string]any) {
			s, _ := fd004Selector(fd004ReviewPolicyChange(p, "app")["after"].(map[string]any))
			s["match_labels"] = map[string]any{}
		}},
		{"wrong_before_selector", func(p map[string]any) {
			s, _ := fd004Selector(fd004ReviewPolicyChange(p, "data")["before"].(map[string]any))
			s["match_labels"] = map[string]any{"firedoor.dev/zone": "edge"}
		}},
		{"changed_port", func(p map[string]any) {
			sp, _ := fd004PlanFirst(fd004ReviewPolicyChange(p, "data")["after"].(map[string]any), "spec")
			in, _ := fd004PlanFirst(sp, "ingress")
			port, _ := fd004PlanFirst(in, "ports")
			port["port"] = "9090"
		}},
		{"changed_policy_identity", func(p map[string]any) {
			meta, _ := fd004PlanFirst(fd004ReviewPolicyChange(p, "app")["after"].(map[string]any), "metadata")
			meta["name"] = "foreign"
		}},
		{"changed_workload", func(p map[string]any) {
			p["resource_changes"].([]any)[0].(map[string]any)["change"].(map[string]any)["actions"] = []any{"update"}
		}},
		{"duplicate_address", func(p map[string]any) {
			p["resource_changes"].([]any)[0].(map[string]any)["address"] = p["resource_changes"].([]any)[1].(map[string]any)["address"]
		}},
		{"wrong_type", func(p map[string]any) {
			p["resource_changes"].([]any)[13].(map[string]any)["type"] = "kubernetes_namespace_v1"
		}},
		{"missing_before", func(p map[string]any) { delete(fd004ReviewPolicyChange(p, "data"), "before") }},
		{"missing_after", func(p map[string]any) { delete(fd004ReviewPolicyChange(p, "data"), "after") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fd004RecoveryFakePlan()
			tc.edit(p)
			if result, err := ReviewFD004HardenedRecoveryPlanJSON(fd004ReviewJSON(p)); err == nil {
				t.Fatalf("unsafe plan accepted: %+v", result)
			}
		})
	}
}
func TestFD004HardenedRecoveryPlanMalformed(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty":          nil,
		"trailing":       append(fd004ReviewJSON(fd004RecoveryFakePlan()), []byte(`{}`)...),
		"duplicate":      []byte(`{"format_version":"1.2","format_version":"1.2"}`),
		"oversized":      bytes.Repeat([]byte(" "), FD004MaxPlanJSONBytes+1),
		"null_byte":      append(fd004ReviewJSON(fd004RecoveryFakePlan()), 0),
		"unknown_format": []byte(`{"format_version":"2.0"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReviewFD004HardenedRecoveryPlanJSON(b); err == nil {
				t.Fatal("invalid recovery plan accepted")
			}
		})
	}
}
func TestFD004HardenedRecoveryPlanDoesNotAlterSource(t *testing.T) {
	original := fd004ReviewJSON(fd004RecoveryFakePlan())
	copyBytes := bytes.Clone(original)
	_, err := ReviewFD004HardenedRecoveryPlanJSON(copyBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, copyBytes) {
		t.Fatal("mutated raw plan input")
	}
	var v any
	if err := json.Unmarshal(original, &v); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(original), "apply_authorized") {
		t.Fatal("fixture claims apply authorization")
	}
}
