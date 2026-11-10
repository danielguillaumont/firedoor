package liveverify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// FD004PlanReviewSchema describes a bounded plan-only review of the proposed
// hardened-to-baseline change. It is NOT authorization to apply a saved plan.
const FD004PlanReviewSchema = "firedoor.v10.fd004-plan-review.v1"
const FD004MaxPlanJSONBytes = 8 << 20

type FD004PlanReview struct {
	SchemaVersion          string   `json:"schema_version"`
	Scenario               string   `json:"scenario"`
	FromProfile            string   `json:"from_profile"`
	ToProfile              string   `json:"to_profile"`
	Cluster                string   `json:"cluster"`
	Context                string   `json:"context"`
	TrackedResources       int      `json:"tracked_resources"`
	InPlacePolicyUpdates   int      `json:"in_place_policy_updates"`
	OtherChanges           int      `json:"other_changes"`
	ReviewedAddresses      []string `json:"reviewed_addresses"`
	Status                 string   `json:"status"`
	ApplyAuthorized        bool     `json:"apply_authorized"`
	TerraformAuthenticated bool     `json:"terraform_authenticated"`
	PolicyCausalityProven  bool     `json:"policy_causality_proven"`
	Limitations            string   `json:"limitations"`
}

// Decode plan JSON with strict duplicate-key detection. Terraform's JSON plan
// format is intentionally open-ended; unknown top-level metadata is accepted,
// while the allowlisted resource addresses, actions and values are strict.
func fd004ReadJSONValue(d *json.Decoder) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return tok, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			kt, err := d.Token()
			if err != nil {
				return nil, err
			}
			k, ok := kt.(string)
			if !ok {
				return nil, errors.New("non-string JSON object key")
			}
			if _, exists := m[k]; exists {
				return nil, fmt.Errorf("duplicate JSON key: %s", k)
			}
			v, err := fd004ReadJSONValue(d)
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errors.New("malformed JSON object")
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			v, err := fd004ReadJSONValue(d)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errors.New("malformed JSON array")
		}
		return a, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}

