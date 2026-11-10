// Package deployreview checks the shape of an initial FD-004 Terraform plan.
// It cannot authenticate provider identity or authorize a Terraform apply.
package deployreview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const Schema = "firedoor.v10.fd004-isolated-deploy-plan.v1"
const Limit = 8 << 20
const Status = "initial_hardened_plan_shape_reviewed_no_apply_authorization"

var clusterPattern = regexp.MustCompile(`^firedoor-rh-[a-f0-9]{12}$`)

type Review struct {
	SchemaVersion          string   `json:"schema_version"`
	Scenario               string   `json:"scenario"`
	Status                 string   `json:"status"`
	Cluster                string   `json:"cluster"`
	Context                string   `json:"context"`
	Profile                string   `json:"profile"`
	ResourceCount          int      `json:"resource_count"`
	PolicyCount            int      `json:"hardened_policy_count"`
	Addresses              []string `json:"reviewed_addresses"`
	ApplyAuthorized        bool     `json:"apply_authorized"`
	TerraformAuthenticated bool     `json:"terraform_authenticated"`
	ClusterChecked         bool     `json:"cluster_checked"`
	RecoveryProven         bool     `json:"recovery_proven"`
	Limitations            string   `json:"limitations"`
}

func strictValue(dec *json.Decoder) (any, error) {
	t, e := dec.Token()
	if e != nil {
		return nil, e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return t, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for dec.More() {
			k, e := dec.Token()
			if e != nil {
				return nil, e
			}
			key, ok := k.(string)
			if !ok {
				return nil, errors.New("non-string JSON key")
			}
			if _, exists := m[key]; exists {
				return nil, fmt.Errorf("duplicate key %q", key)
			}
			v, e := strictValue(dec)
			if e != nil {
				return nil, e
			}
			m[key] = v
		}
		end, e := dec.Token()
		if e != nil || end != json.Delim('}') {
			return nil, errors.New("invalid object delimiter")
		}
		return m, nil
	case '[':
		a := []any{}
		for dec.More() {
			v, e := strictValue(dec)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		end, e := dec.Token()
		if e != nil || end != json.Delim(']') {
			return nil, errors.New("invalid array delimiter")
		}
		return a, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}
func obj(v any, label string) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be object", label)
	}
	return m, nil
}
func arr(v any, label string) ([]any, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be array", label)
	}
	return a, nil
}
func stringAt(m map[string]any, key string) (string, error) {
	v, ok := m[key].(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	return v, nil
}
func single(m map[string]any, key string) (map[string]any, error) {
	a, e := arr(m[key], key)
	if e != nil {
		return nil, e
	}
	if len(a) != 1 {
		return nil, fmt.Errorf("%s must have exactly one entry", key)
	}
	return obj(a[0], key)
}
func action(change map[string]any) (string, error) {
	a, e := arr(change["actions"], "actions")
	if e != nil || len(a) != 1 {
		return "", errors.New("action must contain exactly one operation")
	}
	s, ok := a[0].(string)
	if !ok {
		return "", errors.New("action must be string")
	}
	return s, nil
}
func expectedResources() map[string]string {
	m := map[string]string{}
	for _, z := range []string{"edge", "app", "data"} {
		for _, t := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			m[fmt.Sprintf(`module.workloads[%q].%s.this`, z, t)] = t
		}
	}
	for _, z := range []string{"app", "data"} {
		m[fmt.Sprintf(`module.network_boundaries[%q].kubernetes_network_policy_v1.this`, z)] = "kubernetes_network_policy_v1"
	}
	return m
}
func zoneAndType(address string) (string, string) {
	for _, z := range []string{"edge", "app", "data"} {
		if strings.Contains(address, `["`+z+`"]`) {
			switch {
			case strings.Contains(address, ".kubernetes_network_policy_v1."):
				return z, "policy"
			case strings.Contains(address, ".kubernetes_namespace_v1."):
				return z, "namespace"
			case strings.Contains(address, ".kubernetes_service_account_v1."):
				return z, "serviceaccount"
			case strings.Contains(address, ".kubernetes_service_v1."):
				return z, "service"
			case strings.Contains(address, ".kubernetes_deployment_v1."):
				return z, "deployment"
			}
		}
	}
	return "", ""
}
func exactLabels(m map[string]any, kvs map[string]string) bool {
	if len(m) != len(kvs) {
		return false
	}
	for k, v := range kvs {
		if m[k] != v {
			return false
		}
	}
	return true
}
func checkResource(address string, after map[string]any) error {
	z, t := zoneAndType(address)
	if z == "" {
		return errors.New("unsupported resource kind")
	}
	meta, e := single(after, "metadata")
	if e != nil {
		return e
	}
	expectedName := "firedoor-" + z
	if t == "serviceaccount" {
		expectedName += "-sa"
	}
	if t == "policy" {
		expectedName += "-ingress"
	}
	if meta["name"] != expectedName {
		return fmt.Errorf("%s wrong metadata.name", address)
	}
	if t != "namespace" && meta["namespace"] != "firedoor-"+z {
		return fmt.Errorf("%s wrong namespace", address)
	}
	labels, e := obj(meta["labels"], "metadata.labels")
	if e != nil {
		return e
	}
	if labels["app.kubernetes.io/part-of"] != "firedoor" || labels["firedoor.dev/managed-by"] != "terraform" {
		return fmt.Errorf("%s owner labels invalid", address)
	}
	wantLabels := map[string]string{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}
	if t != "policy" {
		wantLabels["app.kubernetes.io/name"] = "firedoor-" + z
		wantLabels["firedoor.dev/zone"] = z
	}
	if t == "namespace" {
		wantLabels["pod-security.kubernetes.io/enforce"] = "baseline"
	}
	if !exactLabels(labels, wantLabels) {
		return fmt.Errorf("%s metadata labels differ from owned lab contract", address)
	}
	if t == "namespace" || t == "serviceaccount" || t == "deployment" || t == "service" {
		if t != "namespace" && labels["firedoor.dev/zone"] != z {
			return fmt.Errorf("%s zone label invalid", address)
		}
		if t == "serviceaccount" && after["automount_service_account_token"] != false {
			return errors.New("service account token must not automount")
		}
		return nil
	}
	spec, e := single(after, "spec")
	if e != nil {
		return e
	}
	types, e := arr(spec["policy_types"], "policy_types")
	if e != nil || len(types) != 1 || types[0] != "Ingress" {
		return errors.New("policy must restrict ingress only")
	}
	pod, e := single(spec, "pod_selector")
	if e != nil {
		return e
	}
	podLabels, e := obj(pod["match_labels"], "pod labels")
	if e != nil || !exactLabels(podLabels, map[string]string{"app.kubernetes.io/name": "firedoor-" + z}) {
		return errors.New("policy Pod selector invalid")
	}
	ingress, e := single(spec, "ingress")
	if e != nil {
		return e
	}
	from, e := single(ingress, "from")
	if e != nil {
		return e
	}
	sel, e := single(from, "namespace_selector")
	if e != nil {
		return e
	}
	source := "edge"
	if z == "data" {
		source = "app"
	}
	sourceLabels, e := obj(sel["match_labels"], "source match_labels")
	if e != nil || !exactLabels(sourceLabels, map[string]string{"firedoor.dev/zone": source}) {
		return fmt.Errorf("%s policy is not hardened", z)
	}
	port, e := single(ingress, "ports")
	if e != nil {
		return e
	}
	if port["port"] != "8080" || port["protocol"] != "TCP" {
		return fmt.Errorf("%s policy port/protocol invalid", z)
	}
	return nil
}

