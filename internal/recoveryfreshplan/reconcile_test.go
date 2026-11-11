package recoveryfreshplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSessionForTest() (Session, Manifest) {
	manifest := Manifest{Cluster: Cluster, Context: Context, Profile: "hardened"}
	manifest.FreshPath = filepath.Join(os.TempDir(), "Firedoor", "live-recovery-fresh-plans", "20261111-183950682-af260cea2bed")
	s := Session{SchemaVersion: FreshRunSchema, Status: FreshRunStatus, Cluster: Cluster, Context: Context, Profile: "hardened",
		SourceStageManifestSHA256: strings.Repeat("a", 64), SourceStagePath: manifest.FreshPath,
		OriginalBootstrapReconciliationSHA256: DefaultPins.Bootstrap, HistoricalPlanReconciliationSHA256: DefaultPins.Plan,
		SavedPlanSHA256: strings.Repeat("b", 64), PlanJSONSHA256: strings.Repeat("c", 64), AdmissionSHA256: strings.Repeat("d", 64),
		ResourceCount: 14, NetworkPolicyCount: 2, LiveClusterChecked: true, TerraformPlanExecuted: true,
		CreatedUTC: "2026-11-11T19:09:05Z", Limitations: "state not verified"}
	return s, manifest
}
func TestSessionRejectsForgedStateAndApplyClaims(t *testing.T) {
	s, m := validSessionForTest()
	if err := validateSession(s, m, "run", m.FreshPath, strings.Repeat("e", 64), strings.Repeat("b", 64), strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		do   func(*Session)
	}{
		{"state ownership", func(s *Session) { s.AuthoritativeStateOwnershipVerified = true }},
		{"state bound", func(s *Session) { s.FreshPlanBoundToAuthoritativeState = true }},
		{"apply authorized", func(s *Session) { s.ApplyAuthorized = true }},
		{"apply happened", func(s *Session) { s.ApplyExecuted = true }},
		{"recovery proven", func(s *Session) { s.NetworkRecoveryProven = true }},
		{"plan missing", func(s *Session) { s.TerraformPlanExecuted = false }},
		{"foreign cluster", func(s *Session) { s.Cluster = "firedoor-local" }},
		{"different plan hash", func(s *Session) { s.SavedPlanSHA256 = strings.Repeat("0", 64) }},
		{"wrong count", func(s *Session) { s.ResourceCount = 13 }},
		{"old source", func(s *Session) { s.SourceStagePath = "C:\\untrusted" }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			copy := s
			tt.do(&copy)
			if err := validateSession(copy, m, "run", m.FreshPath, strings.Repeat("e", 64), strings.Repeat("b", 64), strings.Repeat("a", 64)); err == nil {
				t.Fatal("accepted forged record")
			}
		})
	}
}
func TestAdmissionRejectsChanges(t *testing.T) {
	s, _ := validSessionForTest()
	a := AdmissionSummary{SchemaVersion: "firedoor.v10.fd004-retained-plan-admission.v1", Status: "retained_hardened_initial_plan_shape_reviewed_no_apply", Cluster: Cluster, Context: Context, Profile: "hardened", ResourcesReviewed: 14, HardenedPoliciesReviewed: 2, PlanSHA256: s.PlanJSONSHA256, ReconciliationSHA256: DefaultPins.Bootstrap, HistoricalEvidenceHashMatched: true}
	if err := validateAdmission(a, s); err != nil {
		t.Fatal(err)
	}
	a.ApplyAuthorized = true
	if err := validateAdmission(a, s); err == nil {
		t.Fatal("accepted apply authorization")
	}
	a.ApplyAuthorized = false
	a.PlanSHA256 = strings.Repeat("f", 64)
	if err := validateAdmission(a, s); err == nil {
		t.Fatal("accepted unrelated plan")
	}
}
func TestForeignPrivateRunRejected(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "Firedoor")
	stage := filepath.Join(base, "live-recovery-fresh-plans", "20261111-183950682-af260cea2bed")
	run := filepath.Join(base, "live-recovery-fresh-plan-runs", "20261111-190904908-c7f196bb8eb7")
	if err := checkRunPaths(filepath.Join(base, "other", filepath.Base(run)), stage); err == nil {
		t.Fatal("foreign run accepted")
	}
	if err := checkRunPaths(run, filepath.Join(base, "other", filepath.Base(stage))); err == nil {
		t.Fatal("foreign source accepted")
	}
}
func TestRunInventoryRejectsStateAndUnexpectedFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := Manifest{SourceSHA256: map[string]string{}, TFVarsSHA256: strings.Repeat("a", 64)}
	for _, n := range sourceFiles {
		m.SourceSHA256[n] = strings.Repeat("a", 64)
		mk(strings.TrimPrefix(n, "infrastructure/"))
	}
	for _, n := range []string{"terraform/testbed/terraform.tfvars.json", "initial-hardened.tfplan", "initial-hardened-plan.json", "fresh-plan-admission.v1.json", "fresh-plan-session.v1.json"} {
		mk(n)
	}
	if err := checkPlanSourceInventory(dir, m); err == nil {
		t.Fatal("accepted bad source fingerprint")
	}
	// Source files must be read from their exact pinned hashes, not simply counted.
	mk("terraform/testbed/terraform.tfstate")
	if err := checkPlanSourceInventory(dir, m); err == nil {
		t.Fatal("accepted state or altered source")
	}
}