func fd004PlanObject(v any, label string) (map[string]any, error) {
	o, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be object", label)
	}
	return o, nil
}
func fd004PlanArray(v any, label string) ([]any, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be array", label)
	}
	return a, nil
}
func fd004PlanPath(root any, keys ...string) (any, error) {
	cur := root
	for _, k := range keys {
		m, err := fd004PlanObject(cur, k)
		if err != nil {
			return nil, err
		}
		next, exists := m[k]
		if !exists {
			return nil, fmt.Errorf("missing %s", strings.Join(keys, "."))
		}
		cur = next
	}
	return cur, nil
}
func fd004PlanString(root any, keys ...string) (string, error) {
	v, err := fd004PlanPath(root, keys...)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", strings.Join(keys, "."))
	}
	return s, nil
}
func fd004Actions(change map[string]any) (string, error) {
	v, err := fd004PlanPath(change, "actions")
	if err != nil {
		return "", err
	}
	a, err := fd004PlanArray(v, "actions")
	if err != nil {
		return "", err
	}
	if len(a) != 1 {
		return "", errors.New("resource must have a single no-op or update action")
	}
	s, ok := a[0].(string)
	if !ok {
		return "", errors.New("invalid action")
	}
	return s, nil
}
func fd004PlanFirst(root any, keys ...string) (map[string]any, error) {
	v, err := fd004PlanPath(root, keys...)
	if err != nil {
		return nil, err
	}
	a, err := fd004PlanArray(v, strings.Join(keys, "."))
	if err != nil {
		return nil, err
	}
	if len(a) != 1 {
		return nil, fmt.Errorf("expected one %s item", strings.Join(keys, "."))
	}
	return fd004PlanObject(a[0], strings.Join(keys, "."))
}
func fd004Selector(policy map[string]any) (map[string]any, error) {
	spec, err := fd004PlanFirst(policy, "spec")
	if err != nil {
		return nil, err
	}
	ingress, err := fd004PlanFirst(spec, "ingress")
	if err != nil {
		return nil, err
	}
	from, err := fd004PlanFirst(ingress, "from")
	if err != nil {
		return nil, err
	}
	selector, err := fd004PlanFirst(from, "namespace_selector")
	if err != nil {
		return nil, err
	}
	return selector, nil
}
func fd004CloneJSONValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, x := range val {
			out[k] = fd004CloneJSONValue(x)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, x := range val {
			out[i] = fd004CloneJSONValue(x)
		}
		return out
	default:
		return v
	}
}
func fd004PlanPolicy(zone string, change map[string]any) error {
	before, err := fd004PlanObject(change["before"], "policy.before")
	if err != nil {
		return err
	}
	after, err := fd004PlanObject(change["after"], "policy.after")
	if err != nil {
		return err
	}
	// Clone without converting json.Number to float64, then compare everything
	// except the one explicitly approved selector change.
	normalized := fd004CloneJSONValue(after).(map[string]any)
	beforeSel, err := fd004Selector(before)
	if err != nil {
		return err
	}
	afterSel, err := fd004Selector(normalized)
	if err != nil {
		return err
	}
	beforeLabels, err := fd004PlanObject(beforeSel["match_labels"], "hardened labels")
	if err != nil {
		return err
	}
	expectedSrc := "edge"
	if zone == "data" {
		expectedSrc = "app"
	}
	if len(beforeLabels) != 1 || beforeLabels["firedoor.dev/zone"] != expectedSrc {
		return fmt.Errorf("%s hardened source selector mismatch", zone)
	}
	afterLabels := afterSel["match_labels"]
	if afterLabels != nil {
		m, err := fd004PlanObject(afterLabels, "baseline labels")
		if err != nil {
			return err
		}
		if len(m) != 0 {
			return fmt.Errorf("%s baseline must use empty source selector", zone)
		}
	}
	// Only selector labels may change. Other changes including ports, selectors,
	// names, annotations and unrelated rules are deliberately rejected.
	afterSel["match_labels"] = beforeLabels
	if !reflect.DeepEqual(before, normalized) {
		return fmt.Errorf("%s plan changes attributes beyond the namespace source selector", zone)
	}
	// Attribute contract additionally pins policy identity and ingress port.
	meta, err := fd004PlanFirst(before, "metadata")
	if err != nil {
		return err
	}
	if meta["name"] != "firedoor-"+zone+"-ingress" || meta["namespace"] != "firedoor-"+zone {
		return fmt.Errorf("%s policy resource identity mismatch", zone)
	}
	ownerLabels, err := fd004PlanObject(meta["labels"], "policy ownership labels")
	if err != nil || ownerLabels["firedoor.dev/managed-by"] != "terraform" || ownerLabels["app.kubernetes.io/part-of"] != "firedoor" {
		return fmt.Errorf("%s policy ownership labels mismatch", zone)
	}
	spec, err := fd004PlanFirst(before, "spec")
	if err != nil {
		return err
	}
	types, err := fd004PlanArray(spec["policy_types"], "policy_types")
	if err != nil {
		return err
	}
	if len(types) != 1 || types[0] != "Ingress" {
		return errors.New("unexpected policy type")
	}
	podSel, err := fd004PlanFirst(spec, "pod_selector")
	if err != nil {
		return err
	}
	labels, err := fd004PlanObject(podSel["match_labels"], "pod selector")
	if err != nil {
		return err
	}
	if len(labels) != 1 || labels["app.kubernetes.io/name"] != "firedoor-"+zone {
		return errors.New("unexpected policy Pod selector")
	}
	ingress, err := fd004PlanFirst(spec, "ingress")
	if err != nil {
		return err
	}
	ports, err := fd004PlanFirst(ingress, "ports")
	if err != nil {
		return err
	}
	if ports["port"] != "8080" || ports["protocol"] != "TCP" {
		return errors.New("unexpected ingress port or protocol")
	}
	return nil
}

