// Package controls defines read-only security inspection contracts.
// Definitions and findings are NOT proof that any Kubernetes control is enforced.
package controls

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// These verdicts describe inspected configuration, not network containment.
// They are distinct from the behavioral verdicts in pkg/models.
type Verdict string

const (
	Pass          Verdict = "pass"
	Fail          Verdict = "fail"
	Unknown       Verdict = "unknown"
	NotApplicable Verdict = "not_applicable"
)

func (v Verdict) Valid() bool {
	switch v {
	case Pass, Fail, Unknown, NotApplicable:
		return true
	default:
		return false
	}
}

type Applicability string

const (
	Applicable           Applicability = "applicable"
	Inapplicable         Applicability = "not_applicable"
	ApplicabilityUnknown Applicability = "unknown"
)

func (a Applicability) Valid() bool {
	switch a {
	case Applicable, Inapplicable, ApplicabilityUnknown:
		return true
	default:
		return false
	}
}

type Severity string

const (
	Info     Severity = "info"
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

func (s Severity) Valid() bool {
	switch s {
	case Info, Low, Medium, High, Critical:
		return true
	default:
		return false
	}
}

// Definition is a static control specification. It is not an observed finding.
type Definition struct {
	ID          string   `json:"id"`
	LegacyID    string   `json:"legacy_id"`
	Name        string   `json:"name"`
	Domain      string   `json:"domain"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
}

func (d Definition) Validate() error {
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.LegacyID) == "" ||
		strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Domain) == "" ||
		strings.TrimSpace(d.Description) == "" || !d.Severity.Valid() {
		return fmt.Errorf("incomplete control definition: %q", d.ID)
	}
	return nil
}

// ObjectReference identifies one Kubernetes object by namespace, kind, and name.
// Namespace is empty for cluster-scoped objects (for example, Namespace).
type ObjectReference struct {
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

func (r ObjectReference) Validate() error {
	if strings.TrimSpace(r.Kind) == "" || strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("Kubernetes object reference requires kind and name")
	}
	return nil
}

// TerraformReference relates an observation to IaC intent when a known mapping exists.
// It does not imply a drift check or prove behavioral enforcement.
type TerraformReference struct {
	ResourceAddress string `json:"resource_address"`
	Profile         string `json:"profile"`
}

func (r TerraformReference) Validate() error {
	if strings.TrimSpace(r.ResourceAddress) == "" ||
		(r.Profile != "baseline" && r.Profile != "hardened") {
		return fmt.Errorf("Terraform reference requires an address and known profile")
	}
	return nil
}

// Evidence records an actual inspection value and exactly where it came from.
// Evaluators must never fill Observed with a value inferred from Terraform intent.
type Evidence struct {
	Object        ObjectReference     `json:"object"`
	FieldPath     string              `json:"field_path"`
	Observed      string              `json:"observed"`
	Expected      string              `json:"expected"`
	CapturedAtUTC time.Time           `json:"captured_at_utc"`
	Terraform     *TerraformReference `json:"terraform,omitempty"`
}

func (e Evidence) Validate() error {
	if err := e.Object.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(e.FieldPath) == "" || strings.TrimSpace(e.Observed) == "" ||
		strings.TrimSpace(e.Expected) == "" || e.CapturedAtUTC.IsZero() {
		return fmt.Errorf("inspection evidence must include field, observed/expected values and timestamp")
	}
	if e.Terraform != nil {
		return e.Terraform.Validate()
	}
	return nil
}

// InspectionScope is the authorized, owned testbed and selected inspection profile.
// The interface alone does not perform authorization; live adapters must fail closed.
type InspectionScope struct {
	Cluster string          `json:"cluster"`
	Context string          `json:"context"`
	Profile string          `json:"profile"`
	Target  ObjectReference `json:"target"`
}

func (s InspectionScope) Validate() error {
	if s.Cluster != "firedoor-local" || s.Context != "kind-firedoor-local" ||
		(s.Profile != "baseline" && s.Profile != "hardened") {
		return fmt.Errorf("inspection requires dedicated Firedoor context, cluster and known profile")
	}
	return s.Target.Validate()
}

// Finding describes an inspection outcome; it never asserts network containment.
// Unknown and not-applicable results must have an explanation and cannot silently pass.
type Finding struct {
	Control       Definition      `json:"control"`
	Scope         InspectionScope `json:"scope"`
	Applicability Applicability   `json:"applicability"`
	Verdict       Verdict         `json:"verdict"`
	Reason        string          `json:"reason,omitempty"`
	Evidence      []Evidence      `json:"evidence,omitempty"`
}

func (f Finding) Validate() error {
	if err := f.Control.Validate(); err != nil {
		return err
	}
	if err := f.Scope.Validate(); err != nil {
		return err
	}
	if !f.Applicability.Valid() || !f.Verdict.Valid() {
		return fmt.Errorf("invalid applicability or verdict")
	}
	switch f.Applicability {
	case Applicable:
		if f.Verdict == NotApplicable {
			return fmt.Errorf("applicable control cannot have not_applicable verdict")
		}
	case Inapplicable:
		if f.Verdict != NotApplicable {
			return fmt.Errorf("inapplicable control must have not_applicable verdict")
		}
	case ApplicabilityUnknown:
		if f.Verdict != Unknown {
			return fmt.Errorf("unknown applicability must have unknown verdict")
		}
	}
	if (f.Verdict == Unknown || f.Verdict == NotApplicable) && strings.TrimSpace(f.Reason) == "" {
		return fmt.Errorf("unknown and not_applicable findings require a reason")
	}
	if (f.Verdict == Pass || f.Verdict == Fail) && len(f.Evidence) == 0 {
		return fmt.Errorf("pass/fail findings require observed evidence")
	}
	for i, e := range f.Evidence {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("evidence %d: %w", i, err)
		}
		// Related Kubernetes objects (ServiceAccounts, RoleBindings, namespaces)
		// may differ from the target. This is schema validation, not an
		// authorization or ownership check; live adapters must verify both.
		if e.Terraform != nil && e.Terraform.Profile != f.Scope.Profile {
			return fmt.Errorf("evidence %d has a different Terraform profile", i)
		}
	}
	return nil
}

// Evaluator is intentionally a pure contract in Slice 1.
// No Kubernetes API requests, writes, pods, or Terraform applies are implemented here.
type Evaluator interface {
	Definition() Definition
	Evaluate(ctx context.Context, scope InspectionScope) (Finding, error)
}

var foundationDefinitions = []Definition{
	{ID: "CTRL-ID-001", LegacyID: "C-001", Name: "Dedicated workload ServiceAccount", Domain: "identity", Severity: Medium, Description: "Inspect whether a workload uses its intended dedicated ServiceAccount."},
	{ID: "CTRL-ID-002", LegacyID: "C-002", Name: "Disable unnecessary token automount", Domain: "identity", Severity: High, Description: "Inspect workload token automount settings and applicable inherited defaults."},
	{ID: "CTRL-ID-003", LegacyID: "C-003", Name: "Least-privilege Kubernetes RBAC", Domain: "identity", Severity: High, Description: "Inspect scoped workload identity authorization without assuming absent permissions from missing bindings."},
	{ID: "CTRL-NET-001", LegacyID: "C-004", Name: "Default-deny network policy", Domain: "network", Severity: High, Description: "Inspect effective NetworkPolicy selection and policy types; a policy's presence alone does not prove enforcement."},
	{ID: "CTRL-WL-001", LegacyID: "C-006", Name: "Run as non-root", Domain: "workload", Severity: High, Description: "Inspect the effective Pod and container user security context."},
	{ID: "CTRL-WL-002", LegacyID: "C-007", Name: "Disable privilege escalation", Domain: "workload", Severity: High, Description: "Inspect allowPrivilegeEscalation for each applicable container."},
}

// FoundationDefinitions returns a defensive copy of the v0.2 target catalog.
// Catalog entries are specifications only; none have been evaluated yet.
func FoundationDefinitions() []Definition {
	out := make([]Definition, len(foundationDefinitions))
	copy(out, foundationDefinitions)
	return out
}