func TestSyntheticPrivateReconciliationReadOnly(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	base := filepath.Join(local, "Firedoor")
	stage := filepath.Join(base, "live-recovery-fresh-plans", "20261111-183950682-af260cea2bed")
	run := filepath.Join(base, "live-recovery-fresh-plan-runs", "20261111-190904908-c7f196bb8eb7")
	for _, d := range []string{stage, run} {
		if err := os.MkdirAll(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, data []byte) string {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		v := sha256.Sum256(data)
		return hex.EncodeToString(v[:])
	}
	source := []byte("test source")
	manifest := Manifest{SchemaVersion: Schema, Status: Status, Cluster: Cluster, Context: Context, Profile: "hardened", RepoCommit: MergedSourceCommit, SourceCommit: OriginalSourceCommit, FreshPath: stage, BootstrapReconciliationSHA256: DefaultPins.Bootstrap, RetainedPlanReconciliationSHA256: DefaultPins.Plan, SourceSHA256: map[string]string{}}
	for _, n := range sourceFiles {
		manifest.SourceSHA256[n] = write(filepath.Join(run, filepath.FromSlash(strings.TrimPrefix(n, "infrastructure/"))), source)
	}
	manifest.TFVarsSHA256 = write(filepath.Join(run, "terraform", "testbed", "terraform.tfvars.json"), []byte("{}"))
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := write(filepath.Join(stage, "manifest.json"), manifestRaw)
	planHash := write(filepath.Join(run, "initial-hardened.tfplan"), []byte(strings.Repeat("p", 200)))
	planJSONHash := write(filepath.Join(run, "initial-hardened-plan.json"), []byte(`{"kind":"synthetic"}`))
	admission := AdmissionSummary{SchemaVersion: "firedoor.v10.fd004-retained-plan-admission.v1", Status: "retained_hardened_initial_plan_shape_reviewed_no_apply", Cluster: Cluster, Context: Context, Profile: "hardened", ResourcesReviewed: 14, HardenedPoliciesReviewed: 2, PlanSHA256: planJSONHash, ReconciliationSHA256: DefaultPins.Bootstrap, HistoricalEvidenceHashMatched: true}
	admissionRaw, err := json.Marshal(admission)
	if err != nil {
		t.Fatal(err)
	}
	admissionHash := write(filepath.Join(run, "fresh-plan-admission.v1.json"), admissionRaw)
	session := Session{SchemaVersion: FreshRunSchema, Status: FreshRunStatus, Cluster: Cluster, Context: Context, Profile: "hardened", SourceStageManifestSHA256: manifestHash, SourceStagePath: stage, OriginalBootstrapReconciliationSHA256: DefaultPins.Bootstrap, HistoricalPlanReconciliationSHA256: DefaultPins.Plan, SavedPlanSHA256: planHash, PlanJSONSHA256: planJSONHash, AdmissionSHA256: admissionHash, ResourceCount: 14, NetworkPolicyCount: 2, LiveClusterChecked: true, TerraformPlanExecuted: true, CreatedUTC: "2026-11-11T19:09:05Z", Limitations: "non-authoritative planning only"}
	sessionRaw, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	sessionHash := write(filepath.Join(run, "fresh-plan-session.v1.json"), sessionRaw)
	report, err := ReconcileFreshRun(run, stage, sessionHash, planHash, manifestHash)
	if err != nil {
		t.Fatalf("synthetic read-only reconciliation: %v", err)
	}
	if report.ApplyAuthorized || report.CurrentStateOwnershipVerified || report.PlanBoundToAuthoritativeState || report.RecoveryProven || report.SourceFilesVerified != 12 {
		t.Fatal("unsafe synthetic report")
	}
	write(filepath.Join(run, "terraform", "testbed", "terraform.tfstate"), []byte("{}"))
	if _, err := ReconcileFreshRun(run, stage, sessionHash, planHash, manifestHash); err == nil {
		t.Fatal("accepted an unexpected managed state file")
	}
}