// ReviewFD004BaselinePlanJSON checks a narrowly-scoped saved Terraform plan,
// NOT the provenance of state, live CNI behavior, plan applicability or authority
// to apply. A passing review must never be interpreted as approval to mutate.
func ReviewFD004BaselinePlanJSON(raw []byte) (FD004PlanReview, error) {
	fail := func(s string) (FD004PlanReview, error) { return FD004PlanReview{}, errors.New(s) }
	if len(raw) == 0 || len(raw) > FD004MaxPlanJSONBytes || bytes.IndexByte(raw, 0) >= 0 {
		return fail("plan JSON missing, oversized or contains NUL")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := fd004ReadJSONValue(d)
	if err != nil {
		return FD004PlanReview{}, fmt.Errorf("invalid plan JSON: %w", err)
	}
	if _, err = d.Token(); err != io.EOF {
		return fail("trailing plan JSON or invalid trailing data")
	}
	root, err := fd004PlanObject(value, "plan")
	if err != nil {
		return FD004PlanReview{}, err
	}
	version, err := fd004PlanString(root, "format_version")
	if err != nil || !strings.HasPrefix(version, "1.") {
		return fail("unsupported Terraform JSON plan format_version")
	}
	vars, err := fd004PlanObject(root["variables"], "variables")
	if err != nil {
		return FD004PlanReview{}, err
	}
	if len(vars) != 3 {
		return fail("Terraform variables differ from three expected testbed inputs")
	}
	expectedVars := map[string]string{"security_profile": "baseline", "kube_context": "kind-firedoor-local", "kubeconfig_path": "~/.kube/config"}
	for k, want := range expectedVars {
		v, err := fd004PlanString(vars, k, "value")
		if err != nil || v != want {
			return fail("unexpected Terraform plan variable " + k)
		}
	}
	if drift, exists := root["resource_drift"]; exists && drift != nil {
		items, ok := drift.([]any)
		if !ok || len(items) != 0 {
			return fail("Terraform plan includes resource drift records")
		}
	}
	resourceVal, err := fd004PlanPath(root, "resource_changes")
	if err != nil {
		return FD004PlanReview{}, err
	}
	resources, err := fd004PlanArray(resourceVal, "resource_changes")
	if err != nil {
		return FD004PlanReview{}, err
	}
	if len(resources) != 14 {
		return fail("expected exactly 14 managed resource changes including no-ops")
	}
	allowed := map[string]string{}
	for _, zone := range []string{"edge", "app", "data"} {
		for _, t := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			allowed[fmt.Sprintf(`module.workloads[%q].%s.this`, zone, t)] = "no-op"
		}
	}
	for _, zone := range []string{"app", "data"} {
		allowed[fmt.Sprintf(`module.network_boundaries[%q].kubernetes_network_policy_v1.this`, zone)] = "update"
	}
	seen := map[string]bool{}
	approved := []string{}
	for _, v := range resources {
		r, err := fd004PlanObject(v, "resource_change")
		if err != nil {
			return FD004PlanReview{}, err
		}
		addr, ok := r["address"].(string)
		if !ok {
			return fail("missing resource address")
		}
		actionWant, allowedAddress := allowed[addr]
		if !allowedAddress || seen[addr] || r["mode"] != "managed" {
			return fail("unrecognized or duplicate managed resource address")
		}
		seen[addr] = true
		// The resource address and declared type must agree.
		typeName, ok := r["type"].(string)
		if !ok || typeName == "" || !strings.Contains(addr, "."+typeName+".this") {
			return fail("resource type/address mismatch for " + addr)
		}
		c, err := fd004PlanObject(r["change"], "resource change")
		if err != nil {
			return FD004PlanReview{}, err
		}
		action, err := fd004Actions(c)
		if err != nil || action != actionWant {
			return fail("unexpected planned resource action for " + addr)
		}
		if rp, exists := c["replace_paths"]; exists && rp != nil {
			arr, ok := rp.([]any)
			if !ok || len(arr) != 0 {
				return fail("replacement paths present in plan")
			}
		}
		if action == "update" {
			zone := "app"
			if strings.Contains(addr, `["data"]`) {
				zone = "data"
			}
			if err := fd004PlanPolicy(zone, c); err != nil {
				return FD004PlanReview{}, fmt.Errorf("unsafe policy plan %s: %w", addr, err)
			}
			approved = append(approved, addr)
		} else {
			if c["before"] == nil || c["after"] == nil || !reflect.DeepEqual(c["before"], c["after"]) {
				return fail("no-op workload is missing or has changed values")
			}
		}
	}
	if len(seen) != len(allowed) || len(approved) != 2 {
		return fail("incomplete Terraform managed resource inventory")
	}
	outputs, err := fd004PlanObject(root["output_changes"], "output_changes")
	if err != nil {
		return FD004PlanReview{}, err
	}
	knownOutput := map[string]bool{"active_profile": true, "owned_namespaces": true, "services": true, "managed_ingress_policies": true}
	if len(outputs) > len(knownOutput) {
		return fail("unknown Terraform output change")
	}
	changedProfile := false
	for name, entry := range outputs {
		if !knownOutput[name] {
			return fail("unknown Terraform output: " + name)
		}
		change, err := fd004PlanObject(entry, "output_change")
		if err != nil {
			return FD004PlanReview{}, err
		}
		if embedded, ok := change["change"]; ok {
			change, err = fd004PlanObject(embedded, "nested output change")
			if err != nil {
				return FD004PlanReview{}, err
			}
		}
		action, err := fd004Actions(change)
		if err != nil {
			return FD004PlanReview{}, err
		}
		if name == "active_profile" {
			if action != "update" || change["before"] != "hardened" || change["after"] != "baseline" {
				return fail("active_profile output must change hardened to baseline")
			}
			changedProfile = true
		} else if action != "no-op" {
			return fail("unrelated Terraform output would change")
		}
	}
	if !changedProfile {
		return fail("missing active_profile output update")
	}
	// sort output deterministically, regardless of Terraform resource order.
	if approved[0] > approved[1] {
		approved[0], approved[1] = approved[1], approved[0]
	}
	return FD004PlanReview{
		SchemaVersion: FD004PlanReviewSchema, Scenario: "FD-004", FromProfile: "hardened", ToProfile: "baseline", Cluster: OwnedCluster, Context: OwnedContext,
		TrackedResources: 14, InPlacePolicyUpdates: 2, OtherChanges: 0, ReviewedAddresses: approved, Status: "plan_shape_reviewed_no_apply_authorization",
		ApplyAuthorized: false, TerraformAuthenticated: false, PolicyCausalityProven: false,
		Limitations: "Offline JSON plan-shape review only. Local state and provider origin are not cryptographically authenticated. No apply was authorized or performed; no behavior, recovery or CNI cause is proven. A later separately approved operation must revalidate state, resource ownership, plan SHA256 and hardened recovery.",
	}, nil
}
