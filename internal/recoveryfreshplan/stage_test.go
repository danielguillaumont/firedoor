package recoveryfreshplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	repo, stage, fresh, planRec string
	manifest                    Manifest
	pins                        Pins
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
}
func put(t *testing.T, p string, b []byte) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func jsonPut(t *testing.T, p string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	put(t, p, b)
}
func hash(t *testing.T, p string) string {
	t.Helper()
	h, e := fingerprint(p)
	if e != nil {
		t.Fatal(e)
	}
	return h
}
func newFixture(t *testing.T) fixture {
	t.Helper()
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	base := filepath.Join(local, "Firedoor")
	repo := filepath.Join(t.TempDir(), "repo")
	stage := filepath.Join(base, "live-recovery-staging", StageLeaf)
	fresh := filepath.Join(base, "live-recovery-fresh-plans", "20261111-182501123-acde1234beef")
	bootstrapPath := filepath.Join(base, "live-recovery-bootstraps", StageLeaf, "bootstrap-reconciliation.v1.json")
	planPath := filepath.Join(base, "live-recovery-plans", "20261111-040926850-3c138b60a075", "retained-plan-reconciliation.v1.json")
	kube := filepath.Join(stage, "kubeconfig")
	put(t, kube, []byte("private mock kubeconfig"))
	srcHash := map[string]string{}
	for _, n := range append(SourceFiles(), kindSource) {
		data := []byte("# synthetic " + n + "\n")
		put(t, filepath.Join(repo, filepath.FromSlash(n)), data)
		relative := strings.TrimPrefix(n, "infrastructure/")
		if n == kindSource {
			relative = "kind/cluster.yaml"
		}
		put(t, filepath.Join(stage, filepath.FromSlash(relative)), data)
		if n != kindSource {
			put(t, filepath.Join(fresh, filepath.FromSlash(relative)), data)
		}
		srcHash[n] = hash(t, filepath.Join(repo, filepath.FromSlash(n)))
	}
	tfvars := vars{SecurityProfile: "hardened", KubeContext: Context, KubeconfigPath: kube}
	varsPath := filepath.Join(stage, "terraform", "testbed", "terraform.tfvars.json")
	jsonPut(t, varsPath, tfvars)
	buf, err := os.ReadFile(varsPath)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(fresh, "terraform", "testbed", "terraform.tfvars.json"), buf)
	i := intent{SchemaVersion: "firedoor.v10.fd004-live-recovery-stage.v1", Status: "hardened_source_staged_no_infrastructure", SourceCommit: OriginalSourceCommit, Cluster: Cluster, Context: Context, KubeconfigPath: kube, Profile: "hardened", SourceSHA256: srcHash}
	jsonPut(t, filepath.Join(stage, "intent.json"), i)
	jsonPut(t, bootstrapPath, bootstrap{Cluster: Cluster, Context: Context, InfrastructureObservedReady: true, PrivateKubeconfigSHA256: hash(t, kube)})
	jsonPut(t, planPath, planReconciliation{Cluster: Cluster})
	pins := Pins{Bootstrap: hash(t, bootstrapPath), Plan: hash(t, planPath)}
	twelve := map[string]string{}
	for _, n := range SourceFiles() {
		twelve[n] = srcHash[n]
	}
	m := Manifest{SchemaVersion: Schema, Status: Status, Cluster: Cluster, Context: Context, Profile: "hardened", SourceCommit: OriginalSourceCommit, RepoCommit: MergedSourceCommit, StagePath: stage, FreshPath: fresh, KubeconfigPath: kube, SourceSHA256: twelve, TFVarsSHA256: hash(t, varsPath), StageIntentSHA256: hash(t, filepath.Join(stage, "intent.json")), BootstrapReconciliationSHA256: pins.Bootstrap, RetainedPlanReconciliationSHA256: pins.Plan}
	jsonPut(t, filepath.Join(fresh, "manifest.json"), m)
	return fixture{repo, stage, fresh, planPath, m, pins}
}
func TestVerifyStage(t *testing.T) {
	f := newFixture(t)
	r, err := Verify(f.repo, f.stage, f.fresh, f.pins)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesVerified != 12 || !r.DeploymentBlocked || r.ApplyAuthorized || r.TerraformStateVerified || r.FreshPlanProduced || r.WorkspaceSHA256 == "" {
		t.Fatalf("unexpected report: %+v", r)
	}
}
func TestFailClosedMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *fixture)
	}{
		{"foreign cluster", func(t *testing.T, f *fixture) { f.manifest.Cluster = "firedoor-local" }},
		{"false approval", func(t *testing.T, f *fixture) { f.manifest.ApplyAuthorized = true }},
		{"false state verification", func(t *testing.T, f *fixture) { f.manifest.StateVerified = true }},
		{"false plan produced", func(t *testing.T, f *fixture) { f.manifest.FreshPlanProduced = true }},
		{"false terraform execution", func(t *testing.T, f *fixture) { f.manifest.TerraformExecuted = true }},
		{"missing source", func(t *testing.T, f *fixture) { delete(f.manifest.SourceSHA256, SourceFiles()[0]) }},
		{"changed source", func(t *testing.T, f *fixture) {
			n := strings.TrimPrefix(SourceFiles()[0], "infrastructure/")
			put(t, filepath.Join(f.fresh, filepath.FromSlash(n)), []byte("tampered"))
		}},
		{"extra state", func(t *testing.T, f *fixture) {
			put(t, filepath.Join(f.fresh, "terraform", "testbed", "terraform.tfstate"), []byte("{}"))
		}},
		{"extra plan", func(t *testing.T, f *fixture) {
			put(t, filepath.Join(f.fresh, "terraform", "testbed", "test.tfplan"), []byte("plan"))
		}},
		{"backend declaration", func(t *testing.T, f *fixture) {
			n := "infrastructure/terraform/testbed/providers.tf"
			s := []byte(`terraform { backend "http" {} }`)
			for _, p := range []string{filepath.Join(f.repo, filepath.FromSlash(n)), filepath.Join(f.stage, strings.TrimPrefix(filepath.FromSlash(n), "infrastructure"+string(filepath.Separator))), filepath.Join(f.fresh, strings.TrimPrefix(filepath.FromSlash(n), "infrastructure"+string(filepath.Separator)))} {
				put(t, p, s)
			}
			h := hash(t, filepath.Join(f.repo, filepath.FromSlash(n)))
			f.manifest.SourceSHA256[n] = h
			var i intent
			path := filepath.Join(f.stage, "intent.json")
			if err := readJSON(path, &i); err != nil {
				t.Fatal(err)
			}
			i.SourceSHA256[n] = h
			jsonPut(t, path, i)
			f.manifest.StageIntentSHA256 = hash(t, path)
		}},
		{"source pin altered", func(t *testing.T, f *fixture) { f.manifest.SourceSHA256[SourceFiles()[0]] = strings.Repeat("a", 64) }},
		{"wrong bootstrap receipt", func(t *testing.T, f *fixture) { f.manifest.BootstrapReconciliationSHA256 = strings.Repeat("0", 64) }},
		{"unexpected fresh path", func(t *testing.T, f *fixture) { f.manifest.FreshPath = filepath.Dir(f.fresh) }},
		{"wrong kubeconfig path", func(t *testing.T, f *fixture) { f.manifest.KubeconfigPath = "C:\\wrong" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.mutate(t, &f)
			jsonPut(t, filepath.Join(f.fresh, "manifest.json"), f.manifest)
			_, err := Verify(f.repo, f.stage, f.fresh, f.pins)
			if err == nil {
				t.Fatalf("unsafe mutation accepted: %s", tt.name)
			}
		})
	}
}
func TestBadJSON(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.fresh, "manifest.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte(`{"schema_version":"a","schema_version":"b"}`), append(raw, []byte("{}")...), []byte(fmt.Sprintf(`{"unknown":%q}`, "x"))} {
		put(t, p, bad)
		if _, err = Verify(f.repo, f.stage, f.fresh, f.pins); err == nil {
			t.Fatal("bad JSON accepted")
		}
	}
}
func TestSymlinkRefused(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.fresh, "terraform", "testbed", "terraform.tfstate")
	if err := os.Symlink(filepath.Join(f.stage, "intent.json"), path); err != nil {
		t.Skip("symlinks not permitted in this test environment")
	}
	if _, err := Verify(f.repo, f.stage, f.fresh, f.pins); err == nil {
		t.Fatal("symlink was accepted")
	}
}
