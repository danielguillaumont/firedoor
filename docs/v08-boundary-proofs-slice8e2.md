# Firedoor v0.8 · Slice 8E2 — Reviewable offline proof reports

**Parent commit:** `92c7249` on `feat/v08-proofs`. This is a *presentation-only* enhancement for the v0.8 Boundary Proof and comparison HTML renderers.

## What reviewers see first

- **INCONCLUSIVE** and drift **NOT ASSESSED** remain the only conclusion; there is no verified Kubernetes enforcement claim.
- The comparison opens with an **Evidence at a glance** overview showing the relationships among the Terraform, Kubernetes and historical network **artifact digests**. These are relationships between detached references, **not** detected security improvements or a causal baseline/hardened experiment.
- A matching historical network digest is explicitly called out as a **shared reference, not a fresh traffic test**.
- Evidence relationships have plain-language labels alongside the unchanged machine-readable relation codes.
- SHA-256 appears abbreviated in the visual summary but the complete digest remains inside an accessible `<details>` disclosure on the individual evidence cards. Printed pages are optimized for readability. No JavaScript, remote resources, sensitive file links, added timestamp, or change to the JSON contract.

The standalone Boundary Proof HTML also opens with an evidence-layer summary. `recorded` means supplied provenance reference, **not** verified behavior; `not_available` means missing provenance. The three layers remain independent.

## Reproduce the synthetic review on Windows PowerShell 5.1

From the repository root, after the 8E2 validation passes:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v08\Run-FiredoorV08SyntheticReview.ps1 `
  -ReviewFolder Firedoor-v08-Synthetic-Review-8E2

Start-Process "$HOME\Downloads\Firedoor-v08-Synthetic-Review-8E2\SYNTHETIC-comparison.html"
```

The `-ReviewFolder` parameter only accepts a tightly constrained folder name under Downloads, and the script **refuses to overwrite any existing folder**. The program runs the real offline comparison CLI against a **disposable Git repository with synthetic inputs**. No private evidence, live Kubernetes API, Terraform operation or external network connection is needed. Do not present the output as actual enforcement evidence.

## Review checklist

1. Check the `SYNTHETIC DEMO` warning stays visible above the title when the page is scrolled.
2. Check summary cards show **different digests / not comparable / same digest**, and the explicit warning about shared historical network artifacts.
3. Expand a digest to confirm the complete 64-character hexadecimal SHA-256 is still readable. Verify keyboard navigation and a narrow viewport.
4. Confirm no report makes a passing or failing enforcement claim. JSON schemas and payloads remain unchanged.
5. Compare `SYNTHETIC-baseline.html` and `SYNTHETIC-hardened.html` for equivalent visual hierarchy.

**Release boundary:** the HTML presentation is easier to review; it still cannot authenticate sources, reconstruct traffic tests, or verify policy causation. v0.8 remains an offline evidence-reporting feature, not a security attestation product.
