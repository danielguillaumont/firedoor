package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// FD004RecoveryPlanReviewSchema identifies a recovery-plan *shape* review.
// Neither this report nor the preceding baseline-plan review authorizes apply.
const FD004RecoveryPlanReviewSchema = "firedoor.v10.fd004-hardened-recovery-plan-review.v1"

// ReviewFD004HardenedRecoveryPlanJSON accepts only a Terraform JSON plan that
// changes the same two owned policies from baseline selectors to hardened
// selectors. It reuses the strict baseline-plan validator by reversing the two
// policy value pairs and the active_profile output in an in-memory parsed copy.
// No Terraform command, provider call or mutation occurs in this function.
func ReviewFD004HardenedRecoveryPlanJSON(raw []byte) (FD004PlanReview, error) {
	empty := FD004PlanReview{}
	if len(raw) == 0 || len(raw) > FD004MaxPlanJSONBytes || bytes.IndexByte(raw, 0) >= 0 {
		return empty, errors.New("recovery plan JSON missing, oversized or contains NUL")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	parsed, err := fd004ReadJSONValue(d) // duplicate keys fail closed
	if err != nil {
		return empty, fmt.Errorf("invalid recovery plan JSON: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return empty, errors.New("trailing recovery plan JSON or malformed trailing data")
	}
	root, err := fd004PlanObject(parsed, "recovery plan")
	if err != nil {
		return empty, err
	}
	vars, err := fd004PlanObject(root["variables"], "recovery variables")
	if err != nil {
		return empty, err
	}
	security, err := fd004PlanObject(vars["security_profile"], "recovery security_profile")
	if err != nil {
		return empty, err
	}
	if security["value"] != "hardened" {
		return empty, errors.New("recovery plan target variable must be hardened")
	}
	security["value"] = "baseline" // normalize only for existing strict contract

	resources, err := fd004PlanArray(root["resource_changes"], "recovery resource_changes")
	if err != nil {
		return empty, err
	}
	if len(resources) != 14 {
		return empty, errors.New("recovery plan requires exactly 14 managed resources")
	}
	const app = `module.network_boundaries["app"].kubernetes_network_policy_v1.this`
	const data = `module.network_boundaries["data"].kubernetes_network_policy_v1.this`
	policyCount := 0
	for _, resource := range resources {
		r, err := fd004PlanObject(resource, "recovery resource")
		if err != nil {
			return empty, err
		}
		address, ok := r["address"].(string)
		if !ok {
			return empty, errors.New("missing recovery resource address")
		}
		if address != app && address != data {
			continue // strict forward validator rejects unknown or non-no-op resources
		}
		change, err := fd004PlanObject(r["change"], "recovery policy change")
		if err != nil {
			return empty, err
		}
		if change["before"] == nil || change["after"] == nil {
			return empty, errors.New("recovery policy values must include before and after")
		}
		change["before"], change["after"] = change["after"], change["before"]
		policyCount++
	}
	if policyCount != 2 {
		return empty, errors.New("recovery requires exactly two policy updates")
	}

	outputs, err := fd004PlanObject(root["output_changes"], "recovery output_changes")
	if err != nil {
		return empty, err
	}
	profileEntry, err := fd004PlanObject(outputs["active_profile"], "recovery active_profile")
	if err != nil {
		return empty, err
	}
	profile := profileEntry
	if nested, exists := profileEntry["change"]; exists {
		profile, err = fd004PlanObject(nested, "nested recovery active_profile")
		if err != nil {
			return empty, err
		}
	}
	if profile["before"] != "baseline" || profile["after"] != "hardened" {
		return empty, errors.New("recovery output must change baseline to hardened")
	}
	profile["before"], profile["after"] = profile["after"], profile["before"]

	// Preserve numeric equality and all unrelated fields. The existing forward
	// reviewer re-checks all 14 addresses, no-ops, policy identities, ports,
	// ownership metadata, exact selector changes, actions and output constraints.
	normalized, err := json.Marshal(root)
	if err != nil {
		return empty, fmt.Errorf("marshal normalized recovery plan: %w", err)
	}
	forward, err := ReviewFD004BaselinePlanJSON(normalized)
	if err != nil {
		return empty, fmt.Errorf("unsafe hardened recovery plan: %w", err)
	}
	forward.SchemaVersion = FD004RecoveryPlanReviewSchema
	forward.FromProfile = "baseline"
	forward.ToProfile = "hardened"
	forward.Status = "recovery_plan_shape_reviewed_no_apply_authorization"
	forward.ApplyAuthorized = false
	forward.TerraformAuthenticated = false
	forward.PolicyCausalityProven = false
	forward.Limitations = "Offline Terraform recovery plan-shape review only; no recovery plan has been applied, current state is not authenticated, and cluster behavior or restored hardening is not proven. Never use as apply authorization. Prepare independent hardened recovery and explicit bounded approval before any live weakening."
	return forward, nil
}
