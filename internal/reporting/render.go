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
var proofTemplate = template.Must(template.New("boundary-proof").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Firedoor · Boundary Proof · {{.ProofID}}</title>
<style>
:root{font-family:system-ui,Arial,sans-serif;color:#243b45;background:#f2ede3}
*{box-sizing:border-box}body{margin:0}header{background:#243b45;color:#fff;padding:32px max(24px,calc((100% - 980px)/2))}
.logo{font-weight:800;letter-spacing:.13em}.eyebrow{text-transform:uppercase;letter-spacing:.12em;font-size:.72rem;color:#e2ac53}
h1{font-size:2.2rem;line-height:1.12;margin:.6em 0}.verdict{color:#e2ac53;font-weight:800}
main{max-width:980px;margin:30px auto;padding:0 24px}section{background:#fffdf9;border:1px solid #d6d6d2;padding:20px;margin-bottom:14px}
.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.layer{min-width:0}
.label{font-size:.72rem;letter-spacing:.1em;text-transform:uppercase;color:#67777d;font-weight:700}
p{overflow-wrap:anywhere}dt{color:#67777d;font-size:.82rem;margin-top:12px}dd{margin:2px 0;font-weight:600;overflow-wrap:anywhere}
li{margin:0 0 9px;line-height:1.5}.note{border-left:4px solid #e2ac53;padding-left:14px}
footer{font-size:.78rem;color:#67777d;padding:20px 0 40px}@media(max-width:700px){.grid{grid-template-columns:1fr}}
</style></head><body>
<header><div class="logo">FIREDOOR</div><div class="eyebrow">v0.8 · detached infrastructure evidence</div>
<h1>Boundary Proof</h1><p>Verdict: <span class="verdict">INCONCLUSIVE</span></p><p>This export does not establish verified policy enforcement.</p></header>
<main><section><div class="label">Report identity</div><dl>
<dt>Proof ID</dt><dd>{{.ProofID}}</dd><dt>Scenario</dt><dd>{{.ScenarioID}}</dd>
<dt>Profile</dt><dd>{{.Profile}}</dd><dt>Route</dt><dd>{{.Route}}</dd>
<dt>Schema</dt><dd>{{.SchemaVersion}}</dd></dl></section>
<div class="grid">
<section class="layer"><div class="label">01 / Declared</div><h2>Terraform intent</h2>
<dl><dt>Provenance status</dt><dd>{{.TerraformIntent.Status}}</dd><dt>Opaque reference</dt><dd>{{if .TerraformIntent.Reference}}{{.TerraformIntent.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .TerraformIntent.SHA256}}{{.TerraformIntent.SHA256}}{{else}}Not available{{end}}</dd>
<dt>Captured (UTC)</dt><dd>{{if .TerraformIntent.CapturedAtUTC}}{{.TerraformIntent.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></section>
<section class="layer"><div class="label">02 / Observed</div><h2>Kubernetes snapshot</h2>
<dl><dt>Provenance status</dt><dd>{{.KubernetesObservation.Status}}</dd><dt>Opaque reference</dt><dd>{{if .KubernetesObservation.Reference}}{{.KubernetesObservation.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .KubernetesObservation.SHA256}}{{.KubernetesObservation.SHA256}}{{else}}Not available{{end}}</dd>
<dt>Captured (UTC)</dt><dd>{{if .KubernetesObservation.CapturedAtUTC}}{{.KubernetesObservation.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></section>
<section class="layer"><div class="label">03 / Tested</div><h2>Network behavior</h2>
<dl><dt>Provenance status</dt><dd>{{.NetworkBehavior.Status}}</dd><dt>Opaque reference</dt><dd>{{if .NetworkBehavior.Reference}}{{.NetworkBehavior.Reference}}{{else}}Not available{{end}}</dd>
<dt>Artifact SHA-256</dt><dd>{{if .NetworkBehavior.SHA256}}{{.NetworkBehavior.SHA256}}{{else}}Not available{{end}}</dd>
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
	var output bytes.Buffer
	if err := proofTemplate.Execute(&output, doc); err != nil {
		return nil, errors.New("Boundary Proof HTML encoding failed")
	}
	if output.Len() > MaxHTMLBytes {
		return nil, errors.New("Boundary Proof HTML exceeds output limit")
	}
	return output.Bytes(), nil
}
