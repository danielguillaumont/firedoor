package cli

import (
	"strings"
	"testing"
)

func TestConsoleAPISavedProofRequiresExplicitApprovalBeforeListen(t *testing.T) {
	for _, args := range [][]string{
		{"console-api", "serve", "--consent", "unapproved"},
		{"console-api", "serve", "--enable-approved-proof"},
		{"console-api", "serve", "--enable-approved-proof", "--consent", "wrong"},
	} {
		root := NewRootCommand()
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !(strings.Contains(err.Error(), "consent") || strings.Contains(err.Error(), "approved detached")) {
			t.Fatalf("unsafe CLI accepted or unclear rejection: %v", err)
		}
	}
}
