package recoverystate

import (
	"encoding/json"
	"strings"
	"testing"
)

func good() Snapshot {
	return Snapshot{Cluster: Cluster, StageAndPlanRootsPlain: true, HistoricalEvidencePinned: true,
		SourcePinsMatch: true, ExactTerraformSources: true, NoExplicitBackendBlocks: true,
		ProviderLockPinned: true, NoStateArtifactsFound: true, NoBackendOverrides: true,
		DefaultWorkspace: true, BackendMetadataType: "local"}
}

func TestReadOnlyBoundary(t *testing.T) {
	for _, backend := range []string{"local", "absent"} {
		s := good()
		s.BackendMetadataType = backend
		r, err := Evaluate(s)
		if err != nil {
			t.Fatalf("backend %q: %v", backend, err)
		}
		if r.Status != Status || !r.BackendSourceReviewed || !r.DeploymentBlocked ||
			r.CurrentStateVerified || r.FreshPlanBoundToState || r.ApplyAuthorized || r.ApplyExecuted {
			t.Fatalf("untruthful state report: %+v", r)
		}
	}
}

func TestEveryBoundaryFailsClosed(t *testing.T) {
	cases := map[string]func(*Snapshot){
		"foreign cluster":              func(s *Snapshot) { s.Cluster = "firedoor-local" },
		"linked root":                  func(s *Snapshot) { s.StageAndPlanRootsPlain = false },
		"changed evidence":             func(s *Snapshot) { s.HistoricalEvidencePinned = false },
		"changed source":               func(s *Snapshot) { s.SourcePinsMatch = false },
		"extra Terraform source":       func(s *Snapshot) { s.ExactTerraformSources = false },
		"backend block":                func(s *Snapshot) { s.NoExplicitBackendBlocks = false },
		"lockfile":                     func(s *Snapshot) { s.ProviderLockPinned = false },
		"state artifact":               func(s *Snapshot) { s.NoStateArtifactsFound = false },
		"backend environment override": func(s *Snapshot) { s.NoBackendOverrides = false },
		"alternate workspace":          func(s *Snapshot) { s.DefaultWorkspace = false },
		"remote backend":               func(s *Snapshot) { s.BackendMetadataType = "s3" },
		"cloud backend":                func(s *Snapshot) { s.BackendMetadataType = "remote" },
		"fake live authentication":     func(s *Snapshot) { s.ClusterAuthenticated = true },
		"fake state verification":      func(s *Snapshot) { s.TerraformStateVerified = true },
		"fake fresh plan":              func(s *Snapshot) { s.FreshPlanBoundToState = true },
		"apply authorization":          func(s *Snapshot) { s.ApplyAuthorized = true },
		"apply executed":               func(s *Snapshot) { s.ApplyExecuted = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := good()
			mutate(&s)
			if _, err := Evaluate(s); err == nil {
				t.Fatalf("accepted %s", name)
			}
		})
	}
}

func TestStrictJSON(t *testing.T) {
	s := good()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decode(raw)
	if err != nil || out.Cluster != Cluster {
		t.Fatalf("valid JSON: %v", err)
	}
	cases := [][]byte{
		{}, []byte(`{}` + `{}`), []byte(`{"cluster":"a","cluster":"b"}`),
		[]byte(`{"cluster":"a","unknown":true}`), append(raw, []byte(`null`)...),
		append(raw, 0), []byte(strings.Repeat(" ", 33*1024)),
	}
	for _, in := range cases {
		if _, err := Decode(in); err == nil {
			t.Fatalf("accepted invalid JSON %q", in)
		}
	}
}
