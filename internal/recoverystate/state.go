// Package recoverystate evaluates a bounded, non-mutating FD-004 Terraform
// backend/state discovery snapshot. It never grants apply authorization.
package recoverystate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	Cluster = "firedoor-rh-3c138b60a075"
	Schema  = "firedoor.v10.fd004-state-boundary.v1"
	Status  = "local_backend_boundary_inspected_state_unverified_no_apply"
)

type Snapshot struct {
	Cluster                  string `json:"cluster"`
	StageAndPlanRootsPlain   bool   `json:"stage_and_plan_roots_plain"`
	HistoricalEvidencePinned bool   `json:"historical_evidence_pinned"`
	SourcePinsMatch          bool   `json:"source_pins_match"`
	ExactTerraformSources    bool   `json:"exact_terraform_sources"`
	NoExplicitBackendBlocks  bool   `json:"no_explicit_backend_blocks"`
	ProviderLockPinned       bool   `json:"provider_lock_pinned"`
	NoStateArtifactsFound    bool   `json:"no_state_artifacts_found"`
	NoBackendOverrides       bool   `json:"no_backend_overrides"`
	DefaultWorkspace         bool   `json:"default_workspace"`
	BackendMetadataType      string `json:"backend_metadata_type"`
	ClusterAuthenticated     bool   `json:"cluster_authenticated"`
	TerraformStateVerified   bool   `json:"terraform_state_verified"`
	FreshPlanBoundToState    bool   `json:"fresh_plan_bound_to_state"`
	ApplyAuthorized          bool   `json:"apply_authorized"`
	ApplyExecuted            bool   `json:"apply_executed"`
}

type Report struct {
	SchemaVersion         string `json:"schema_version"`
	Status                string `json:"status"`
	Cluster               string `json:"cluster"`
	BackendSourceReviewed bool   `json:"backend_source_reviewed"`
	StateArtifactsFound   bool   `json:"state_artifacts_found"`
	BackendMetadataType   string `json:"backend_metadata_type"`
	CurrentStateVerified  bool   `json:"current_terraform_state_verified"`
	FreshPlanBoundToState bool   `json:"fresh_plan_bound_to_state"`
	ApplyAuthorized       bool   `json:"apply_authorized"`
	ApplyExecuted         bool   `json:"apply_executed"`
	DeploymentBlocked     bool   `json:"deployment_blocked"`
	NextBoundary          string `json:"next_boundary"`
}

func readValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for d.More() {
			kt, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := kt.(string)
			if !ok {
				return errors.New("nonstring JSON key")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate JSON key")
			}
			seen[key] = struct{}{}
			if err := readValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("malformed JSON object")
		}
	case '[':
		for d.More() {
			if err := readValue(d); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("malformed JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func Decode(raw []byte) (Snapshot, error) {
	var s Snapshot
	if len(raw) == 0 || len(raw) > 32*1024 || bytes.IndexByte(raw, 0) >= 0 {
		return s, errors.New("unbounded snapshot")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := readValue(d); err != nil {
		return s, err
	}
	if _, err := d.Token(); err != io.EOF {
		return s, errors.New("trailing input")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, err
	}
	if _, err := d.Token(); err != io.EOF {
		return s, errors.New("unexpected additional data")
	}
	return s, nil
}

func Evaluate(s Snapshot) (Report, error) {
	var empty Report
	if s.Cluster != Cluster {
		return empty, errors.New("foreign cluster")
	}
	if !s.StageAndPlanRootsPlain || !s.HistoricalEvidencePinned || !s.SourcePinsMatch ||
		!s.ExactTerraformSources || !s.NoExplicitBackendBlocks || !s.ProviderLockPinned ||
		!s.NoStateArtifactsFound || !s.NoBackendOverrides || !s.DefaultWorkspace {
		return empty, errors.New("state boundary not independently established")
	}
	if s.BackendMetadataType != "local" && s.BackendMetadataType != "absent" {
		return empty, errors.New("unexpected initialized backend metadata")
	}
	if s.TerraformStateVerified || s.FreshPlanBoundToState || s.ApplyAuthorized || s.ApplyExecuted {
		return empty, errors.New("unsupported state verification, plan, or execution claim")
	}
	// A separate 10C3C2 runner authenticates the live cluster. This command
	// does not, and a caller must not conflate the two observations.
	if s.ClusterAuthenticated {
		return empty, errors.New("this reader cannot assert live cluster authentication")
	}
	return Report{
		SchemaVersion: Schema, Status: Status, Cluster: Cluster,
		BackendSourceReviewed: true, StateArtifactsFound: false,
		BackendMetadataType:  s.BackendMetadataType,
		CurrentStateVerified: false, FreshPlanBoundToState: false,
		ApplyAuthorized: false, ApplyExecuted: false, DeploymentBlocked: true,
		NextBoundary: "separately establish authoritative Terraform state ownership and fresh plan in a new private working directory",
	}, nil
}
