# Firedoor v0.9D2 — Typed live Go regression verdicts

## Purpose

9A is synthetic-only. 9D1 proved a deliberately weakened policy causes real local
kind/Calico reachability to return and that the protected rule can be restored.
9D2 adds a separate Go observation contract and livegate CLI so these verified
in-pod outcomes drive explicit machine-readable verdicts and process exit codes.

## Live observation contract

- `firedoor.v09.regression.live.v1`, `guarded_kind_calico_probe`
- Fixed `firedoor-v09-<12 lowercase hex>` cluster, matching run ID and kind context
- Fixed namespace, NetworkPolicy identity/UID, route, TCP port, destination
- Fixed phases: `protected`, `weakened`, `restored`; matching expected policy modes
- Fixed targets: Kubernetes `service` or direct `pod_ip`
- Required positive health controls: trusted HTTP marker, live edge process,
  Calico/node readiness, verified API policy identity/rules
- Unknown fields, duplicate fields, missing keys, `null`, wrong types,
  inconsistent state, oversized input, trailing JSON are rejected

Go livegate exit codes: 0 = `MATCH`, 10 = `REGRESSION`, 11 = `INCONCLUSIVE`,
2 = rejected input or execution error. `MATCH` is allowed only for a verified
protected/restored policy with a blocked edge and all health controls intact.
A weakened policy that remains unreachable is `INCONCLUSIVE`, not `MATCH`.

The typed `source` is a claim from the guarded driver, **not** a signed attestation;
standalone JSON inputs can be fabricated. The in-run PowerShell harness uses
isolated kubeconfig/context, API policy identity/UID checks and actual fresh
client requests before writing temporary JSON for Go evaluation. The temporary
binary/observations are removed with the generated cluster. This is evidence of
local kind/Calico behavior, not Terraform state or production enforcement.

## Local steps

1. Run `scripts/v09/Test-FiredoorV09Slice9D2.ps1` (no cluster mutation).
2. Confirm the pinned Calico manifest SHA256 still matches previously approved bytes.
3. Run `Invoke-FiredoorV09CalicoSmoke.ps1 -Execute -VerifyGoVerdicts -CalicoManifestSha256 <approved-64-hex>`.
4. Expect **six** lines of `LIVE_GO_VERDICT`: two protected MATCH, two weakened
   REGRESSION (exit 10 expected), two restored MATCH. Then expect full cleanup.
5. Check that only `firedoor-local` remains. Do not commit until live proof passes.

## Safety and limitations

Only generated disposable kind clusters may be mutated; `firedoor-local` is not
used. No Terraform is applied. Forced process termination may prevent cleanup.
9D2 checks the regression CLI inside the controlled live harness; it does not
provision live kind/Calico in GitHub Actions. Drift detection and dedicated CI
integration are future v0.9 slices.
