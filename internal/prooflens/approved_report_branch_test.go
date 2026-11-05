package prooflens

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The release read-only reporter must still work after v0.5 is merged to main.
// Other branches and a missing Git-ignore safeguard must be rejected.
func TestApprovedEvidenceRepoBranchAllowlistAndIgnoredDatabase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, data)
		}
	}
	git("-c", "init.defaultBranch=fixture-root", "init", "-q")
	git("-c", "user.name=Firedoor Test", "-c", "user.email=firedoor-test@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture")
	git("checkout", "-q", "-b", "feat/v05-evidence")
	if approvedEvidenceRepo(root) {
		t.Fatal("missing evidence ignore rule should refuse approval")
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("docs/evidence/local/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !approvedEvidenceRepo(root) {
		t.Fatal("feature branch plus ignored evidence path should allow read-only reporting")
	}
	git("checkout", "-q", "-b", "main")
	if !approvedEvidenceRepo(root) {
		t.Fatal("main should allow read-only report after v0.5 merge")
	}
	git("checkout", "-q", "-b", "other-branch")
	if approvedEvidenceRepo(root) {
		t.Fatal("unapproved branch should fail closed")
	}
}
