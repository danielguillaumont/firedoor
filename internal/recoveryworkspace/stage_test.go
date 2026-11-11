package recoveryworkspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/rehearsalworkspace"
)

func makeStage(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	root := filepath.Join(base, "private")
	stage := filepath.Join(root, "run-a")
	for _, path := range []string{repo, stage} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	hashes := map[string]string{}
	for _, src := range rehearsalworkspace.SourcePaths {
		original := filepath.Join(repo, filepath.FromSlash(src))
		staged := filepath.Join(stage, filepath.FromSlash(rehearsalworkspace.StageRelative(src)))
		for _, path := range []string{original, staged} {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(src+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		h := sha256.Sum256([]byte(src + "\n"))
		hashes[src] = hex.EncodeToString(h[:])
	}
	cluster := "firedoor-rh-aabbccddeeff"
	kube := filepath.Join(stage, "kubeconfig")
	intent := Intent{SchemaVersion: Schema, Status: Status, SourceCommit: BaseCommit,
		Cluster: cluster, Context: "kind-" + cluster, KubeconfigPath: kube, Profile: "hardened", SourceSHA256: hashes}
	data, _ := json.Marshal(intent)
	if err := os.WriteFile(filepath.Join(stage, "intent.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	vars, _ := json.Marshal(variables{Profile: "hardened", KubeconfigPath: kube, Context: "kind-" + cluster})
	if err := os.WriteFile(filepath.Join(stage, "terraform", "testbed", "terraform.tfvars.json"), vars, 0600); err != nil {
		t.Fatal(err)
	}
	return repo, root, stage
}
func TestVerifyPristineStage(t *testing.T) {
	repo, root, stage := makeStage(t)
	r, err := Verify(repo, root, stage)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesCompared != 13 || r.Profile != "hardened" || r.ApplyAuthorized || r.RecoveryTested || r.KubeconfigPresent || r.TerraformStatePresent {
		t.Fatalf("unsafe report: %+v", r)
	}
}
func TestRejectUnsafeStage(t *testing.T) {
	tests := map[string]func(t *testing.T, repo, root, stage string){
		"tampered stage": func(t *testing.T, _, _, stage string) {
			os.WriteFile(filepath.Join(stage, "terraform", "testbed", "main.tf"), []byte("bad"), 0600)
		},
		"tampered repo": func(t *testing.T, repo, _, _ string) {
			os.WriteFile(filepath.Join(repo, "infrastructure", "terraform", "testbed", "main.tf"), []byte("bad"), 0600)
		},
		"terraform state": func(t *testing.T, _, _, stage string) {
			os.WriteFile(filepath.Join(stage, "terraform", "testbed", "terraform.tfstate"), []byte("{}"), 0600)
		},
		"private kubeconfig": func(t *testing.T, _, _, stage string) {
			os.WriteFile(filepath.Join(stage, "kubeconfig"), []byte("secret"), 0600)
		},
		"unknown terraform file": func(t *testing.T, _, _, stage string) {
			os.WriteFile(filepath.Join(stage, "terraform", "testbed", "evil.tf"), []byte("provider"), 0600)
		},
		"missing kind definition": func(t *testing.T, _, _, stage string) { os.Remove(filepath.Join(stage, "kind", "cluster.yaml")) },
		"baseline instead of hardened": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "terraform", "testbed", "terraform.tfvars.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(b), "hardened", "baseline", 1)), 0600)
		},
		"forged apply": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "intent.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(b), `"apply_authorized":false`, `"apply_authorized":true`, 1)), 0600)
		},
		"historical cluster": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "intent.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "firedoor-rh-aabbccddeeff", HistoricalCluster)), 0600)
		},
		"wrong source commit": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "intent.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(b), BaseCommit, "9bae903", 1)), 0600)
		},
		"unexpected intent field": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "intent.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(b), `"status":`, `"extra":true,"status":`, 1)), 0600)
		},
		"duplicate intent key": func(t *testing.T, _, _, stage string) {
			p := filepath.Join(stage, "intent.json")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(b), `"status":`, `"status":"forged","status":`, 1)), 0600)
		},
		"disallowed nested directory": func(t *testing.T, _, _, stage string) {
			os.MkdirAll(filepath.Join(stage, "terraform", "testbed", ".terraform"), 0700)
		},
	}
	for label, mutate := range tests {
		t.Run(label, func(t *testing.T) {
			repo, root, stage := makeStage(t)
			mutate(t, repo, root, stage)
			if _, err := Verify(repo, root, stage); err == nil {
				t.Fatal("unsafe stage accepted")
			}
		})
	}
}
func TestRejectOutsidePrivateRoot(t *testing.T) {
	repo, root, stage := makeStage(t)
	if _, err := Verify(repo, root, repo); err == nil {
		t.Fatal("accepted repository as stage")
	}
	if _, err := Verify(repo, stage, stage); err == nil {
		t.Fatal("accepted private root as stage")
	}
	if _, err := Verify(repo, root, filepath.Dir(root)); err == nil {
		t.Fatal("accepted external stage")
	}
}
func TestRejectLinkedArtifact(t *testing.T) {
	repo, root, stage := makeStage(t)
	p := filepath.Join(stage, "terraform", "testbed", "main.tf")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "infrastructure", "terraform", "testbed", "main.tf"), p); err != nil {
		t.Skipf("cannot create symlink on host: %v", err)
	}
	if _, err := Verify(repo, root, stage); err == nil {
		t.Fatal("accepted linked file")
	}
}
