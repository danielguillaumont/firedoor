// Package destroyreview validates only the shape of an isolated FD-004
// Terraform cleanup plan. It never authenticates Kubernetes or authorizes apply.
package destroyreview

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

const (
	Limit  = 8 << 20
	Schema = "firedoor.v10.fd004-disposable-destroy-plan.v1"
	Status = "isolated_hardened_cleanup_shape_reviewed_no_execution_authorization"
)

var clusterPattern = regexp.MustCompile(`^firedoor-rh-[a-f0-9]{12}$`)

type Review struct {
	SchemaVersion        string   `json:"schema_version"`
	Scenario             string   `json:"scenario"`
	Status               string   `json:"status"`
	Cluster              string   `json:"cluster"`
	Context              string   `json:"context"`
	Profile              string   `json:"profile"`
	ResourceCount        int      `json:"resource_count"`
	PolicyCount          int      `json:"hardened_policy_count"`
	Addresses            []string `json:"reviewed_addresses"`
	DestroyAuthorized    bool     `json:"destroy_authorized"`
	ClusterAuthenticated bool     `json:"cluster_authenticated"`
	StateAuthenticated   bool     `json:"state_authenticated"`
	RecoveryProven       bool     `json:"recovery_proven"`
	Limitations          string   `json:"limitations"`
}

// Strictly decode JSON with duplicate-key detection, including nested objects.
func value(d *json.Decoder) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '{':
		m := make(map[string]any)
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, errors.New("non-string JSON key")
			}
			if _, exists := m[key]; exists {
				return nil, fmt.Errorf("duplicate JSON key %q", key)
			}
			v, err := value(d)
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errors.New("unclosed object")
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			v, err := value(d)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errors.New("unclosed array")
		}
		return a, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}
func object(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok }
func array(v any) ([]any, bool)           { a, ok := v.([]any); return a, ok }
func one(v any) (map[string]any, bool) {
	a, ok := array(v)
	if !ok || len(a) != 1 {
		return nil, false
	}
	return object(a[0])
}
func action(v any) (string, bool) {
	a, ok := array(v)
	if !ok || len(a) != 1 {
		return "", false
	}
	s, ok := a[0].(string)
	return s, ok
}

func expected() map[string]string {
	m := map[string]string{}
	for _, z := range []string{"edge", "app", "data"} {
		for _, kind := range []string{"kubernetes_namespace_v1", "kubernetes_service_account_v1", "kubernetes_deployment_v1", "kubernetes_service_v1"} {
			m[fmt.Sprintf(`module.workloads[%q].%s.this`, z, kind)] = kind
		}
	}
	for _, z := range []string{"app", "data"} {
		m[fmt.Sprintf(`module.network_boundaries[%q].kubernetes_network_policy_v1.this`, z)] = "kubernetes_network_policy_v1"
	}
	return m
}
func checkMetadata(address, kind string, before map[string]any) error {
	meta, ok := one(before["metadata"])
	if !ok {
		return errors.New("resource metadata must have one entry")
	}
	zone := ""
	for _, z := range []string{"edge", "app", "data"} {
		if strings.Contains(address, `["`+z+`"]`) {
			zone = z
			break
		}
	}
	if zone == "" {
		return errors.New("unknown zone")
	}
	name := "firedoor-" + zone
	if kind == "kubernetes_service_account_v1" {
		name += "-sa"
	}
	if kind == "kubernetes_network_policy_v1" {
		name += "-ingress"
	}
	if meta["name"] != name {
		return errors.New("unexpected resource name")
	}
	if kind != "kubernetes_namespace_v1" && meta["namespace"] != "firedoor-"+zone {
		return errors.New("unexpected resource namespace")
	}
	labs, ok := object(meta["labels"])
	if !ok {
		return errors.New("missing resource labels")
	}
	want := map[string]string{"app.kubernetes.io/part-of": "firedoor", "firedoor.dev/managed-by": "terraform"}
	if kind != "kubernetes_network_policy_v1" {
		want["app.kubernetes.io/name"] = "firedoor-" + zone
		want["firedoor.dev/zone"] = zone
	}
	if kind == "kubernetes_namespace_v1" {
		want["pod-security.kubernetes.io/enforce"] = "baseline"
	}
	if len(labs) != len(want) {
		return errors.New("metadata labels differ")
	}
	for k, v := range want {
		if labs[k] != v {
			return fmt.Errorf("metadata label %s differs", k)
		}
	}
	if kind == "kubernetes_network_policy_v1" {
		spec, ok := one(before["spec"])
		if !ok {
			return errors.New("missing policy spec")
		}
		types, ok := array(spec["policy_types"])
		if !ok || len(types) != 1 || types[0] != "Ingress" {
			return errors.New("unsafe policy types")
		}
		pod, ok := one(spec["pod_selector"])
		if !ok {
			return errors.New("missing pod selector")
		}
		pl, ok := object(pod["match_labels"])
		if !ok || len(pl) != 1 || pl["app.kubernetes.io/name"] != "firedoor-"+zone {
			return errors.New("wrong policy pod selector")
		}
		ing, ok := one(spec["ingress"])
		if !ok {
			return errors.New("wrong policy ingress")
		}
		from, ok := one(ing["from"])
		if !ok {
			return errors.New("wrong policy source")
		}
		ns, ok := one(from["namespace_selector"])
		if !ok {
			return errors.New("wrong namespace selector")
		}
		nl, ok := object(ns["match_labels"])
		if !ok || len(nl) != 1 {
			return errors.New("unsafe namespace selector")
		}
		src := "edge"
		if zone == "data" {
			src = "app"
		}
		if nl["firedoor.dev/zone"] != src {
			return errors.New("policy is not hardened")
		}
		ports, ok := one(ing["ports"])
		if !ok || ports["port"] != "8080" || ports["protocol"] != "TCP" {
			return errors.New("unsafe policy port")
		}
	}
	return nil
}

