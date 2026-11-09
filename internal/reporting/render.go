package reporting

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
)

// RenderJSON returns deterministic, versioned data; it never writes a file.
// No timestamps, IDs, findings or claims are synthesized by this function.
func RenderJSON(doc Document) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, errors.New("Boundary Proof JSON encoding failed")
	}
	data = append(data, '\n')
	if len(data) > MaxJSONBytes {
		return nil, errors.New("Boundary Proof JSON exceeds output limit")
	}
	return data, nil
}

// Only a trusted, built-in template is parsed; all values are ordinary strings
// escaped by html/template. No template.HTML, scripts, remote resources or links
// to sensitive local files can enter the export.
type proofLayerSummary struct {
	Name   string
	Status string
}

type proofPresentation struct {
	Document
	Summary []proofLayerSummary
}

var proofTemplate = template.Must(template.New("boundary-proof").Funcs(template.FuncMap{
	"shortDigest": shortDigest,
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Firedoor · Boundary Proof · {{.ProofID}}</title>
<style>
:root{font-family:system-ui,Arial,sans-serif;color:#243b45;background:#f2ede3;line-height:1.55}
*{box-sizing:border-box}body{margin:0}header{background:#243b45;color:#fff;padding:34px max(22px,calc((100% - 980px)/2))}
.logo{font-weight:800;letter-spacing:.14em}.eyebrow{text-transform:uppercase;letter-spacing:.1em;font-size:.77rem;color:#e2ac53}
h1{font-size:clamp(1.9rem,4vw,2.35rem);line-height:1.17;margin:.55em 0}.verdict{color:#f0b65d;font-weight:800}
main{max-width:980px;margin:26px auto;padding:0 20px}section{background:#fffdf9;border:1px solid #d6d6d2;padding:20px;margin-bottom:14px}
.grid,.overview{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.layer,.overview-card{min-width:0}
.overview-card{padding:14px;border:1px solid #d6d6d2;background:#f8f5ee}.overview-card strong{display:block;margin-top:8px}
.label{font-size:.75rem;letter-spacing:.08em;text-transform:uppercase;color:#53686e;font-weight:750}
h2{font-size:1.2rem;line-height:1.3}p{overflow-wrap:anywhere;line-height:1.55}
dt{color:#53686e;font-size:.82rem;margin-top:12px}dd{margin:2px 0;font-weight:600;overflow-wrap:anywhere}
.helper{color:#4d6269;font-size:.89rem}.note{border-left:4px solid #c89342}
code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:.83em;overflow-wrap:anywhere}
details{margin-top:.4rem}details summary{cursor:pointer;font-weight:650}details[open]{padding-bottom:.3rem}.digest-full{display:block;overflow-wrap:anywhere;margin:.6rem 0 0;padding:.65rem;background:#f1ede5;border:1px solid #d6d6d2}
li{margin:0 0 9px;line-height:1.5}footer{font-size:.78rem;color:#53686e;padding:20px 0 40px}
:focus-visible{outline:3px solid #bf812c;outline-offset:3px}
@media(max-width:760px){.grid,.overview{grid-template-columns:1fr}section{padding:16px}}
@media print{:root{background:#fff}header{background:#243b45!important;color:#fff!important;-webkit-print-color-adjust:exact;print-color-adjust:exact}main{margin:14px auto}section{break-inside:avoid}}
</style></head><body>
<header><div class="logo">FIREDOOR</div><div class="eyebrow">v0.8 · detached infrastructure evidence</div>
<h1>Boundary Proof</h1><p>Verdict: <span class="verdict">INCONCLUSIVE</span></p>
<p>This export does not establish verified policy enforcement or causation.</p></header>
<main>
<section aria-labelledby="summary-title"><div class="label">Reader summary</div><h2 id="summary-title">Evidence at a glance</h2>
<p class="helper">Recorded means a provenance reference was supplied, not that live enforcement was verified. The three layers are independent.</p>
<div class="overview">{{range .Summary}}<div class="overview-card"><div class="label">{{.Name}}</div><strong>{{.Status}}</strong></div>{{end}}</div></section>
<section aria-labelledby="identity-title"><div class="label">Report identity</div><h2 id="identity-title">Scope</h2><dl>
<dt>Proof ID</dt><dd><code>{{.ProofID}}</code></dd><dt>Scenario</dt><dd>{{.ScenarioID}}</dd>
<dt>Profile</dt><dd>{{.Profile}}</dd><dt>Route</dt><dd>{{.Route}}</dd>
<dt>Schema</dt><dd><code>{{.SchemaVersion}}</code></dd></dl></section>
<div class="grid">
<section class="layer"><div class="label">01 / Declared</div><h2>Terraform intent</h2>
<dl><dt>Provenance status</dt><dd>{{if eq .TerraformIntent.Status "recorded"}}Recorded reference{{else}}Not available{{end}} <code>({{.TerraformIntent.Status}})</code></dd><dt>Opaque reference</dt><dd>{{if .TerraformIntent.Reference}}{{.TerraformIntent.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .TerraformIntent.SHA256}}<details><summary>{{shortDigest .TerraformIntent.SHA256}} · Show full digest</summary><code class="digest-full">{{.TerraformIntent.SHA256}}</code></details>{{else}}Not available{{end}}</dd>
<dt>Captured (UTC)</dt><dd>{{if .TerraformIntent.CapturedAtUTC}}{{.TerraformIntent.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></section>
<section class="layer"><div class="label">02 / Observed</div><h2>Kubernetes snapshot</h2>
<dl><dt>Provenance status</dt><dd>{{if eq .KubernetesObservation.Status "recorded"}}Recorded reference{{else}}Not available{{end}} <code>({{.KubernetesObservation.Status}})</code></dd><dt>Opaque reference</dt><dd>{{if .KubernetesObservation.Reference}}{{.KubernetesObservation.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .KubernetesObservation.SHA256}}<details><summary>{{shortDigest .KubernetesObservation.SHA256}} · Show full digest</summary><code class="digest-full">{{.KubernetesObservation.SHA256}}</code></details>{{else}}Not available{{end}}</dd>
<dt>Captured (UTC)</dt><dd>{{if .KubernetesObservation.CapturedAtUTC}}{{.KubernetesObservation.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></section>
<section class="layer"><div class="label">03 / Historical</div><h2>Network behavior</h2>
<dl><dt>Provenance status</dt><dd>{{if eq .NetworkBehavior.Status "recorded"}}Recorded reference{{else}}Not available{{end}} <code>({{.NetworkBehavior.Status}})</code></dd><dt>Opaque reference</dt><dd>{{if .NetworkBehavior.Reference}}{{.NetworkBehavior.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .NetworkBehavior.SHA256}}<details><summary>{{shortDigest .NetworkBehavior.SHA256}} · Show full digest</summary><code class="digest-full">{{.NetworkBehavior.SHA256}}</code></details>{{else}}Not available{{end}}</dd>
<dt>Captured (UTC)</dt><dd>{{if .NetworkBehavior.CapturedAtUTC}}{{.NetworkBehavior.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></section>
</div><section class="note"><div class="label">Interpretation boundaries</div><h2>What this report cannot prove</h2><ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul></section>
<footer>Local, detached and read-only. References and SHA-256 are not signatures or source authentication. No current Kubernetes inspection occurred while rendering.</footer></main>
</body></html>`))

// RenderHTML returns a deterministic, self-contained, safely escaped HTML
// report. The renderer does not write or open any files or contact a network.
func RenderHTML(doc Document) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	status := func(layer Layer) string {
		if layer.Status == StatusRecorded {
			return "Recorded reference"
		}
		return "Not available"
	}
	view := proofPresentation{Document: doc, Summary: []proofLayerSummary{
		{Name: "Terraform intent", Status: status(doc.TerraformIntent)},
		{Name: "Kubernetes snapshot", Status: status(doc.KubernetesObservation)},
		{Name: "Historical network", Status: status(doc.NetworkBehavior)},
	}}
	var output bytes.Buffer
	if err := proofTemplate.Execute(&output, view); err != nil {
		return nil, errors.New("Boundary Proof HTML encoding failed")
	}
	if output.Len() > MaxHTMLBytes {
		return nil, errors.New("Boundary Proof HTML exceeds output limit")
	}
	return output.Bytes(), nil
}
