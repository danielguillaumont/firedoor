package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerifyCommandRegisteredAndReadOnly(t *testing.T) {
	root := NewRootCommand()
	verify, _, err := root.Find([]string{"verify"})
	if err != nil || verify == nil || verify.Name() != "verify" {
		t.Fatalf("top-level verify command not registered: %v", err)
	}
	if verify.Flags().Lookup("input") == nil || verify.Flags().Lookup("profile") == nil ||
		verify.Flags().Lookup("format") == nil || verify.Flags().Lookup("require-match") == nil {
		t.Fatal("verify safety/reporting flags missing")
	}
	for _, test := range []struct {
		name     string
		args     []string
		contains string
	}{
		{"missing input", []string{"verify", "--profile", "hardened"}, "input"},
		{"wrong profile", []string{"verify", "--input", "missing.json", "--profile", "production"}, "profile"},
		{"wrong format", []string{"verify", "--input", "missing.json", "--profile", "hardened", "--format", "csv"}, "format"},
		{"missing file", []string{"verify", "--input", "does-not-exist.json", "--profile", "hardened"}, "evidence rejected"},
		{"extra arguments", []string{"verify", "--input", "file.json", "--profile", "hardened", "extra"}, "unknown command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.contains)) {
				t.Fatalf("expected %q error, got %v", test.contains, err)
			}
		})
	}
}
