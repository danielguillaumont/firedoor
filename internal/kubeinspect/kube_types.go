package kubeinspect

// Narrow Kubernetes API JSON wire types; only explicitly inspected fields are
// decoded. These are kept separate from Firedoor's stable output schema.
type kubeMetadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type kubeNamespace struct {
	Metadata kubeMetadata `json:"metadata"`
}

type kubeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type nodeList struct {
	Items []struct {
		Metadata kubeMetadata `json:"metadata"`
		Status   struct {
			Conditions []kubeCondition `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type kubeSecurity struct {
	RunAsNonRoot             *bool  `json:"runAsNonRoot"`
	RunAsUser                *int64 `json:"runAsUser"`
	AllowPrivilegeEscalation *bool  `json:"allowPrivilegeEscalation"`
	Privileged               *bool  `json:"privileged"`
}

func (s kubeSecurity) output() Security {
	return Security{RunAsNonRoot: s.RunAsNonRoot, RunAsUser: s.RunAsUser,
		AllowPrivilegeEscalation: s.AllowPrivilegeEscalation, Privileged: s.Privileged}
}

type kubeContainer struct {
	Name            string       `json:"name"`
	Image           string       `json:"image"`
	SecurityContext kubeSecurity `json:"securityContext"`
}

type kubeDeployment struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Replicas *int32 `json:"replicas"`
		Template struct {
			Spec struct {
				ServiceAccountName           string          `json:"serviceAccountName"`
				AutomountServiceAccountToken *bool           `json:"automountServiceAccountToken"`
				SecurityContext              kubeSecurity    `json:"securityContext"`
				Containers                   []kubeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas int32 `json:"readyReplicas"`
	} `json:"status"`
}

type kubeAccount struct {
	Metadata                     kubeMetadata `json:"metadata"`
	AutomountServiceAccountToken *bool        `json:"automountServiceAccountToken"`
}

type kubePodList struct {
	Items []struct {
		Metadata kubeMetadata `json:"metadata"`
		Status   struct {
			Phase      string          `json:"phase"`
			Conditions []kubeCondition `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type kubeSelectorExpr struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type kubeLabelSelector struct {
	MatchLabels      map[string]string  `json:"matchLabels"`
	MatchExpressions []kubeSelectorExpr `json:"matchExpressions"`
}

func (s kubeLabelSelector) output() LabelSelector {
	out := LabelSelector{MatchLabels: s.MatchLabels}
	for _, e := range s.MatchExpressions {
		out.MatchExpressions = append(out.MatchExpressions, SelectorExpr{Key: e.Key, Operator: e.Operator, Values: e.Values})
	}
	return out
}

type kubeIPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except"`
}

type kubePeer struct {
	NamespaceSelector *kubeLabelSelector `json:"namespaceSelector"`
	PodSelector       *kubeLabelSelector `json:"podSelector"`
	IPBlock           *kubeIPBlock       `json:"ipBlock"`
}

func (p kubePeer) output() Peer {
	var out Peer
	if p.NamespaceSelector != nil {
		v := p.NamespaceSelector.output()
		out.NamespaceSelector = &v
	}
	if p.PodSelector != nil {
		v := p.PodSelector.output()
		out.PodSelector = &v
	}
	if p.IPBlock != nil {
		out.IPBlock = &IPBlock{CIDR: p.IPBlock.CIDR, Except: p.IPBlock.Except}
	}
	return out
}

type kubePort struct {
	Protocol *string `json:"protocol"`
	Port     any     `json:"port"`
	EndPort  *int32  `json:"endPort"`
}

func (p kubePort) output() Port { return Port{Protocol: p.Protocol, Port: p.Port, EndPort: p.EndPort} }

type kubePolicyRule struct {
	From  []kubePeer `json:"from"`
	To    []kubePeer `json:"to"`
	Ports []kubePort `json:"ports"`
}

type kubePolicyList struct {
	Items []struct {
		Metadata kubeMetadata `json:"metadata"`
		Spec     struct {
			PodSelector kubeLabelSelector `json:"podSelector"`
			PolicyTypes []string          `json:"policyTypes"`
			Ingress     []kubePolicyRule  `json:"ingress"`
			Egress      []kubePolicyRule  `json:"egress"`
		} `json:"spec"`
	} `json:"items"`
}

type kubeRule struct {
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	ResourceNames   []string `json:"resourceNames"`
	Verbs           []string `json:"verbs"`
	NonResourceURLs []string `json:"nonResourceURLs"`
}

func (r kubeRule) output() Rule {
	return Rule{APIGroups: r.APIGroups, Resources: r.Resources, ResourceNames: r.ResourceNames, Verbs: r.Verbs, NonResourceURLs: r.NonResourceURLs}
}

type kubeRoleList struct {
	Items []struct {
		Metadata kubeMetadata `json:"metadata"`
		Rules    []kubeRule   `json:"rules"`
	} `json:"items"`
}

type kubeRoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

func (r kubeRoleRef) output() RoleRef {
	return RoleRef{APIGroup: r.APIGroup, Kind: r.Kind, Name: r.Name}
}

type kubeBindingList struct {
	Items []struct {
		Metadata kubeMetadata `json:"metadata"`
		RoleRef  kubeRoleRef  `json:"roleRef"`
		Subjects []Subject    `json:"subjects"`
	} `json:"items"`
}
