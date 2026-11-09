package reporting

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
)

const (
	MaxComparisonJSONBytes = 32 * 1024
	MaxComparisonHTMLBytes = 64 * 1024
)

// RenderComparisonJSON serializes a validated, fixed-schema projection. It
// never manufactures timestamps, credentials, observations, or findings.
func RenderComparisonJSON(d ComparisonDocument) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, errors.New("Boundary Proof comparison JSON encoding failed")
	}
	data = append(data, '\n')
	if len(data) > MaxComparisonJSONBytes {
		return nil, errors.New("Boundary Proof comparison JSON exceeds limit")
	}
	return data, nil
}

// presentationRelation supplies human-readable copy for an already validated
// digest relationship. This is a display-only transform, never a new finding.
func presentationRelation(relation string) (string, string) {
	switch relation {
	case RelationDifferentDigest:
		return "Different artifact digests", "The recorded artifact hashes differ. This is not proof of drift or security improvement."
	case RelationSameDigest:
		return "Same artifact digest", "The recorded hashes match. This is not evidence of repeated testing or equal network behavior."
	default:
		return "Not comparable", "One or both sides lack recorded artifact provenance. No result can be inferred."
	}
}

// shortDigest is only a visual abbreviation; complete SHA-256 stays in HTML.
func shortDigest(digest string) string {
	if len(digest) != 64 {
		return "Not available"
	}
	return digest[:12] + "…" + digest[len(digest)-8:]
}

type comparisonLayerView struct {
	Name                string
	Data                LayerComparison
	RelationLabel       string
	RelationExplanation string
}

type comparisonPresentation struct {
	ComparisonDocument
	Layers []comparisonLayerView
}

