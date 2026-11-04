package models

import "testing"

func TestOfficialStatuses(t *testing.T) {
	statuses := []Status{Crossed, Contained, Blocked, Inconclusive, NotTested, DriftDetected}
	seen := map[Status]bool{}
	for _, s := range statuses {
		if !s.Valid() {
			t.Fatalf("valid status rejected: %s", s)
		}
		if seen[s] {
			t.Fatalf("duplicate status: %s", s)
		}
		seen[s] = true
	}
	if Status("secure").Valid() {
		t.Fatal("unofficial status accepted")
	}
}

func TestScenarioRequiresCompleteContract(t *testing.T) {
	complete := Scenario{SchemaVersion: "1.0.0", ID: "FD-004", Name: "Lateral Movement", Claim: "Bounded connectivity", Controls: []string{"C-004"}}
	if err := complete.Validate(); err != nil {
		t.Fatalf("complete scenario rejected: %v", err)
	}
	complete.Controls = nil
	if err := complete.Validate(); err == nil {
		t.Fatal("empty controls should be rejected")
	}
}
