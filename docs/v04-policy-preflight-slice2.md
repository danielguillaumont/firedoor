# Firedoor v0.4 — Slice 2: owned-lab NetworkPolicy preflight

**Status:** Read-only admission tooling. This slice is not the controlled A/B/A experiment.

## Purpose

v0.3's two blocked routes are empirical observations, not policy attribution. Slice 1 specified temporary single-route ingress witnesses for Edge → Data and Data → App. NetworkPolicies are **additive allows**; ingress permits do not negate egress restrictions. Slice 2 checks whether the *current live cluster* has the expected ingress-only Terraform policy configuration **before** adding any mutation-capable runner.

## What is checked

- Active `kubectl` context is exactly `kind-firedoor-local`; the kind cluster exists and has one named Ready node.
- Terraform local `active_profile` output reads `hardened`.
- Exactly three expected, owned, Active namespaces with the required namespace labels, including `kubernetes.io/metadata.name` for future source selectors.
- Exactly one Ready, owned Pod in each namespace, including node, Pod UID, private Pod IP, labels and expected container readiness; identities are re-read to check for replacement during collection.
- **Edge:** no ingress NetworkPolicies. **App:** only `firedoor-app-ingress`, allowing the **Edge** namespace into the App Pod over TCP 8080. **Data:** only `firedoor-data-ingress`, allowing the **App** namespace into the Data Pod over TCP 8080.
- Unexpected or already-existing witness policies, extra policies, wrong selectors, extra rules, egress policies, wrong ports and changed ownership all prevent an admissible result.

## Safety and restrictions

Only read commands are present (`kubectl config current-context`, `kind get clusters`, fixed-scope `kubectl get`, and `terraform output`). No `kubectl exec`, network probe, resource mutations, `terraform plan` or `terraform apply`. Commands have strict time budgets and fixed target names. The evidence report is written only after all validations pass, under the existing Git-ignored `docs/evidence/local/` path.

A preflight result of `admissible_for_later_review` is *not* authorization to mutate. The upcoming Slice 3 must recheck policies and Pod identities immediately before and after each witness, separately confirm Calico operational status and baseline network behavior, require explicit operator consent, clean up exact-owned temporary resources, and report failure or inconclusive cases without overstating causality.

## Use on Windows

Run source-only tests first:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice2.ps1
```

Only if that succeeds, run the read-only live lab preflight:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Inspect-FiredoorV04Lab.ps1
```

Stop and investigate any failed admission rather than widening the approved plan. Saved evidence is local and must not be committed to GitHub.
