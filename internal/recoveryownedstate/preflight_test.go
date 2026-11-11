package recoveryownedstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	p := t.TempDir()
	if err := os.Mkdir(filepath.Join(p, "Firedoor"), 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEligibleCandidateNeverGrantsOwnership(t *testing.T) {
	p := fixture(t)
	r, err := Eligible(p, []string{"PATH=/usr/bin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != Schema || r.Status != Status || r.Cluster != Cluster ||
		!r.ExistingFreshPlanReconciled || !r.CandidateRootAbsent || !r.CandidateCanBeReviewed ||
		r.ExistingFreshPlanDeployable || r.CurrentTerraformStateOwnershipVerified ||
		r.StateBackendInitialized || r.StateBoundPlanGenerated || r.ApplyAuthorized ||
		r.ApplyExecuted || r.NetworkRecoveryProven || !r.DeploymentBlocked {
		t.Fatalf("unsafe success claim: %+v", r)
	}
	if _, err := os.Lstat(filepath.Join(p, "Firedoor", CandidateLeaf)); !os.IsNotExist(err) {
		t.Fatalf("reader created a candidate directory: %v", err)
	}
}

func TestAbsentOriginalReconciliationFails(t *testing.T) {
	if _, err := Eligible(fixture(t), nil, false); err == nil {
		t.Fatal("unreconciled evidence accepted")
	}
}

func TestAllExistingCandidateTypesRejected(t *testing.T) {
	for _, tt := range []string{"directory", "file", "symlink"} {
		t.Run(tt, func(t *testing.T) {
			p := fixture(t)
			path := filepath.Join(p, "Firedoor", CandidateLeaf)
			var err error
			switch tt {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "file":
				err = os.WriteFile(path, []byte("existing"), 0600)
			case "symlink":
				err = os.Symlink(p, path)
			}
			if err != nil {
				t.Skipf("unsupported fixture: %v", err)
			}
			if err := CheckCandidate(p, nil); err == nil {
				t.Fatal("existing candidate was accepted")
			}
		})
	}
}

func TestLinkedParentRejected(t *testing.T) {
	p := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(p, "Firedoor")); err != nil {
		t.Skip(err)
	}
	if err := CheckCandidate(p, nil); err == nil {
		t.Fatal("linked parent accepted")
	}
}

func TestOverridesFailClosed(t *testing.T) {
	for _, bad := range []string{"TF_DATA_DIR=x", "tf_workspace=dev", "TF_CLI_ARGS_plan=-destroy", "KUBECONFIG=x", "KUBE_CONTEXT=kind-firedoor-local"} {
		t.Run(strings.SplitN(bad, "=", 2)[0], func(t *testing.T) {
			if err := CheckOverrides([]string{"PATH=ok", bad}); err == nil {
				t.Fatal("ambient override accepted")
			}
		})
	}
}

func TestInvalidPrivateParentsRejected(t *testing.T) {
	p := fixture(t)
	if err := CheckCandidate("relative", nil); err == nil {
		t.Fatal("relative path accepted")
	}
	if err := os.Remove(filepath.Join(p, "Firedoor")); err != nil {
		t.Fatal(err)
	}
	if err := CheckCandidate(p, nil); err == nil {
		t.Fatal("absent private parent accepted")
	}
}
