package prooflens

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Unlike Slice 2B1 (terraform show -json), this reducer accepts Terraform's
// line-delimited -json machine-readable *UI* stream. The two JSON formats
// are distinct and must never be passed to the wrong parser.
const (
	TerraformUIPlanSchema = "firedoor.v05.terraform-plan-ui.v1"
	TerraformUIPlanMode   = "UNTRUSTED_SANITIZED_UI_STREAM"
	maxTerraformUILine    = 512 * 1024
	maxTerraformUIBytes   = 12 * 1024 * 1024
	maxTerraformUIEvents  = 10000
)

type TerraformUIChangeSummary struct {
	Add    int `json:"add"`
	Change int `json:"change"`
	Remove int `json:"remove"`
}

// TerraformUIPlanSummary contains only a speculative plan UI event summary,
// never raw Terraform plan JSON, state, variables, output values, or logs.
// SHA-256 is a consistency identifier, not a signature or attestation.
type TerraformUIPlanSummary struct {
	SchemaVersion      string                   `json:"schema_version"`
	Mode               string                   `json:"mode"`
	CapturedUTC        string                   `json:"captured_utc"`
	TerraformVersion   string                   `json:"terraform_version"`
	UIFormatVersion    string                   `json:"ui_format_version"`
	SourceTreeSHA256   string                   `json:"source_tree_sha256"`
	UIStreamSHA256     string                   `json:"normalized_ui_stream_sha256"`
	Changes            TerraformUIChangeSummary `json:"changes"`
	EventCount         int                      `json:"event_count"`
	IgnoredEventCount  int                      `json:"ignored_event_count"`
	PlanSummaryPresent bool                     `json:"plan_summary_present"`
	ProofVerdict       string                   `json:"proof_verdict"`
	Limitations        []string                 `json:"limitations"`
}

type terraformUIEvent struct {
	Type      string `json:"type"`
	Level     string `json:"@level"`
	Terraform string `json:"terraform"`
	UI        string `json:"ui"`
	Changes   *struct {
		Add       *int   `json:"add"`
		Change    *int   `json:"change"`
		Remove    *int   `json:"remove"`
		Operation string `json:"operation"`
	} `json:"changes"`
}

func terraformUILimitations() []string {
	return []string{
		"Speculative Terraform planning is not apply; this is not evidence of deployed or enforced state.",
		"Only Terraform UI event counts are retained; raw events, state, output values and diagnostics must never be logged.",
		"The source-tree and stream SHA256 digests provide consistency, not authentication or a trusted attestation.",
	}
}

