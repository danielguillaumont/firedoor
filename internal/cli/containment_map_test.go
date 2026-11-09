package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
)

func TestContainmentMapHistoricalCLIIsExplicitAndOffline(t *testing.T) {
	root := NewRootCommand()
	group, _, err := root.Find([]string{"containment-map"})
	if err != nil || group == nil || group.Name() != "containment-map" {
		t.Fatalf("missing containment-map CLI group: %v", err)
	}
	inspect, _, err := root.Find([]string{"containment-map", "inspect-historical"})
	if err != nil || inspect == nil || inspect.Name() != "inspect-historical" {
		t.Fatalf("missing offline historical inspection: %v", err)
	}
	for _, name := range []string{"input", "profile", "acknowledge-historical"} {
		if inspect.Flags().Lookup(name) == nil {
			t.Fatalf("missing explicit history option %s", name)
		}
	}
	for _, forbidden := range []string{"serve", "host", "listen", "port", "write", "apply", "kubeconfig", "terraform", "export", "context"} {
		if inspect.Flags().Lookup(forbidden) != nil {
			t.Fatalf("unsafe flag on offline inspector: %s", forbidden)
		}
	}
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"containment-map", "inspect-historical", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "origin") || !strings.Contains(out.String(), "--acknowledge-historical") {
		t.Fatal("offline inspection help hides provenance restrictions")
	}
}

func TestContainmentMapHistoricalCLIRefusesUnsafeRequestsWithoutData(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		contains string
	}{
		{"missing input", []string{"containment-map", "inspect-historical", "--profile", "baseline"}, "input"},
		{"missing profile", []string{"containment-map", "inspect-historical", "--input", "private.json"}, "profile"},
		{"wrong profile", []string{"containment-map", "inspect-historical", "--input", "private.json", "--profile", "production", "--acknowledge-historical"}, "profile"},
		{"missing acknowledgement", []string{"containment-map", "inspect-historical", "--input", "private.json", "--profile", "hardened"}, "acknowledge"},
		{"missing file", []string{"containment-map", "inspect-historical", "--input", "secret-report-do-not-echo.json", "--profile", "hardened", "--acknowledge-historical"}, "rejected"},
		{"positional argument", []string{"containment-map", "inspect-historical", "--input", "private.json", "--profile", "hardened", "--acknowledge-historical", "extra"}, "unknown command"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := NewRootCommand()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(test.args)
			err := root.Execute()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.contains) {
				t.Fatalf("expected failure with %q, got %v", test.contains, err)
			}
			if strings.Contains(out.String(), "secret-report-do-not-echo") || strings.Contains(err.Error(), "secret-report-do-not-echo") {
				t.Fatal("private input path leaked in an error")
			}
		})
	}
}

func TestHistoricalContainmentSummaryOnlyShowsBoundedSafeLabels(t *testing.T) {
	m, err := containmentmap.NewDeclaredMap(liveverify.ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	m.Source = "historical_projection"
	m.Limitations = []string{"Offline synthetic test record only; not authenticated."}
	m.Routes[0].Observed = "reachable"
	m.Routes[0].Assessment = "inconclusive"
	m.Routes[0].ObservationEvidenceIDs = []string{"observation-edge-to-app"}
	m.Evidence = []containmentmap.EvidenceRef{{
		ID: "observation-edge-to-app", Kind: "network_observation",
		SourceID: "secret-source-identifier-123", CapturedAtUTC: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC),
		Profile: "hardened", Result: "reachable",
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeHistoricalContainmentSummary(&out, m); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"hardened (operator supplied", "6 directed routes", "Saved observed routes: 1", "Not observed: 5", "edge-to-app", "saved_observed=reachable", "assessment=inconclusive", "ENFORCEMENT VERIFIED: NO", "no source authentication"} {
		if !strings.Contains(strings.ToLower(out.String()), strings.ToLower(expected)) {
			t.Fatalf("summary omits boundary/route: %q", expected)
		}
	}
	for _, forbidden := range []string{"secret-source-identifier-123", "observation-edge-to-app", "10.70.0.", "kubectl", "private.json"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("summary exposed private or misleading detail: %q", forbidden)
		}
	}

	// The default planning-only map is never permitted to masquerade as a
	// saved historical observation, even for a CLI called explicitly.
	declared, err := containmentmap.NewDeclaredMap(liveverify.ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := writeHistoricalContainmentSummary(&out, declared); err == nil || out.Len() != 0 {
		t.Fatal("declared-only map promoted to historical summary")
	}
}
