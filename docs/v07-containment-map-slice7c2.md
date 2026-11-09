# Firedoor v0.7 — Slice 7C2: bounded React planning-map transport

## Source of truth

This slice consumes **only** the two Go 7C1 fixed, localhost-only HTTP routes:
`GET /api/v1/containment-map/baseline` and
`GET /api/v1/containment-map/hardened`.

The React `parsePlanningContainmentMap` function accepts an exact subset of
`firedoor.v07.containment-map.v1`: three owned lab **zone declarations**, six
known directed routes, empty evidence, unchanged security limitations, and
`declared: unspecified`, `observed: not_observed`, `assessment: not_assessed`.
It enforces a matching requested profile, canonical expected-route rationale,
and strict no-extra-fields checking at all levels. Parsed data is reconstructed
from an allowlisted structure rather than directly trusting received objects.

A Go v0.3 planning expectation is not an observed connection or a verified
Terraform receipt. The frontend cannot turn planned expected behavior into
verified NetworkPolicy enforcement or current Kubernetes inventory.

## Transport/security boundaries

- Vite's one development proxy regex allows **exactly nine** fixed read-only
  endpoints: the seven existing v0.6 routes and these two profile-specific
  planning-map routes. It does not allow arbitrary paths, query strings,
  `/production`, `/live`, historical map URLs, wildcard endpoints, or writes
  introduced by the UI.
- The Go server independently rejects unsafe methods, hosts, origins and query
  parameters. Vite is a development proxy, **not** an authentication layer.
- Both loaders reuse `getJSON`: explicit GET, no browser credentials, no cache,
  redirects rejected, strict `application/json`, 32 KiB streamed size limit,
  UTF-8 decoding, caller abort propagation and an eight-second deadline.
- **No auto-fetch** and no React navigation or visual changes are added in 7C2.
  7D will own the page, loading/error states, deliberate profile selection and
  accessibility tests.
- The v0.6 nine-entry `/capabilities` registry is unchanged by design. It does
  not yet advertise the map. The 7D UI integration should migrate the Go
  registry and frontend schema together, without treating approved proof mode
  as authorization to load unreviewed historical network observations.
- Approved detached proof reports and the 7B2 in-memory historical adapter
  **remain separate**. No historical map is exposed through HTTP in this slice.
- No Kubernetes reads, Terraform operations, evidence-file access, secrets,
  kubeconfig access or SQLite operations are added.

## Acceptance

`Test-FiredoorV07Slice7C2.ps1` checks branch and exact source scope; Go tests,
vet and dependencies; all React TypeScript tests and production build; the new
negative parser/transport/proxy tests; and private-evidence Git ignore safety.
The UI must remain the existing v0.6 inspection desk. No screenshots needed.

The existing `api` server can keep running; **restart `react`** after installing
because `vite.config.ts` has changed. From `react`:

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

Then use `other` to test the Vite proxy (not the direct Go API):

```powershell
Invoke-RestMethod 'http://127.0.0.1:5173/api/v1/containment-map/baseline' |
    Select-Object profile, source, current_inventory, enforcement_verified
Invoke-RestMethod 'http://127.0.0.1:5173/api/v1/containment-map/hardened' |
    Select-Object profile, source, current_inventory, enforcement_verified
```

These routes supply data for later UI integration; 7C2 itself does not add a
Containment Map page or any network execution capability.