func validTerraformUIVersion(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "1" {
		return false
	}
	for _, part := range parts[1:] {
		if len(part) < 1 || len(part) > 4 {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// SummarizeTerraformUIStream reduces potentially sensitive newline-delimited
// Terraform UI JSON *without retaining or echoing the raw messages*. We never
// inspect @message, resource addresses, diagnostics or output values. Unknown
// informational messages are counted but not copied to the result.
// The result is UNTRUSTED: this function cannot authenticate its caller.
func SummarizeTerraformUIStream(stream io.Reader, source TerraformSourceManifest) (TerraformUIPlanSummary, error) {
	var empty TerraformUIPlanSummary
	if err := VerifyTerraformSourceManifest(source); err != nil {
		return empty, errors.New("Terraform source manifest failed verification")
	}
	if stream == nil {
		return empty, errors.New("Terraform UI stream required")
	}
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 16*1024), maxTerraformUILine)
	h := sha256.New()
	var count, ignored, length, summaries int
	var version, ui string
	var changes TerraformUIChangeSummary
	for scanner.Scan() {
		line := scanner.Bytes()
		length += len(line) + 1
		count++
		if length > maxTerraformUIBytes || count > maxTerraformUIEvents || len(line) == 0 {
			return empty, errors.New("Terraform UI stream exceeded size/event limits or contained a blank line")
		}
		_, _ = h.Write(line)
		_, _ = h.Write([]byte{'\n'})
		var event terraformUIEvent
		if err := json.Unmarshal(line, &event); err != nil || event.Type == "" {
			return empty, errors.New("invalid Terraform UI event")
		}
		if event.Level == "error" || event.Type == "diagnostic" ||
			strings.HasPrefix(event.Type, "apply_") || strings.HasPrefix(event.Type, "provision_") ||
			event.Type == "planned_action_invocation" || event.Type == "policy_result" {
			return empty, errors.New("errored, mutating or unsupported Terraform UI event")
		}
		if count == 1 && event.Type != "version" {
			return empty, errors.New("Terraform UI must begin with a version event")
		}
		switch event.Type {
		case "version":
			if count != 1 || !validText(event.Terraform, 40) || !validTerraformUIVersion(event.UI) {
				return empty, errors.New("invalid or duplicate Terraform UI version")
			}
			version, ui = event.Terraform, event.UI
		case "change_summary":
			if summaries != 0 || event.Changes == nil || event.Changes.Operation != "plan" ||
				event.Changes.Add == nil || event.Changes.Change == nil || event.Changes.Remove == nil {
				return empty, errors.New("missing, duplicate or non-plan change summary")
			}
			changes = TerraformUIChangeSummary{Add: *event.Changes.Add, Change: *event.Changes.Change, Remove: *event.Changes.Remove}
			if changes.Add < 0 || changes.Change < 0 || changes.Remove < 0 ||
				changes.Add > maxPlanChanges || changes.Change > maxPlanChanges || changes.Remove > maxPlanChanges {
				return empty, errors.New("invalid Terraform change summary counts")
			}
			summaries++
		default:
			// Never examine event output values, names, or diagnostics. Unknown
			// informational types do not grant a stronger proof verdict.
			ignored++
		}
	}
	if err := scanner.Err(); err != nil {
		return empty, errors.New("Terraform UI stream read failed or event exceeded size limit")
	}
	if count == 0 || version == "" || summaries != 1 {
		return empty, errors.New("Terraform UI plan missing version or final change summary")
	}
	result := TerraformUIPlanSummary{
		SchemaVersion: TerraformUIPlanSchema, Mode: TerraformUIPlanMode,
		CapturedUTC:      time.Now().UTC().Format(time.RFC3339Nano),
		TerraformVersion: version, UIFormatVersion: ui,
		SourceTreeSHA256: source.TreeSHA256,
		UIStreamSHA256:   hex.EncodeToString(h.Sum(nil)), Changes: changes,
		EventCount: count, IgnoredEventCount: ignored, PlanSummaryPresent: true,
		ProofVerdict: Inconclusive, Limitations: terraformUILimitations(),
	}
	if err := VerifyTerraformUIPlanSummary(result); err != nil {
		return empty, fmt.Errorf("invalid sanitized plan UI summary: %w", err)
	}
	return result, nil
}

// VerifyTerraformUIPlanSummary catches structural tampering in a saved record,
// but cannot recreate or authenticate the original ephemeral event stream.
func VerifyTerraformUIPlanSummary(s TerraformUIPlanSummary) error {
	if s.SchemaVersion != TerraformUIPlanSchema || s.Mode != TerraformUIPlanMode ||
		!validUTC(s.CapturedUTC) || !validTerraformUIVersion(s.UIFormatVersion) ||
		!validText(s.TerraformVersion, 40) || !validHex(s.SourceTreeSHA256, 64) ||
		!validHex(s.UIStreamSHA256, 64) || !s.PlanSummaryPresent ||
		s.EventCount < 2 || s.EventCount > maxTerraformUIEvents ||
		s.IgnoredEventCount < 0 || s.IgnoredEventCount != s.EventCount-2 ||
		s.Changes.Add < 0 || s.Changes.Change < 0 || s.Changes.Remove < 0 ||
		s.Changes.Add > maxPlanChanges || s.Changes.Change > maxPlanChanges || s.Changes.Remove > maxPlanChanges ||
		s.ProofVerdict != Inconclusive || len(s.Limitations) != 3 ||
		!bytes.Equal([]byte(strings.Join(s.Limitations, "\n")), []byte(strings.Join(terraformUILimitations(), "\n"))) {
		return errors.New("invalid Terraform UI plan summary")
	}
	return nil
}
