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

type comparisonPresentation struct {
	ComparisonDocument
	Layers []struct {
		Name string
		Data LayerComparison
	}
}

var comparisonTemplate = template.Must(template.New("boundary-comparison").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Firedoor · Boundary Proof Comparison</title>
<style>
:root{font-family:system-ui,Arial,sans-serif;color:#243b45;background:#f2ede3}*{box-sizing:border-box}
body{margin:0}header{background:#243b45;color:#fff;padding:30px max(20px,calc((100% - 980px)/2))}
.logo{font-weight:800;letter-spacing:.13em}.eyebrow{text-transform:uppercase;letter-spacing:.1em;font-size:.75rem;color:#e2ac53}
h1{font-size:2rem;margin:.5em 0}.warn{font-weight:750;color:#e2ac53}
main{max-width:980px;margin:25px auto;padding:0 20px}section{background:#fffdf9;border:1px solid #d6d6d2;margin-bottom:14px;padding:18px}
.compare{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.profile{border:1px solid #d6d6d2;padding:12px;min-width:0}
label,.label{color:#64767a;font-size:.76rem;font-weight:700;text-transform:uppercase;letter-spacing:.08em}
dt{color:#64767a;font-size:.8rem;margin-top:8px}dd{margin:2px 0 8px;overflow-wrap:anywhere;font-weight:550}
li{line-height:1.5;margin-bottom:8px}p{line-height:1.55}footer{font-size:.75rem;color:#64767a;padding:20px 0}
@media(max-width:650px){.compare{grid-template-columns:1fr}}
</style></head><body>
<header><div class="logo">FIREDOOR</div><div class="eyebrow">v0.8 · detached historical provenance</div>
<h1>Boundary Proof Comparison</h1><p class="warn">INCONCLUSIVE · Drift NOT ASSESSED</p>
<p>Artifact digest comparison only. No verified policy enforcement, network equivalence, or improvement is established.</p></header>
<main><section><div class="label">Review scope</div><p>Scenario: {{.ScenarioID}} · Route: {{.Route}}</p>
<p>Baseline: {{.BaselineProofID}} · Hardened: {{.HardenedProofID}}</p><p>Schema: {{.SchemaVersion}}</p></section>
{{range .Layers}}<section><h2>{{.Name}}</h2><p><strong>Evidence relation:</strong> {{.Data.EvidenceRelation}}</p>
<div class="compare"><div class="profile"><div class="label">Baseline</div><dl>
<dt>Status</dt><dd>{{.Data.Baseline.Status}}</dd><dt>Opaque reference</dt><dd>{{if .Data.Baseline.Reference}}{{.Data.Baseline.Reference}}{{else}}Not available{{end}}</dd>
<dt>SHA-256</dt><dd>{{if .Data.Baseline.SHA256}}{{.Data.Baseline.SHA256}}{{else}}Not available{{end}}</dd>
<dt>Captured UTC</dt><dd>{{if .Data.Baseline.CapturedAtUTC}}{{.Data.Baseline.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></div>
<div class="profile"><div class="label">Hardened</div><dl>
<dt>Status</dt><dd>{{.Data.Hardened.Status}}</dd><dt>Opaque reference</dt><dd>{{if .Data.Hardened.Reference}}{{.Data.Hardened.Reference}}{{else}}Not available{{end}}</dd>
<dt>SHA-256</dt><dd>{{if .Data.Hardened.SHA256}}{{.Data.Hardened.SHA256}}{{else}}Not available{{end}}</dd>
<dt>Captured UTC</dt><dd>{{if .Data.Hardened.CapturedAtUTC}}{{.Data.Hardened.CapturedAtUTC}}{{else}}Not available{{end}}</dd></dl></div></div></section>{{end}}
<section><h2>Limitations</h2><ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul></section>
<footer>Firedoor · offline inspection artifact · no live cluster query</footer></main></body></html>
`))

// RenderComparisonHTML produces self-contained HTML with escaped strings.
// No browser requests, external styles, scripts, or sensitive local links.
func RenderComparisonHTML(d ComparisonDocument) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	view := comparisonPresentation{ComparisonDocument: d}
	view.Layers = append(view.Layers,
		struct {
			Name string
			Data LayerComparison
		}{"Terraform intent", d.TerraformIntent},
		struct {
			Name string
			Data LayerComparison
		}{"Kubernetes observation", d.KubernetesObservation},
		struct {
			Name string
			Data LayerComparison
		}{"Network behavior", d.NetworkBehavior},
	)
	var buf bytes.Buffer
	if err := comparisonTemplate.Execute(&buf, view); err != nil {
		return nil, errors.New("Boundary Proof comparison HTML rendering failed")
	}
	if buf.Len() > MaxComparisonHTMLBytes {
		return nil, errors.New("Boundary Proof comparison HTML exceeds limit")
	}
	return buf.Bytes(), nil
}
