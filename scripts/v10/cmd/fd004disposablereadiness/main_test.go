package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIRefusesMissingUnconfirmedOrUnknown(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "readiness.json")
	if err := os.WriteFile(input, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"-input", input}, {"-confirm-cluster", "firedoor-rh-112233aabbcc"}, {"-input", input, "-confirm-cluster", "firedoor-local"}, {"-input", input, "-confirm-cluster", "firedoor-rh-112233aabbcc", "extra"}, {"-unexpected", "true"}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted unsupported CLI arguments: %v", args)
		}
	}
}
