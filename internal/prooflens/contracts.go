// Package prooflens defines v0.5 evidence contracts and offline integrity checks.
// It does not gather Terraform/Kubernetes data or establish security causality.
package prooflens

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	BundleSchema   = "firedoor.v05.proof-bundle.v1"
	RunSchema      = "firedoor.v05.run.v1"
	EvidenceSchema = "firedoor.v05.evidence.v1"
	ProofSchema    = "firedoor.v05.proof.v1"

	IntegrityValid = "integrity_valid"
	Inconclusive   = "inconclusive"
)

type Stage string

const (
	Declared Stage = "declared"
	Observed Stage = "observed"
	Tested   Stage = "tested"
)

// SourceSummary contains ONLY a sanitized reference and a content digest.
// Never embed Terraform state, credentials, kubeconfigs, or raw plan JSON.
type SourceSummary struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	SHA256    string `json:"sha256"`
}

// Run names an immutable, scoped validation attempt. A syntactically sound
// record is NOT enough to upgrade its security verdict beyond inconclusive.
type Run struct {
	SchemaVersion       string   `json:"schema_version"`
	RunID               string   `json:"run_id"`
	ScenarioID          string   `json:"scenario_id"`
	GitSHA              string   `json:"git_sha"`
	Profile             string   `json:"profile"`
	ClusterFingerprint  string   `json:"cluster_fingerprint_sha256"`
	TerraformPlanSHA256 string   `json:"terraform_plan_sha256,omitempty"`
	StartedUTC          string   `json:"started_utc"`
	EvidenceIDs         []string `json:"evidence_ids"`
	Verdict             string   `json:"verdict"`
	Limitations         []string `json:"limitations"`
}

// Evidence separates declared intent, observed state, and behavioral testing.
// ArtifactSHA256 hashes this record excluding its own checksum field.
type Evidence struct {
	SchemaVersion  string        `json:"schema_version"`
	EvidenceID     string        `json:"evidence_id"`
	RunID          string        `json:"run_id"`
	Stage          Stage         `json:"stage"`
	Kind           string        `json:"kind"`
	Source         SourceSummary `json:"source"`
	CapturedUTC    string        `json:"captured_utc"`
	Summary        string        `json:"summary"`
	Expected       string        `json:"expected,omitempty"`
	Observed       string        `json:"observed,omitempty"`
	ArtifactSHA256 string        `json:"artifact_sha256"`
}

// Proof LINKS the three evidence categories; it is NOT a causal certification.
// v1 requires an inconclusive verdict until a separate semantic evaluator ships.
type Proof struct {
	SchemaVersion      string   `json:"schema_version"`
	ProofID            string   `json:"proof_id"`
	RunID              string   `json:"run_id"`
	Claim              string   `json:"claim"`
	DeclaredEvidenceID string   `json:"declared_evidence_id"`
	ObservedEvidenceID string   `json:"observed_evidence_id"`
	TestedEvidenceID   string   `json:"tested_evidence_id"`
	Verdict            string   `json:"verdict"`
	Limitations        []string `json:"limitations"`
}

// BundleSHA256 covers all records and embedded evidence hashes. This detects
// accidental/tampered changes in LOCAL data, but is NOT a signature or trusted
// provenance: an attacker who can rewrite the file can recompute the digest.
type Bundle struct {
	SchemaVersion string     `json:"schema_version"`
	Run           Run        `json:"run"`
	Evidence      []Evidence `json:"evidence"`
	Proof         Proof      `json:"proof"`
	BundleSHA256  string     `json:"bundle_sha256"`
}

type ValidationReport struct {
	SchemaVersion string   `json:"schema_version"`
	Integrity     string   `json:"integrity"`
	ProofVerdict  string   `json:"proof_verdict"`
	EvidenceCount int      `json:"evidence_count"`
	Problems      []string `json:"problems"`
	Limitations   []string `json:"limitations"`
}

