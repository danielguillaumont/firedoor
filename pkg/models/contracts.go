// Package models defines the initial public evidence and verdict vocabulary.
// The v0.0 types are contracts, not proof that Kubernetes validation exists.
package models

import "fmt"

type Status string

const (
	Crossed       Status = "crossed"
	Contained     Status = "contained"
	Blocked       Status = "blocked"
	Inconclusive  Status = "inconclusive"
	NotTested     Status = "not_tested"
	DriftDetected Status = "drift_detected"
)

func (s Status) Valid() bool {
	switch s {
	case Crossed, Contained, Blocked, Inconclusive, NotTested, DriftDetected:
		return true
	default:
		return false
	}
}

type Scenario struct {
	SchemaVersion string   `json:"schema_version"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Claim         string   `json:"claim"`
	Controls      []string `json:"mapped_controls"`
}

func (s Scenario) Validate() error {
	if s.SchemaVersion == "" || s.ID == "" || s.Name == "" || s.Claim == "" || len(s.Controls) == 0 {
		return fmt.Errorf("incomplete scenario contract")
	}
	return nil
}