// ReviewJSON validates the 14-address initial creation shape, not a plan's
// provenance. The caller must separately verify a disposable kubeconfig,
// committed HCL, private state, cluster ownership and saved plan SHA256.
func ReviewJSON(raw []byte, cluster, kubeconfig string) (Review, error) {
	fail := func(m string) (Review, error) { return Review{}, errors.New(m) }
	if !clusterPattern.MatchString(cluster) {
		return fail("disposable cluster identity invalid")
	}
	if kubeconfig == "" || strings.ContainsAny(kubeconfig, "\r\n\x00") || strings.Contains(kubeconfig, "~") {
		return fail("expected private kubeconfig missing or invalid")
	}
	if len(raw) == 0 || len(raw) > Limit || bytes.IndexByte(raw, 0) >= 0 {
		return fail("JSON missing, oversized or contains NUL")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, e := strictValue(dec)
	if e != nil {
		return Review{}, fmt.Errorf("invalid JSON: %w", e)
	}
	if _, e = dec.Token(); e != io.EOF {
		return fail("trailing data after plan JSON")
	}
	root, e := obj(v, "plan")
	if e != nil {
		return Review{}, e
	}
	format, e := stringAt(root, "format_version")
	if e != nil || !strings.HasPrefix(format, "1.") {
		return fail("unsupported Terraform JSON format")
	}
	variables, e := obj(root["variables"], "variables")
	if e != nil {
		return Review{}, e
	}
	if len(variables) != 3 {
		return fail("unexpected variable set")
	}
	for k, want := range map[string]string{"security_profile": "hardened", "kube_context": "kind-" + cluster, "kubeconfig_path": kubeconfig} {
		v, e := obj(variables[k], k)
		if e != nil || v["value"] != want {
			return fail("wrong Terraform variable " + k)
		}
	}
	if drift, ok := root["resource_drift"]; ok && drift != nil {
		a, e := arr(drift, "resource_drift")
		if e != nil || len(a) != 0 {
			return fail("resource drift is not allowed")
		}
	}
	expected := expectedResources()
	changes, e := arr(root["resource_changes"], "resource_changes")
	if e != nil {
		return Review{}, e
	}
	if len(changes) != 14 {
		return fail("plan must contain exactly 14 initial resources")
	}
	seen := map[string]bool{}
	addresses := []string{}
	policies := 0
	for _, v := range changes {
		r, e := obj(v, "resource_change")
		if e != nil {
			return Review{}, e
		}
		address, e := stringAt(r, "address")
		if e != nil {
			return Review{}, e
		}
		t, ok := expected[address]
		if !ok || seen[address] || r["mode"] != "managed" || r["type"] != t || r["name"] != "this" {
			return fail("foreign, duplicate or wrong-typed address: " + address)
		}
		seen[address] = true
		c, e := obj(r["change"], "change")
		if e != nil {
			return Review{}, e
		}
		a, e := action(c)
		if e != nil || a != "create" {
			return fail("non-create resource action: " + address)
		}
		if before, exists := c["before"]; exists && before != nil {
			return fail("initial creation has existing before value")
		}
		if rp, ok := c["replace_paths"]; ok && rp != nil {
			paths, e := arr(rp, "replace_paths")
			if e != nil || len(paths) != 0 {
				return fail("replace_paths forbidden")
			}
		}
		after, e := obj(c["after"], "after")
		if e != nil {
			return Review{}, e
		}
		if e = checkResource(address, after); e != nil {
			return Review{}, fmt.Errorf("%s: %w", address, e)
		}
		addresses = append(addresses, address)
		if t == "kubernetes_network_policy_v1" {
			policies++
		}
	}
	if len(seen) != 14 || policies != 2 {
		return fail("resource inventory incomplete")
	}
	outputs, e := obj(root["output_changes"], "output_changes")
	if e != nil {
		return Review{}, e
	}
	if len(outputs) != 4 {
		return fail("unexpected output set")
	}
	for _, name := range []string{"active_profile", "owned_namespaces", "services", "managed_ingress_policies"} {
		o, e := obj(outputs[name], "output change")
		if e != nil {
			return Review{}, e
		}
		if nested, ok := o["change"]; ok {
			o, e = obj(nested, "nested output change")
			if e != nil {
				return Review{}, e
			}
		}
		a, e := action(o)
		if e != nil || a != "create" {
			return fail("output not newly created: " + name)
		}
		if before, ok := o["before"]; ok && before != nil {
			return fail("unexpected output before value")
		}
		if name == "active_profile" && o["after"] != "hardened" {
			return fail("output active_profile not hardened")
		}
	}
	sort.Strings(addresses)
	return Review{SchemaVersion: Schema, Scenario: "FD-004", Status: Status, Cluster: cluster, Context: "kind-" + cluster, Profile: "hardened", ResourceCount: 14, PolicyCount: 2, Addresses: addresses, ApplyAuthorized: false, TerraformAuthenticated: false, ClusterChecked: false, RecoveryProven: false, Limitations: "Offline plan-shape review only; neither provider provenance nor live cluster identity, source HCL, state origin, applyability, security enforcement or restoration is proven. DO NOT APPLY. A separate guarded live plan runner must verify identity, state isolation, sources and plan hash."}, nil
}
