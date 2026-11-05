package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAttributionCLIRegisteredAndOfflineOnly(t *testing.T) {
	cmd := NewRootCommand()
	sub, _, err := cmd.Find([]string{"attribution"})
	if err != nil || sub == nil || sub.Name() != "attribution" {
		t.Fatalf("attribution subcommand not registered: %v", err)
	}
	for _, flag := range []string{"edge-report", "edge-journal", "data-report", "data-journal", "format"} {
		if sub.Flags().Lookup(flag) == nil {
			t.Fatalf("missing safety/reporting flag %s", flag)
		}
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing reports", []string{"attribution"}, "edge-report"},
		{"wrong format", []string{"attribution", "--edge-report", "a", "--edge-journal", "b", "--data-report", "c", "--data-journal", "d", "--format", "html"}, "format"},
		{"missing saved files", []string{"attribution", "--edge-report", "a", "--edge-journal", "b", "--data-report", "c", "--data-journal", "d"}, "evidence rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := NewRootCommand()
			var out bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&out)
			command.SetArgs(tc.args)
			got := command.Execute()
			if got == nil || !strings.Contains(strings.ToLower(got.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected %q, got %v", tc.want, got)
			}
		})
	}
}
