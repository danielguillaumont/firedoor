package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
)

func TestV07HTTPHistoricalMapRequiresApprovalBeforeListener(t *testing.T) {
	root := NewRootCommand()
	serve, _, err := root.Find([]string{"console-api", "serve"})
	if err != nil || serve == nil {
		t.Fatal("console-api serve missing")
	}
	for _, flag := range []string{"enable-historical-map", "historical-map-profile", "historical-map-consent"} {
		if serve.Flags().Lookup(flag) == nil {
			t.Fatalf("missing explicit HTTP gate %s", flag)
		}
	}
	if serve.Flags().Lookup("historical-map-file") != nil || serve.Flags().Lookup("historical-map-input") != nil || serve.Flags().Lookup("map-path") != nil {
		t.Fatal("arbitrary HTTP evidence path flag introduced")
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing consent", []string{"--enable-historical-map", "--historical-map-profile", "hardened"}},
		{"unapproved consent", []string{"--enable-historical-map", "--historical-map-profile", "baseline", "--historical-map-consent", "BAD"}},
		{"unsupported profile", []string{"--enable-historical-map", "--historical-map-profile", "production", "--historical-map-consent", containmentmap.ApprovedHistoricalMapConsent}},
		{"missing toggle", []string{"--historical-map-profile", "baseline", "--historical-map-consent", containmentmap.ApprovedHistoricalMapConsent}},
		{"combined proof modes", []string{"--enable-approved-proof", "--enable-historical-map", "--historical-map-profile", "baseline", "--historical-map-consent", containmentmap.ApprovedHistoricalMapConsent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewRootCommand()
			cmd.SetArgs(append([]string{"console-api", "serve"}, tc.args...))
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if err == nil || strings.Contains(out.String(), "GET http://") || strings.Contains(out.String(), "FIREDOOR v0.6 / LOCAL") {
				t.Fatalf("unapproved listener might have started: %v", err)
			}
			if strings.Contains(err.Error(), "C:\\") || strings.Contains(err.Error(), "private-v03") {
				t.Fatal("private path leaked")
			}
		})
	}
}
