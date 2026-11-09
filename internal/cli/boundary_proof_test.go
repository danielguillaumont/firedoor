package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBoundaryExportCLIRequiresExplicitConsentAndHasNoArbitraryEvidencePaths(t *testing.T) {
	cmd := newBoundaryProofExportCommand()
	if cmd.Flags().Lookup("input") != nil || cmd.Flags().Lookup("output") != nil || cmd.Flags().Lookup("format") != nil {
		t.Fatal("export command permits an arbitrary source, destination or format")
	}
	root := NewRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"proof", "export-boundary", "--root", t.TempDir()})
	if err := root.Execute(); err == nil {
		t.Fatal("export without consent accepted")
	}
	if strings.Contains(output.String(), "TFPLAN-") || strings.Contains(output.String(), "ReceiptSHA256") {
		t.Fatal("error output revealed source metadata")
	}
	root = NewRootCommand()
	root.SetArgs([]string{"proof", "export-boundary", "--root", t.TempDir(), "--consent", "invalid"})
	if err := root.Execute(); err == nil {
		t.Fatal("invalid consent accepted")
	}
}
