package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/controls"
)

// The closeout contract checks the report that a CLI JSON consumer receives.
// It uses the deterministic fixture, not a live Kubernetes API.
func TestCloseoutJSONContractAcrossAllZones(t *testing.T) {
	r, err := Build(sampleOwnedSnapshot(), "hardened")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProfileSource != "operator_supplied_unverified" {
		t.Fatalf("profile provenance falsely trusted: %q", decoded.ProfileSource)
	}
	if decoded.Summary.Total != len(decoded.Findings) || len(decoded.Findings) != 18 {
		t.Fatalf("finding count mismatch: %d vs %d", decoded.Summary.Total, len(decoded.Findings))
	}
	for index, zone := range []string{"edge", "app", "data"} {
		expectedNamespace := "firedoor-" + zone
		zoneFindings := decoded.Findings[index*6 : (index+1)*6]
		seen := make(map[string]bool)
		for _, finding := range zoneFindings {
			if finding.Scope.Target.Namespace != expectedNamespace || finding.Scope.Cluster != "firedoor-local" {
				t.Fatalf("wrong finding scope: %+v", finding.Scope)
			}
			if finding.Control.ID == "" || seen[finding.Control.ID] {
				t.Fatalf("missing or repeated control ID: %+v", finding.Control)
			}
			seen[finding.Control.ID] = true
			if err := finding.Validate(); err != nil {
				t.Fatalf("invalid serialized finding: %v", err)
			}
			for _, ev := range finding.Evidence {
				if ev.CapturedAtUTC.IsZero() || ev.Object.Name == "" || ev.FieldPath == "" || ev.Observed == "" {
					t.Fatalf("incomplete evidence in %s: %+v", expectedNamespace, ev)
				}
			}
		}
		if len(seen) != 6 {
			t.Fatalf("expected six distinct controls for %s", zone)
		}
	}
}

func TestCloseoutSecurityFindingRemainsVisible(t *testing.T) {
	r, err := Build(sampleOwnedSnapshot(), "hardened")
	if err != nil {
		t.Fatal(err)
	}
	failures, unknowns := 0, 0
	for _, finding := range r.Findings {
		switch finding.Verdict {
		case controls.Fail:
			failures++
			if finding.Control.ID != "CTRL-NET-001" || finding.Scope.Target.Namespace != "firedoor-edge" {
				t.Fatalf("unexpected failed control: %+v", finding)
			}
		case controls.Unknown:
			unknowns++
			if finding.Control.ID != "CTRL-ID-003" {
				t.Fatalf("unexpected unknown: %+v", finding)
			}
		}
	}
	if failures != 1 || unknowns != 3 || !strings.Contains(strings.ToLower(r.Limitations), "does not prove") {
		t.Fatalf("uncertainty or limitation not preserved: fail=%d unknown=%d", failures, unknowns)
	}
}
