# Firedoor v0.7 — Slice 7C1: bounded planning-map HTTP contract

## Intent and source

Extend the v0.6 localhost-only Go API with two explicitly enumerated, GET-only
planning projections. Both results come from `containmentmap.NewDeclaredMap`,
which derives the fixed v0.3 six-route plan. They make **no** Kubernetes,
Terraform, filesystem, network-probe, database, or evidence reads.

This is the first **API-only** half of Slice 7C. The original v0.6 React app,
its nine-entry capability registry, and its Vite proxy are intentionally
unchanged: the 7C2/7D frontend integration will migrate the registry/parser,
proxy and consumer together. Access 7C1 directly at `127.0.0.1:8787`; do not
expect either endpoint to be reachable through Vite yet.

## New explicit routes

| Method | Path | Meaning |
| --- | --- | --- |
| GET | `/api/v1/containment-map/baseline` | Fixed baseline **plan**, no observations |
| GET | `/api/v1/containment-map/hardened` | Fixed hardened **plan**, no observations |

All path, method, query, Host, Origin and request-header restrictions from
`internal/api/handler.go` remain in force. No route accepts a user-supplied
cluster, context, IP, namespace, profile parameter, file path, or probe target.
No wildcard, HTTP write handler, or CORS access is added.

Returned objects use `firedoor.v07.containment-map.v1`, are validated against
`containmentmap.DecodeMap`, and remain within the 32 KiB JSON bound. They have
three declared zone nodes, six directed edges, empty `evidence`,
`declared: unspecified`, `observed: not_observed`, `assessment: not_assessed`,
`current_inventory: false`, and `enforcement_verified: false`.

A **probe-plan expectation** is not a Terraform plan receipt. The expected
baseline/hardened behavior appears only as interpretation text in `reason`;
it cannot be promoted to a verified finding or an observed result.

## Separately gated historical observations

The 7B2 adapter accepts a previously captured v0.3 network observation in
memory. **This slice does not serve it over HTTP.** The older v0.6 approved
proof reader is a different historical report; enabling that reader does not
make topology evidence appear. We will assess a separate, fail-closed and
reviewed source/approval mechanism before exposing any 7B2 projection.

## Manual local smoke check

From the `api` terminal (after `Ctrl+C` if needed):

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

In the `other` terminal (direct Go API; the existing Vite proxy is not yet updated):

```powershell
$baseline = Invoke-RestMethod 'http://127.0.0.1:8787/api/v1/containment-map/baseline'
$hardened = Invoke-RestMethod 'http://127.0.0.1:8787/api/v1/containment-map/hardened'
$baseline | Select-Object profile, source, current_inventory, enforcement_verified
$hardened | Select-Object profile, source, current_inventory, enforcement_verified
$hardened.routes | Select-Object id, declared, observed, assessment
```

Expected: six unobserved/not-assessed edges for each profile. The API process
should remain running after printing the banner; these are only definitions.

## Regression and next steps

`Test-FiredoorV07Slice7C1.ps1` verifies the exact six-file patch scope, Go
formatting, focused and complete Go tests, vet, module verification, CLI build,
TypeScript/Vitest/build, and that private evidence is ignored.

Next: align the React capability contract, safe Vite proxy and strict client
parser with this new map API; then design a separate approved historical
projection path that cannot be enabled merely by changing the UI.
