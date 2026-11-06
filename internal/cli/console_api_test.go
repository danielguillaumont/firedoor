package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Tests must never call 'serve' without --help: no listener, cluster or file
// access is needed to validate CLI registration and static safety contracts.
func TestConsoleAPIRegisteredWithSafeExplicitServe(t *testing.T) {
	root := NewRootCommand()
	group, _, err := root.Find([]string{"console-api"})
	if err != nil || group == nil || group.Name() != "console-api" {
		t.Fatalf("missing console-api: %v", err)
	}
	serve, _, err := root.Find([]string{"console-api", "serve"})
	if err != nil || serve == nil || serve.Name() != "serve" {
		t.Fatalf("missing console-api serve: %v", err)
	}
	for _, flag := range []string{"host", "bind", "listen", "address", "port", "root", "context", "apply", "export", "receipt-id"} {
		if serve.Flags().Lookup(flag) != nil {
			t.Fatalf("unsafe user-controlled server flag %q", flag)
		}
	}
	var out bytes.Buffer
	root.SetArgs([]string{"console-api", "serve", "--help"})
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "127.0.0.1:8787") || !strings.Contains(out.String(), "Ctrl+C") {
		t.Fatal("unsafe or unclear serve help")
	}
	for _, name := range []string{"attribution", "proof", "audit", "verify"} {
		cmd, _, err := NewRootCommand().Find([]string{name})
		if err != nil || cmd.Name() != name {
			t.Fatalf("legacy command missing: %s", name)
		}
	}
}
