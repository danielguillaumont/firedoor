package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConsoleAPIApprovedProofInspectionIsExplicitAndInert(t *testing.T) {
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"console-api", "inspect-approved-proof"})
	if err != nil || cmd == nil || cmd.Name() != "inspect-approved-proof" {
		t.Fatalf("missing exact inspection command: %v", err)
	}
	if cmd.Flags().Lookup("consent") == nil || cmd.Flags().Lookup("file") != nil || cmd.Flags().Lookup("path") != nil || cmd.Flags().Lookup("listen") != nil || cmd.Flags().Lookup("kube-context") != nil {
		t.Fatal("unsafe or missing inspection flags")
	}
	var output bytes.Buffer
	inert := &cobra.Command{}
	inert.SetOut(&output)
	for _, value := range []string{"", "incorrect-consent"} {
		if err := inspectConsoleApprovedProof(inert, value); err == nil {
			t.Fatalf("unapproved inspection accepted %q", value)
		}
		if output.Len() != 0 {
			t.Fatal("inspection disclosed a report without consent")
		}
	}
}

func TestConsoleAPIInspectionDoesNotAcceptFileOrServeFlags(t *testing.T) {
	for _, args := range [][]string{
		{"console-api", "inspect-approved-proof", "--file", "some.json"},
		{"console-api", "inspect-approved-proof", "--enable-approved-proof"},
	} {
		cmd := NewRootCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("unexpected acceptance or rejection of arguments %v: %v", args, err)
		}
	}
}