// ReviewJSON is an OFFLINE delete-only shape check. Caller must independently
// authenticate the live disposable Docker/KIND cluster, private kubeconfig,
// fresh private Terraform state, provider, and exact saved plan before cleanup.
func ReviewJSON(raw []byte, cluster, kubeconfig string) (Review, error) {
	reject := func(m string) (Review, error) { return Review{}, errors.New(m) }
	if !clusterPattern.MatchString(cluster) {
		return reject("invalid isolated cluster name")
	}
	if kubeconfig == "" || strings.ContainsAny(kubeconfig, "\r\n\x00~") {
		return reject("invalid private kubeconfig path")
	}
	if len(raw) == 0 || len(raw) > Limit || bytes.IndexByte(raw, 0) >= 0 {
		return reject("plan JSON missing, oversized or contains NUL")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	rootValue, err := value(d)
	if err != nil {
		return Review{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err = d.Token(); err != io.EOF {
		return reject("trailing JSON data")
	}
	root, ok := object(rootValue)
	if !ok {
		return reject("plan root is not object")
	}
	version, ok := root["format_version"].(string)
	if !ok || !strings.HasPrefix(version, "1.") {
		return reject("unsupported Terraform JSON format")
	}
	vars, ok := object(root["variables"])
	if !ok || len(vars) != 3 {
		return reject("unexpected Terraform variables")
	}
	for k, want := range map[string]string{"security_profile": "hardened", "kube_context": "kind-" + cluster, "kubeconfig_path": kubeconfig} {
		v, ok := object(vars[k])
		if !ok || v["value"] != want {
			return reject("Terraform variable mismatch: " + k)
		}
	}
	if drift, exists := root["resource_drift"]; exists && drift != nil {
		a, ok := array(drift)
		if !ok || len(a) != 0 {
			return reject("resource drift present")
		}
	}
	changes, ok := array(root["resource_changes"])
	if !ok || len(changes) != 14 {
		return reject("expected exactly 14 resource deletions")
	}
	allowed := expected()
	seen := map[string]bool{}
	paths := []string{}
	policies := 0
	for _, item := range changes {
		res, ok := object(item)
		if !ok {
			return reject("invalid resource change")
		}
		address, ok := res["address"].(string)
		if !ok {
			return reject("resource address missing")
		}
		kind, allowedAddress := allowed[address]
		if !allowedAddress || seen[address] || res["mode"] != "managed" || res["type"] != kind || res["name"] != "this" {
			return reject("foreign, duplicated, or mistyped resource: " + address)
		}
		seen[address] = true
		change, ok := object(res["change"])
		if !ok {
			return reject("missing resource change")
		}
		a, ok := action(change["actions"])
		if !ok || a != "delete" {
			return reject("non-delete operation: " + address)
		}
		if after, exists := change["after"]; exists && after != nil {
			return reject("delete has non-nil after: " + address)
		}
		before, ok := object(change["before"])
		if !ok {
			return reject("delete missing before: " + address)
		}
		if err := checkMetadata(address, kind, before); err != nil {
			return Review{}, fmt.Errorf("%s: %w", address, err)
		}
		if rp, exists := change["replace_paths"]; exists && rp != nil {
			p, ok := array(rp)
			if !ok || len(p) != 0 {
				return reject("replacement paths forbidden")
			}
		}
		paths = append(paths, address)
		if kind == "kubernetes_network_policy_v1" {
			policies++
		}
	}
	if len(seen) != 14 || policies != 2 {
		return reject("cleanup resource inventory incomplete")
	}
	sort.Strings(paths)
	return Review{SchemaVersion: Schema, Scenario: "FD-004", Status: Status, Cluster: cluster, Context: "kind-" + cluster,
		Profile: "hardened", ResourceCount: len(paths), PolicyCount: policies, Addresses: paths,
		DestroyAuthorized: false, ClusterAuthenticated: false, StateAuthenticated: false, RecoveryProven: false,
		Limitations: "Offline plan-shape check only. It does NOT prove cluster identity, resource ownership, Terraform state provenance, live behavior or recovery. Do not apply this destroy plan without a separate authenticated live approval and cleanup runner."}, nil
}
