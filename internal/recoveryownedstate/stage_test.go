package recoveryownedstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/recoveryfreshplan"
)

func writeStageFixture(t *testing.T, name string, content []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, content, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}
func stageFixture(t *testing.T) (local, fresh, owned, hash string) {
	t.Helper()
	local = filepath.Join(t.TempDir(), "Local")
	private := filepath.Join(local, "Firedoor")
	fresh = filepath.Join(private, "live-recovery-fresh-plans", FixedFreshLeaf)
	owned = filepath.Join(private, CandidateLeaf, "20261111-210000000-112233aabbcc")
	if err := os.MkdirAll(owned, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fresh, 0700); err != nil {
		t.Fatal(err)
	}
	files := recoveryfreshplan.SourceFiles()
	pins := map[string]string{}
	for _, name := range files {
		rel := strings.TrimPrefix(name, "infrastructure/")
		content := []byte("file: " + name + "\n")
		pins[name] = writeStageFixture(t, filepath.Join(fresh, filepath.FromSlash(rel)), content)
		writeStageFixture(t, filepath.Join(owned, filepath.FromSlash(rel)), content)
	}
	varshash := writeStageFixture(t, filepath.Join(fresh, "terraform", "testbed", "terraform.tfvars.json"), []byte(`{"security_profile":"hardened"}`))
	writeStageFixture(t, filepath.Join(owned, "terraform", "testbed", "terraform.tfvars.json"), []byte(`{"security_profile":"hardened"}`))
	old := recoveryfreshplan.Manifest{
		SchemaVersion: recoveryfreshplan.Schema, Status: recoveryfreshplan.Status, Cluster: Cluster,
		Context: "kind-" + Cluster, Profile: "hardened", RepoCommit: recoveryfreshplan.MergedSourceCommit,
		FreshPath: fresh, SourceSHA256: pins, TFVarsSHA256: varshash,
	}
	blob, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	hash = writeStageFixture(t, filepath.Join(fresh, "manifest.json"), blob)
	m := StageManifest{
		SchemaVersion: StageSchema, Status: StageStatus, Cluster: Cluster, Context: "kind-" + Cluster,
		Profile: "hardened", SourceStagePath: fresh, OwnedStagePath: owned, SourceStageManifestSHA256: hash,
		PreviousPlanSessionSHA256: "7a6264d08998ff48aa329a59abe7b223fdc72079871ddb97503238983fc6532d",
		PreviousPlanSHA256:        "dd1e9e321484cb71236a6594f53f73041d2da00e0584582d3a05f382da59fb9e",
		SourceSHA256:              pins, TFVarsSHA256: varshash,
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeStageFixture(t, filepath.Join(owned, "manifest.json"), raw)
	return
}
func TestSourceStagePositive(t *testing.T) {
	local, fresh, owned, hash := stageFixture(t)
	report, err := VerifyStaged(local, fresh, owned, hash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.StageVerified || !report.DeploymentBlocked || report.SourceFilesVerified != 12 || report.ApplyAuthorized || report.StateBackendInitialized || report.StateOwnershipVerified {
		t.Fatalf("unsafe report: %+v", report)
	}
}
func TestSourceStageRejectsTampering(t *testing.T) {
	cases := []struct {
		name   string
		tamper func(*testing.T, string, string, string)
	}{
		{"state file", func(t *testing.T, _, owned string, _ string) {
			writeStageFixture(t, filepath.Join(owned, "terraform", "testbed", "terraform.tfstate"), []byte("{}"))
		}},
		{"saved plan", func(t *testing.T, _, owned string, _ string) {
			writeStageFixture(t, filepath.Join(owned, "initial-hardened.tfplan"), []byte("not a plan"))
		}},
		{"source change", func(t *testing.T, _, owned string, _ string) {
			writeStageFixture(t, filepath.Join(owned, "terraform", "testbed", "main.tf"), []byte("changed"))
		}},
		{"foreign previous stage", func(t *testing.T, fresh, owned string, _ string) {
			_ = fresh
			raw, err := os.ReadFile(filepath.Join(owned, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var m StageManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			m.Cluster = "firedoor-local"
			newRaw, _ := json.Marshal(m)
			writeStageFixture(t, filepath.Join(owned, "manifest.json"), newRaw)
		}},
		{"forged approval", func(t *testing.T, _, owned string, _ string) {
			raw, err := os.ReadFile(filepath.Join(owned, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var m StageManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			m.ApplyAuthorized = true
			newRaw, _ := json.Marshal(m)
			writeStageFixture(t, filepath.Join(owned, "manifest.json"), newRaw)
		}},
		{"provider directory", func(t *testing.T, _, owned string, _ string) {
			if err := os.MkdirAll(filepath.Join(owned, "terraform", "testbed", ".terraform"), 0700); err != nil {
				t.Fatal(err)
			}
			writeStageFixture(t, filepath.Join(owned, "terraform", "testbed", ".terraform", "x"), []byte("x"))
		}},
		{"extra file", func(t *testing.T, _, owned string, _ string) {
			writeStageFixture(t, filepath.Join(owned, "surprise.txt"), []byte("x"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local, fresh, owned, hash := stageFixture(t)
			tc.tamper(t, fresh, owned, hash)
			if _, err := VerifyStaged(local, fresh, owned, hash, nil); err == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
}
func TestSourceStageRejectsOverridesAndLinks(t *testing.T) {
	local, fresh, owned, hash := stageFixture(t)
	if _, err := VerifyStaged(local, fresh, owned, hash, []string{"TF_DATA_DIR=foo"}); err == nil {
		t.Fatal("terraform override accepted")
	}
	if _, err := VerifyStaged(local, fresh, owned, hash, []string{"KUBECONFIG=foo"}); err == nil {
		t.Fatal("kubeconfig override accepted")
	}
	if _, err := VerifyStaged(local, fresh, filepath.Join(local, "bad"), hash, nil); err == nil {
		t.Fatal("foreign owned root accepted")
	}
	if err := os.Symlink(filepath.Join(owned, "terraform", "testbed", "main.tf"), filepath.Join(owned, "link.tf")); err == nil {
		if _, err := VerifyStaged(local, fresh, owned, hash, nil); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}
