package cli

import "testing"

// Keep versioned CLI entry points registered when later milestones add commands.
// This is source-only: we locate commands but never invoke Kubernetes, Terraform,
// SQLite, probes or report writers.
func TestFiredoorVersionedCommandsRemainRegistered(t *testing.T) {
	root := NewRootCommand()
	for _, name := range []string{
		"version", "doctor", "evidence", "inspect", "audit", "verify",
		"attribution", "proof",
	} {
		sub, _, err := root.Find([]string{name})
		if err != nil || sub == nil || sub.Name() != name {
			t.Fatalf("missing existing CLI command %q: %v", name, err)
		}
	}
	cmd, _, err := root.Find([]string{"proof", "report"})
	if err != nil || cmd == nil || cmd.Name() != "report" {
		t.Fatalf("missing v0.5 Proof Lens report: %v", err)
	}
}
