// Package kubeinspect reads an explicitly owned local Kubernetes testbed.
// Inventory fields describe observed configuration, NOT effective permissions,
// policy enforcement, or compliant security posture.
package kubeinspect

import "time"

const (
	Cluster = "firedoor-local"
	Context = "kind-firedoor-local"
	Schema  = "firedoor.v02.kube-inventory.v1"
)

var zones = []string{"edge", "app", "data"}

type Snapshot struct {
	SchemaVersion string    `json:"schema_version"`
	CapturedAtUTC time.Time `json:"captured_at_utc"`
	Cluster       string    `json:"cluster"`
	Context       string    `json:"context"`
	Zones         []Zone    `json:"zones"`
	Limitations   string    `json:"limitations"`
}

type Zone struct {
	Name            string        `json:"name"`
	Namespace       string        `json:"namespace"`
	Deployment      Deployment    `json:"deployment"`
	ServiceAccount  Account       `json:"service_account"`
	Pods            []Pod         `json:"pods"`
	NetworkPolicies []Policy      `json:"network_policies"`
	Roles           []Role        `json:"roles"`
	RoleBindings    []RoleBinding `json:"role_bindings"`
}

type Deployment struct {
	Name               string      `json:"name"`
	DesiredReplicas    int32       `json:"desired_replicas"`
	ReadyReplicas      int32       `json:"ready_replicas"`
	ServiceAccountName string      `json:"service_account_name"`
	AutomountToken     *bool       `json:"automount_token,omitempty"`
	PodSecurityContext Security    `json:"pod_security_context"`
	Containers         []Container `json:"containers"`
}

type Security struct {
	RunAsNonRoot             *bool  `json:"run_as_non_root,omitempty"`
	RunAsUser                *int64 `json:"run_as_user,omitempty"`
	AllowPrivilegeEscalation *bool  `json:"allow_privilege_escalation,omitempty"`
	Privileged               *bool  `json:"privileged,omitempty"`
}

type Container struct {
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	Security Security `json:"security_context"`
}

type Account struct {
	Name           string `json:"name"`
	AutomountToken *bool  `json:"automount_token,omitempty"`
}

type Pod struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Phase  string            `json:"phase"`
	Ready  bool              `json:"ready"`
}

type Policy struct {
	Name        string        `json:"name"`
	PodSelector LabelSelector `json:"pod_selector"`
	PolicyTypes []string      `json:"policy_types"`
	Ingress     []PolicyRule  `json:"ingress"`
	Egress      []PolicyRule  `json:"egress"`
}

type LabelSelector struct {
	MatchLabels      map[string]string `json:"match_labels,omitempty"`
	MatchExpressions []SelectorExpr    `json:"match_expressions,omitempty"`
}

type SelectorExpr struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

type PolicyRule struct {
	Peers []Peer `json:"peers,omitempty"`
	Ports []Port `json:"ports,omitempty"`
}

type Peer struct {
	NamespaceSelector *LabelSelector `json:"namespace_selector,omitempty"`
	PodSelector       *LabelSelector `json:"pod_selector,omitempty"`
	IPBlock           *IPBlock       `json:"ip_block,omitempty"`
}

type IPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

type Port struct {
	Protocol *string `json:"protocol,omitempty"`
	Port     any     `json:"port,omitempty"`
	EndPort  *int32  `json:"end_port,omitempty"`
}

type Role struct {
	Name  string `json:"name"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	APIGroups       []string `json:"api_groups"`
	Resources       []string `json:"resources"`
	ResourceNames   []string `json:"resource_names"`
	Verbs           []string `json:"verbs"`
	NonResourceURLs []string `json:"non_resource_urls"`
}

type RoleBinding struct {
	Name     string    `json:"name"`
	RoleRef  RoleRef   `json:"role_ref"`
	Subjects []Subject `json:"subjects"`
}

type RoleRef struct {
	APIGroup string `json:"api_group"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

type Subject struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}
