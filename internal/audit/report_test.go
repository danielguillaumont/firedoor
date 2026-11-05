package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/controls"
	"github.com/danielguillaumont/firedoor/internal/kubeinspect"
)

func ptrBool(b bool) *bool  { return &b }
func ptrInt(v int64) *int64 { return &v }

func sampleOwnedSnapshot() kubeinspect.Snapshot {
	s := kubeinspect.Snapshot{
		SchemaVersion: kubeinspect.Schema, Cluster: kubeinspect.Cluster,
		Context: kubeinspect.Context, CapturedAtUTC: time.Date(2026, 11, 5, 1, 30, 0, 0, time.UTC),
	}
	for _, zone := range []string{"edge", "app", "data"} {
		ns := "firedoor-" + zone
		z := kubeinspect.Zone{
			Name: zone, Namespace: ns,
			Deployment: kubeinspect.Deployment{
				Name: ns, DesiredReplicas: 1, ReadyReplicas: 1,
				ServiceAccountName: ns + "-sa", AutomountToken: ptrBool(false),
				Containers: []kubeinspect.Container{{Name: "demo-http", Security: kubeinspect.Security{
					RunAsNonRoot: ptrBool(true), RunAsUser: ptrInt(10001),
					AllowPrivilegeEscalation: ptrBool(false), Privileged: ptrBool(false),
				}}},
			},
			ServiceAccount: kubeinspect.Account{Name: ns + "-sa", AutomountToken: ptrBool(false)},
			Pods: []kubeinspect.Pod{{Name: ns + "-pod", Ready: true, Phase: "Running", Labels: map[string]string{
				"app.kubernetes.io/name": ns, "firedoor.dev/zone": zone,
			}}},
		}
		if zone != "edge" {
			allowed := "edge"
			if zone == "data" {
				allowed = "app"
			}
			z.NetworkPolicies = []kubeinspect.Policy{{Name: ns + "-ingress", PolicyTypes: []string{"Ingress"},
				PodSelector: kubeinspect.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": ns}},
				Ingress: []kubeinspect.PolicyRule{{Peers: []kubeinspect.Peer{{NamespaceSelector: &kubeinspect.LabelSelector{
					MatchLabels: map[string]string{"firedoor.dev/zone": allowed},
				}}}}},
			}}
		}
		s.Zones = append(s.Zones, z)
	}
	return s
}

func TestBuildReportHonestSummaryAndJSON(t *testing.T) {
	r, err := Build(sampleOwnedSnapshot(), "hardened")
	if err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != Schema || r.Cluster != "firedoor-local" || r.ProfileSource != "operator_supplied_unverified" {
		t.Fatalf("bad scope/provenance: %+v", r)
	}
	s := r.Summary
	if s.Total != 18 || s.Pass != 14 || s.Fail != 1 || s.Unknown != 3 || s.NotApplicable != 0 || s.Assessed != 15 {
		t.Fatalf("inaccurate summary: %+v", s)
	}
	if s.AssessedPassRatePercent != 93.3 || s.AssessmentCoveragePct != 83.3 || s.Status != "findings_require_review" {
		t.Fatalf("wrong score/coverage/disposition: %+v", s)
	}
	for _, f := range r.Findings {
		if err := f.Validate(); err != nil {
			t.Fatalf("invalid finding: %v", err)
		}
	}
	var decoded Report
	payload, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.Unknown != 3 || decoded.Findings[0].Control.ID != "CTRL-ID-001" {
		t.Fatalf("json lost evidence: %+v", decoded)
	}
	t.Logf("Synthetic test only: pass=%d fail=%d unknown=%d; no live cluster API called", s.Pass, s.Fail, s.Unknown)
}

func TestHumanReportDoesNotClaimFullCompliance(t *testing.T) {
	r, err := Build(sampleOwnedSnapshot(), "hardened")
	if err != nil {
		t.Fatal(err)
	}
	s := Human(r)
	for _, required := range []string{"READ ONLY", "UNKNOWN", "FAIL", "firedoor-edge", "firedoor-app", "firedoor-data", "operator supplied", "NOT a security score", "ClusterRoleBindings", "evidence"} {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(required)) {
			t.Fatalf("report omits %q", required)
		}
	}
}

func TestRejectInvalidSnapshotAndProfile(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*kubeinspect.Snapshot)
		profile string
	}{
		{"wrong cluster", func(s *kubeinspect.Snapshot) { s.Cluster = "production" }, "hardened"},
		{"missing zone", func(s *kubeinspect.Snapshot) { s.Zones = s.Zones[:2] }, "hardened"},
		{"bad timestamp", func(s *kubeinspect.Snapshot) { s.CapturedAtUTC = time.Time{} }, "hardened"},
		{"unverified profile name", func(s *kubeinspect.Snapshot) {}, "production"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := sampleOwnedSnapshot()
			tc.change(&s)
			if _, err := Build(s, tc.profile); err == nil {
				t.Fatal("accepted invalid audit input")
			}
		})
	}
}

func TestUncertaintyCannotBecomePassRate100(t *testing.T) {
	s := sampleOwnedSnapshot()
	// Add an Ingress policy to edge; all assessed results then pass, but RBAC
	// still remains unknown and must make the report incomplete.
	s.Zones[0].NetworkPolicies = []kubeinspect.Policy{{Name: "edge-ingress", PolicyTypes: []string{"Ingress"},
		PodSelector: kubeinspect.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "firedoor-edge"}},
		Ingress:     nil,
	}}
	r, err := Build(s, "hardened")
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary.Fail != 0 || r.Summary.Unknown != 3 || r.Summary.Status != "incomplete_assessment" || r.Summary.AssessmentCoveragePct != 83.3 {
		t.Fatalf("unknown was misrepresented as clean: %+v", r.Summary)
	}
	for _, f := range r.Findings {
		if f.Control.ID == "CTRL-ID-003" && f.Verdict != controls.Unknown {
			t.Fatalf("RBAC falsely passed: %+v", f)
		}
	}
}
