package api

// This file projects only the Go engine's static control *definitions* and an
// explicitly approved, already-minimized historical Kubernetes snapshot.
// It never opens a file, uses a client, queries Kubernetes, or evaluates a
// security control. There are deliberately NO finding scores or run results.

import (
	"github.com/danielguillaumont/firedoor/internal/controls"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const (
	ControlCatalogSource        = "go_control_definitions_only"
	HistoricalEnvironmentSource = "approved_detached_historical_report_only"
)

type ControlCatalog struct {
	SchemaVersion    string                `json:"schema_version"`
	Source           string                `json:"source"`
	EvaluationStatus string                `json:"evaluation_status"`
	Definitions      []controls.Definition `json:"definitions"`
	Limitations      []string              `json:"limitations"`
}

func controlCatalog() ControlCatalog {
	return ControlCatalog{
		SchemaVersion:    SchemaVersion,
		Source:           ControlCatalogSource,
		EvaluationStatus: "not_evaluated",
		Definitions:      controls.FoundationDefinitions(),
		Limitations: []string{
			"Control definitions are specifications, not observed findings or enforcement proof.",
			"No Kubernetes inspection, Terraform inspection, or control scoring occurred in this HTTP request.",
		},
	}
}

type HistoricalEnvironment struct {
	SchemaVersion       string   `json:"schema_version"`
	Source              string   `json:"source"`
	ObservationStatus   string   `json:"observation_status"`
	Cluster             string   `json:"cluster"`
	Context             string   `json:"context"`
	Profile             string   `json:"profile"`
	CapturedUTC         string   `json:"captured_utc"`
	NamespaceCount      int      `json:"namespace_count"`
	ReadyPodCount       int      `json:"ready_pod_count"`
	PolicyCount         int      `json:"policy_count"`
	CurrentRead         bool     `json:"current_read"`
	EnforcementVerified bool     `json:"enforcement_verified"`
	Limitations         []string `json:"limitations"`
}

func projectHistoricalEnvironment(approved *prooflens.ConsoleProofSummary) HistoricalEnvironment {
	snapshot := approved.Kubernetes
	return HistoricalEnvironment{
		SchemaVersion:       SchemaVersion,
		Source:              HistoricalEnvironmentSource,
		ObservationStatus:   "historical_snapshot_not_current",
		Cluster:             snapshot.Cluster,
		Context:             snapshot.Context,
		Profile:             snapshot.Profile,
		CapturedUTC:         snapshot.CapturedUTC,
		NamespaceCount:      snapshot.NamespaceCount,
		ReadyPodCount:       snapshot.ReadyPodCount,
		PolicyCount:         snapshot.PolicyCount,
		CurrentRead:         false,
		EnforcementVerified: false,
		Limitations: []string{
			"This is an approved historical Kubernetes snapshot, not a current cluster inventory.",
			"Policy counts and ready Pod counts do not establish enforcement or network containment.",
			"The original detached report is not an independently authenticated attestation.",
		},
	}
}
