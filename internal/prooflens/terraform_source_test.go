package prooflens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tfFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range terraformSourceDirs {
		path := filepath.Join(root, filepath.FromSlash(dir))
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "main.tf"), []byte("# synthetic "+dir+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestTerraformSourcesDeterministicAndNoSensitiveContents(t *testing.T) {
	root := tfFixture(t)
	path := filepath.Join(root, "infrastructure", "terraform", "testbed")
	if err := os.WriteFile(filepath.Join(path, "secrets.auto.tfvars"), []byte("VERY_PRIVATE_DO_NOT_EXPOSE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "terraform.tfstate"), []byte("PRIVATE_STATE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "outputs.tf"), []byte("# Another Terraform source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if a.TreeSHA256 != b.TreeSHA256 || a.FileCount != 4 {
		t.Fatalf("unexpected source count/hash: %+v", a)
	}
	if err := VerifyTerraformSourceManifest(a); err != nil {
		t.Fatal(err)
	}
	if err := RecheckTerraformSources(root, a); err != nil {
		t.Fatal(err)
	}
	sum, err := TerraformSourceSummary(a)
	if err != nil || sum.Kind != "terraform_source_tree" || sum.SHA256 != a.TreeSHA256 {
		t.Fatalf("bad source summary %+v %v", sum, err)
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "secrets.auto.tfvars") {
		t.Fatalf("sensitive content in sanitized manifest: %s", encoded)
	}
}

func TestTerraformSourcesChangeIsDetected(t *testing.T) {
	root := tfFixture(t)
	a, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "infrastructure/terraform/testbed/main.tf"), []byte("# edited\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RecheckTerraformSources(root, a); err == nil {
		t.Fatal("changed Terraform source accepted")
	}
	b, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if a.TreeSHA256 == b.TreeSHA256 {
		t.Fatal("source edit did not change digest")
	}
	b.Files[0].Bytes++
	if err := VerifyTerraformSourceManifest(b); err == nil {
		t.Fatal("tampered index accepted")
	}
}

func TestTerraformSourcesRefuseUnknownOrMissing(t *testing.T) {
	root := tfFixture(t)
	m, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	m.SchemaVersion = "future"
	if err := VerifyTerraformSourceManifest(m); err == nil {
		t.Fatal("unknown schema accepted")
	}
	m.SchemaVersion = TerraformSourceSchema
	m.Limitations = nil
	if err := VerifyTerraformSourceManifest(m); err == nil {
		t.Fatal("missing disclaimers accepted")
	}
	if err := os.Remove(filepath.Join(root, "infrastructure/terraform/modules/workload/main.tf")); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectTerraformSources(root); err == nil {
		t.Fatal("missing module source accepted")
	}
}

func TestTerraformSourcesRejectSymlinkAndOversized(t *testing.T) {
	root := tfFixture(t)
	dir := filepath.Join(root, "infrastructure/terraform/testbed")
	if err := os.WriteFile(filepath.Join(dir, "oversized.tf"), make([]byte, maxTerraformFileBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectTerraformSources(root); err == nil {
		t.Fatal("oversized .tf accepted")
	}
	if err := os.Remove(filepath.Join(dir, "oversized.tf")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "main.tf")
	link := filepath.Join(dir, "linked.tf")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := CollectTerraformSources(root); err == nil {
		t.Fatal("linked .tf accepted")
	}
}

func TestTerraformSourcesVerifyRejectsPathManipulation(t *testing.T) {
	root := tfFixture(t)
	m, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Files[0].Path = "../outside.tf"
	if err := VerifyTerraformSourceManifest(m); err == nil {
		t.Fatal("path traversal accepted")
	}
}