func checksum(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func evidenceChecksum(e Evidence) (string, error) {
	e.ArtifactSHA256 = ""
	return checksum(e)
}

func bundleChecksum(b Bundle) (string, error) {
	b.BundleSHA256 = ""
	return checksum(b)
}

func validHex(s string, n int) bool {
	if len(s) != n || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func validText(s string, limit int) bool {
	return s != "" && strings.TrimSpace(s) == s && len(s) <= limit && !strings.ContainsAny(s, "\x00\r")
}

func validUTC(s string) bool {
	if !strings.HasSuffix(s, "Z") {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, s)
	return err == nil
}

func checkLimitations(items []string) bool {
	if len(items) < 1 || len(items) > 8 {
		return false
	}
	for _, item := range items {
		if !validText(item, 512) {
			return false
		}
	}
	return true
}

func validateShape(b Bundle) []string {
	var problems []string
	add := func(s string) { problems = append(problems, s) }
	if b.SchemaVersion != BundleSchema || b.Run.SchemaVersion != RunSchema || b.Proof.SchemaVersion != ProofSchema {
		add("unexpected bundle/run/proof schema version")
	}
	r := b.Run
	if !validText(r.RunID, 128) || !validText(r.ScenarioID, 64) || !strings.HasPrefix(r.ScenarioID, "FD-") {
		add("invalid run or scenario identifier")
	}
	if !validHex(r.GitSHA, 40) || !validHex(r.ClusterFingerprint, 64) {
		add("missing or invalid Git SHA / cluster fingerprint")
	}
	if r.TerraformPlanSHA256 != "" && !validHex(r.TerraformPlanSHA256, 64) {
		add("invalid sanitized Terraform plan hash")
	}
	if !validText(r.Profile, 32) || !validUTC(r.StartedUTC) {
		add("invalid run profile or UTC timestamp")
	}
	if r.Verdict != Inconclusive || !checkLimitations(r.Limitations) {
		add("run must be explicitly inconclusive with limitations")
	}
	p := b.Proof
	if !validText(p.ProofID, 128) || p.RunID != r.RunID || !validText(p.Claim, 512) {
		add("invalid proof ID, run link, or claim")
	}
	if p.Verdict != Inconclusive || !checkLimitations(p.Limitations) {
		add("proof must be explicitly inconclusive with limitations")
	}
	if len(b.Evidence) != 3 || len(r.EvidenceIDs) != 3 {
		add("v1 requires exactly three distinct declared/observed/tested records")
	}
	if len(b.Evidence) > 32 {
		return append(problems, "too many evidence records")
	}
	byID := make(map[string]Evidence)
	byStage := make(map[Stage]string)
	for _, e := range b.Evidence {
		if e.SchemaVersion != EvidenceSchema || !validText(e.EvidenceID, 128) || e.RunID != r.RunID {
			add("invalid evidence schema, identifier, or run link")
		}
		if _, ok := byID[e.EvidenceID]; ok {
			add("duplicate evidence identifier")
		}
		byID[e.EvidenceID] = e
		if _, ok := byStage[e.Stage]; ok {
			add("duplicate evidence stage")
		}
		byStage[e.Stage] = e.EvidenceID
		expectedKind := map[Stage]string{Declared: "iac_declaration", Observed: "kubernetes_state", Tested: "network_test"}[e.Stage]
		if expectedKind == "" || e.Kind != expectedKind {
			add("invalid evidence stage / kind combination")
		}
		if !validUTC(e.CapturedUTC) || !validText(e.Summary, 512) {
			add("invalid evidence timestamp or summary")
		}
		if !validText(e.Source.Kind, 40) || !validText(e.Source.Reference, 256) || !validHex(e.Source.SHA256, 64) {
			add("invalid or missing sanitized source provenance")
		}
		if (e.Stage == Declared && (!validText(e.Expected, 128) || e.Observed != "")) ||
			(e.Stage == Observed && (!validText(e.Observed, 128) || e.Expected != "")) ||
			(e.Stage == Tested && (!validText(e.Expected, 128) || !validText(e.Observed, 128))) {
			add("expected and observed values must match their evidence stage")
		}
	}
	for _, stage := range []Stage{Declared, Observed, Tested} {
		if byStage[stage] == "" {
			add(fmt.Sprintf("missing %s evidence", stage))
		}
	}
	proofRefs := map[Stage]string{Declared: p.DeclaredEvidenceID, Observed: p.ObservedEvidenceID, Tested: p.TestedEvidenceID}
	for stage, id := range proofRefs {
		if id == "" || byStage[stage] != id {
			add("proof refers to missing or wrong-stage evidence")
		}
	}
	refs := make(map[string]bool)
	for _, id := range r.EvidenceIDs {
		if refs[id] || id == "" {
			add("duplicate/empty run evidence reference")
		}
		refs[id] = true
		if _, ok := byID[id]; !ok {
			add("run refers to missing evidence")
		}
	}
	for id := range byID {
		if !refs[id] {
			add("unreferenced evidence in bundle")
		}
	}
	return problems
}

// Seal signs nothing. It adds reproducible SHA256 integrity checks to an
// otherwise valid, INCONCLUSIVE contract (never converts observations to proof).
func Seal(b Bundle) (Bundle, error) {
	if problems := validateShape(b); len(problems) > 0 {
		return Bundle{}, fmt.Errorf("invalid proof bundle shape: %s", strings.Join(problems, "; "))
	}
	for i := range b.Evidence {
		h, err := evidenceChecksum(b.Evidence[i])
		if err != nil {
			return Bundle{}, err
		}
		b.Evidence[i].ArtifactSHA256 = h
	}
	h, err := bundleChecksum(b)
	if err != nil {
		return Bundle{}, err
	}
	b.BundleSHA256 = h
	return b, nil
}

// VerifyBundle returns an INCONCLUSIVE integrity result on any mismatch.
// A valid hash is not proof of authenticity, enforcement or policy causality.
func VerifyBundle(b Bundle) ValidationReport {
	result := ValidationReport{
		SchemaVersion: "firedoor.v05.integrity-review.v1",
		Integrity:     Inconclusive,
		ProofVerdict:  Inconclusive,
		EvidenceCount: len(b.Evidence),
		Problems:      []string{},
		Limitations: []string{
			"Local SHA256 checks detect changes but do not authenticate who created the file.",
			"Evidence contracts do not establish Terraform provenance, live Kubernetes state, or policy causality.",
		},
	}
	result.Problems = append(result.Problems, validateShape(b)...)
	for _, e := range b.Evidence {
		h, err := evidenceChecksum(e)
		if err != nil || !validHex(e.ArtifactSHA256, 64) || e.ArtifactSHA256 != h {
			result.Problems = append(result.Problems, "evidence checksum mismatch: "+e.EvidenceID)
		}
	}
	h, err := bundleChecksum(b)
	if err != nil || !validHex(b.BundleSHA256, 64) || b.BundleSHA256 != h {
		result.Problems = append(result.Problems, "bundle checksum mismatch")
	}
	if len(result.Problems) == 0 {
		result.Integrity = IntegrityValid
	}
	return result
}
