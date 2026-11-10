package rehearsalworkspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	work := filepath.Join(root, "lab")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, src := range SourcePaths {
		original := filepath.Join(repo, filepath.FromSlash(src))
		staged := filepath.Join(work, filepath.FromSlash(StageRelative(src)))
		for _, dest := range []string{original, staged} {
			if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte(src+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		h := sha256.Sum256([]byte(src + "\n"))
		hashes[src] = hex.EncodeToString(h[:])
	}
	kube := filepath.Join(work, "kubeconfig")
	m := Manifest{SchemaVersion: Schema, Status: Status, SourceCommit: "8f25ec8", Cluster: "firedoor-rh-aabbccddeeff", Context: "kind-firedoor-rh-aabbccddeeff", KubeconfigPath: kube, Profile: "baseline", SourceSHA256: hashes}
	blob, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(work, "intent.json"), blob, 0600); err != nil {
		t.Fatal(err)
	}
	tf, _ := json.Marshal(map[string]string{"security_profile": "baseline", "kubeconfig_path": kube, "kube_context": m.Context})
	if err := os.WriteFile(filepath.Join(work, "terraform/testbed/terraform.tfvars.json"), tf, 0600); err != nil {
		t.Fatal(err)
	}
	return repo, work
}
func TestVerify(t *testing.T) {
	repo, work := fixture(t)
	r, err := Verify(repo, work)
	if err != nil {
		t.Fatal(err)
	}
	if r.FilesCompared != len(SourcePaths) || r.ApplyAuthorized || r.RecoveryTested || r.TerraformStatePresent || r.KubeconfigPresent {
		t.Fatalf("unsafe claim: %+v", r)
	}
}
func TestRefusesTampering(t *testing.T) {
	cases := map[string]func(string){
		"source file mutation": func(w string) {
			_ = os.WriteFile(filepath.Join(w, "terraform/testbed/main.tf"), []byte("changed"), 0600)
		},
		"unexpected state": func(w string) {
			_ = os.WriteFile(filepath.Join(w, "terraform/testbed/terraform.tfstate"), []byte("{}"), 0600)
		},
		"unexpected kubeconfig":     func(w string) { _ = os.WriteFile(filepath.Join(w, "kubeconfig"), []byte("credential"), 0600) },
		"empty terraform directory": func(w string) { _ = os.MkdirAll(filepath.Join(w, "terraform/testbed/.terraform"), 0700) },
		"unexpected module": func(w string) {
			_ = os.WriteFile(filepath.Join(w, "terraform/modules/workload/evil.tf"), []byte("x"), 0600)
		},
		"misdirected terraform vars": func(w string) {
			_ = os.WriteFile(filepath.Join(w, "terraform/testbed/terraform.tfvars.json"), []byte(`{"security_profile":"baseline","kubeconfig_path":"/tmp/other","kube_context":"kind-firedoor-rh-aabbccddeeff"}`), 0600)
		},
		"unsupported execution claim": func(w string) {
			path := filepath.Join(w, "intent.json")
			b, _ := os.ReadFile(path)
			b = []byte(strings.Replace(string(b), `"apply_authorized":false`, `"apply_authorized":true`, 1))
			_ = os.WriteFile(path, b, 0600)
		},
		"local cluster identity": func(w string) {
			path := filepath.Join(w, "intent.json")
			b, _ := os.ReadFile(path)
			b = []byte(strings.ReplaceAll(string(b), "firedoor-rh-aabbccddeeff", "firedoor-local"))
			_ = os.WriteFile(path, b, 0600)
		},
		"missing resource": func(w string) { _ = os.Remove(filepath.Join(w, "kind/cluster.yaml")) },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			repo, work := fixture(t)
			edit(work)
			if _, err := Verify(repo, work); err == nil {
				t.Fatal("unsafe workspace accepted")
			}
		})
	}
}
func TestRefusesWorkspaceInsideRepo(t *testing.T) {
	repo, work := fixture(t)
	_, err := Verify(repo, filepath.Join(repo, "workspace"))
	if err == nil {
		t.Fatal("nested workspace accepted")
	}
	_ = work
}
