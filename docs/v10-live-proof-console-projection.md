# FD-004 / G1 — Historical live-proof console projection

## Purpose

Prepare a **minimized, reproducible display contract** for the completed F5
five-path proof, without giving the React console or localhost API access to
private Terraform state, kubeconfig, IP addresses, or the F5 private receipt.

This slice is **pure offline source code**. It does not modify Go API routes,
serve files over HTTP, auto-load evidence, run a Kubernetes command, or change
the deployed cluster. The user must explicitly supply a file and its previously
verified SHA-256 to the CLI. No file discovery occurs.

## What the projection says

- Exactly three paths returned the expected HTTP success markers at capture.
- Exactly two paths failed their bounded HTTP connectivity attempts at capture.
- The independent F4 audit was recorded as passed in the source F5 receipt.
- The display payload is **historical** and **not a current live state check**.
- Neither recovery nor general production security is proven.
- A local SHA-256 match is not an externally authenticated attestation.

## Operator workflow (future G2 integration)

1. Preserve the original F5 receipt under `%LOCALAPPDATA%\Firedoor`.
2. Verify its byte-exact SHA-256 against the earlier successful F5 run.
3. Explicitly invoke `go run ./scripts/v10/cmd/fd004consoleliveproof -receipt <private-receipt> -sha256 <expected-sha256>`.
4. Review the stdout JSON. It contains no raw receipt, IPs, kubeconfig,
   Terraform state path, or internal cluster identifier.
5. Only an explicitly reviewed, sanitized output may become a public demo
   fixture in a later change. Do not commit the private receipt.

## Versioned contract

`firedoor.v10.console-live-proof-projection.v1`. Output contains an ordered
five-path observation list, capture time, a summary count, false current/live
flags, and fixed evidence limitations. Input parsing is exact-field,
bounded, duplicate-protected, and must match the F5 receipt source identity.

## G2 boundary

G2 may add a UI view of a reviewed, sanitized fixture, separately labelled
**historical verified local lab observation**. It must never quietly turn
this projection into a current validation or the security verdict for FD-001
through FD-005. The existing localhost console API remains unchanged by G1.