var comparisonTemplate = template.Must(template.New("boundary-comparison").Funcs(template.FuncMap{
	"shortDigest": shortDigest,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Firedoor · Boundary Proof Comparison</title>
<style>
:root{font-family:system-ui,Arial,sans-serif;color:#243b45;background:#f2ede3;line-height:1.55}
*{box-sizing:border-box}body{margin:0}header{background:#243b45;color:#fff;padding:32px max(22px,calc((100% - 980px)/2))}
.logo{font-weight:800;letter-spacing:.14em}.eyebrow{text-transform:uppercase;letter-spacing:.1em;font-size:.77rem;color:#e2ac53}
h1{font-size:clamp(1.8rem,4vw,2.35rem);line-height:1.16;margin:.55em 0}h2{font-size:1.24rem;line-height:1.3;margin:.2em 0 .6em}
.warn{font-weight:750;color:#f0b65d}.hero-note{max-width:70ch}
main{max-width:980px;margin:26px auto;padding:0 20px}section{background:#fffdf9;border:1px solid #d6d6d2;margin-bottom:15px;padding:20px}
.compare,.overview{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.overview{grid-template-columns:repeat(3,minmax(0,1fr))}
.profile,.overview-card{border:1px solid #d6d6d2;padding:14px;min-width:0;background:#fffdf9}.overview-card{background:#f8f5ee}
.label{color:#53686e;font-size:.75rem;font-weight:750;text-transform:uppercase;letter-spacing:.075em}
.overview-card strong{display:block;font-size:1rem;line-height:1.4;margin:.5rem 0}.helper{color:#4d6269;font-size:.89rem;line-height:1.5}
p{margin:.55em 0 1em}dt{color:#53686e;font-size:.8rem;margin-top:10px}dd{margin:2px 0 9px;overflow-wrap:anywhere;font-weight:600}
code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:.83em;overflow-wrap:anywhere}
.details-digest{margin-top:.5rem}details summary{cursor:pointer;color:#243b45;font-weight:650;overflow-wrap:anywhere}details[open]{padding-bottom:.3rem}
.digest-full{display:block;overflow-wrap:anywhere;margin:.6rem 0 0;padding:.65rem;background:#f1ede5;border:1px solid #d6d6d2}
.relation{border-left:4px solid #c89342;padding:2px 0 2px 12px;margin-bottom:15px}
li{line-height:1.5;margin-bottom:8px}footer{font-size:.78rem;color:#53686e;padding:17px 0 38px}
:focus-visible{outline:3px solid #bf812c;outline-offset:3px}
@media(max-width:720px){.overview{grid-template-columns:1fr}.compare{grid-template-columns:1fr}section{padding:16px}}
@media print{ :root{background:#fff}header{background:#243b45!important;color:#fff!important;-webkit-print-color-adjust:exact;print-color-adjust:exact}main{margin:14px auto}section,.profile,.overview-card{break-inside:avoid}details .digest-full{display:block}footer{padding-bottom:8px}}
</style></head><body>
<header><div class="logo">FIREDOOR</div><div class="eyebrow">v0.8 · detached historical provenance</div>
<h1>Boundary Proof Comparison</h1><p class="warn">INCONCLUSIVE · Drift NOT ASSESSED</p>
<p class="hero-note">Artifact digest comparison only. Neither a verified Kubernetes enforcement result nor a causal before-and-after experiment.</p></header>
<main>
<section aria-labelledby="overview-title"><div class="label">Reader summary</div><h2 id="overview-title">Evidence at a glance</h2>
<p class="helper">Three independently represented evidence layers. The labels describe artifact provenance only, not security outcomes.</p>
<div class="overview">{{range .Layers}}<div class="overview-card"><div class="label">{{.Name}}</div><strong>{{.RelationLabel}}</strong><div class="helper">{{.RelationExplanation}}</div></div>{{end}}</div>
<p class="helper"><strong>Important:</strong> {{if eq .NetworkBehavior.EvidenceRelation "same_artifact_digest"}}Matching historical network artifacts are a shared reference, not a fresh hardened-state traffic test.{{else}}Historical network artifacts alone do not establish a fresh hardened-state traffic test.{{end}}</p>
</section>
<section aria-labelledby="scope-title"><div class="label">Review scope</div><h2 id="scope-title">Comparison identity</h2>
<p>Scenario: <strong>{{.ScenarioID}}</strong> · Route: <strong>{{.Route}}</strong></p>
<p>Baseline: <code>{{.BaselineProofID}}</code> · Hardened: <code>{{.HardenedProofID}}</code></p><p>Schema: <code>{{.SchemaVersion}}</code></p></section>
{{range .Layers}}
<section aria-labelledby="layer-{{.Data.Layer}}"><h2 id="layer-{{.Data.Layer}}">{{.Name}}</h2>
<div class="relation"><strong>{{.RelationLabel}}</strong><p class="helper">{{.RelationExplanation}}</p><span class="label">Raw relation: </span><code>{{.Data.EvidenceRelation}}</code></div>
<div class="compare"><div class="profile"><div class="label">Baseline</div><dl>
<dt>Evidence status</dt><dd>{{if eq .Data.Baseline.Status "recorded"}}Recorded reference{{else}}Not available{{end}} <code>({{.Data.Baseline.Status}})</code></dd>
<dt>Opaque reference</dt><dd>{{if .Data.Baseline.Reference}}{{.Data.Baseline.Reference}}{{else}}Not available{{end}}</dd>
<dt>SHA-256 digest</dt><dd>{{if .Data.Baseline.SHA256}}<details class="details-digest"><summary>{{shortDigest .Data.Baseline.SHA256}} · Show full digest</summary><code class="digest-full">{{.Data.Baseline.SHA256}}</code></details>{{else}}Not available{{end}}</dd>
<dt>Captured UTC</dt><dd>{{if .Data.Baseline.CapturedAtUTC}}{{.Data.Baseline.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></div>
<div class="profile"><div class="label">Hardened</div><dl>
<dt>Evidence status</dt><dd>{{if eq .Data.Hardened.Status "recorded"}}Recorded reference{{else}}Not available{{end}} <code>({{.Data.Hardened.Status}})</code></dd>
<dt>Opaque reference</dt><dd>{{if .Data.Hardened.Reference}}{{.Data.Hardened.Reference}}{{else}}Not available{{end}}</dd>
<dt>SHA-256 digest</dt><dd>{{if .Data.Hardened.SHA256}}<details class="details-digest"><summary>{{shortDigest .Data.Hardened.SHA256}} · Show full digest</summary><code class="digest-full">{{.Data.Hardened.SHA256}}</code></details>{{else}}Not available{{end}}</dd>
<dt>Captured UTC</dt><dd>{{if .Data.Hardened.CapturedAtUTC}}{{.Data.Hardened.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></div></div></section>{{end}}
<section aria-labelledby="limitations-title"><h2 id="limitations-title">Limitations and interpretation</h2><ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul></section>
<footer>Firedoor · offline inspection artifact · no live cluster query · digest equality is not proof of enforcement</footer></main></body></html>
`))

// RenderComparisonHTML produces self-contained HTML with escaped strings.
// No browser requests, external styles, scripts, or sensitive local links.
func RenderComparisonHTML(d ComparisonDocument) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	view := comparisonPresentation{ComparisonDocument: d}
	for _, item := range []struct {
		name  string
		layer LayerComparison
	}{
		{"Terraform intent", d.TerraformIntent},
		{"Kubernetes observation", d.KubernetesObservation},
		{"Network behavior", d.NetworkBehavior},
	} {
		label, detail := presentationRelation(item.layer.EvidenceRelation)
		view.Layers = append(view.Layers, comparisonLayerView{
			Name: item.name, Data: item.layer,
			RelationLabel: label, RelationExplanation: detail,
		})
	}
	var buf bytes.Buffer
	if err := comparisonTemplate.Execute(&buf, view); err != nil {
		return nil, errors.New("Boundary Proof comparison HTML rendering failed")
	}
	if buf.Len() > MaxComparisonHTMLBytes {
		return nil, errors.New("Boundary Proof comparison HTML exceeds limit")
	}
	return buf.Bytes(), nil
}
