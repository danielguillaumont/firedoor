# Firedoor v0.9 — Release acceptance record (9F2)

**Scope:** `feat/v09-regression` → `main`, PR [#12](https://github.com/danielguillaumont/firedoor/pull/12). This document is a review checklist, **not** a claim that the PR has already merged or that all release controls are met.

## Purpose and evidence boundary

Firedoor v0.9 demonstrates fail-closed Kubernetes NetworkPolicy validation in a **controlled disposable kind/Calico environment**. The 9A synthetic evaluator is separate from the 9D2 typed live observation gate and 9E1/9E2 Kubernetes API policy drift gate. The 9F1 CI workflow creates its own temporary kind cluster on Ubuntu and exercises positive controls, intentional policy weakening, regression detection, restoration, and a missing-policy case. It does not read production clusters, deploy Terraform, or assert production compliance.

## Acceptance matrix

| Area | Observable evidence | Acceptance requirement |
| --- | --- | --- |
| 9A synthetic | Go fixtures; dedicated synthetic CI | Real regression and inconclusive inputs are rejected, with intact control passing |
| 9B cluster & Calico | disposable kind harness | Only generated cluster managed; 2/2 Calico nodes and Kubernetes nodes Ready; generated kubeconfig removed |
| 9C live boundary | three isolated demo pods | Baseline 4/4 HTTP responses, then trusted healthy and edge denied on Service and Pod IP, recovery after policy removal |
| 9D live Go verdicts | `LIVE_GO_VERDICT` records | Protected `MATCH`, weakened `REGRESSION`, restored `MATCH`; `INCONCLUSIVE` and unexpected outcomes fail the healthy gate |
| 9E Kubernetes API drift | `LIVE_DRIFT_VERDICT` records | Protected `MATCH`, weakened `DRIFT`, restored `MATCH`, confirmed missing `DRIFT`; failed API reads never masquerade as missing |
| 9F1 Linux CI | `v09 live boundary regression (kind + Calico)` | Live cluster job succeeds; unexpected healthy-gate success under weakened policy would fail CI |
| Source quality | foundation + synthetic CI checks | Go, console, Terraform format and regression workflows pass for the final PR commit |
| Repository safety | Git and local cluster inventory | No secrets or private evidence tracked, clean working tree, no unreviewed disposable clusters |

## Manual review before marking PR ready

- [ ] Open the **latest** 9F1 live job, not just the green checkmark. Confirm baseline, negative-test rejection, restored policy, missing-policy drift, and generated-cluster cleanup log markers; [CI evidence guide](v09-ci-evidence-review.md)
- [ ] Verify the PR's GitHub status shows all expected checks successful **on the latest commit** and no conflicts
- [ ] Confirm README describes v0.9 as a bounded test harness, not an active production scanner; preserve v0.8 offline report instructions
- [ ] Update PR title and body to describe all slices 9A–9F, with evidence, constraints and test coverage; explicitly mention status of live CI
- [ ] Check the files changed for credentials, private evidence, accidental generated artifacts, and unreviewed dependency/tool changes
- [ ] Confirm branch protection / required checks settings separately; a green CI check is not necessarily a required merge gate
- [ ] Mark PR **Ready for review** only after the above; merge to `main` only after final review and any required approvals
- [ ] After merge, pull `main` fast-forward, validate clean tree, and inspect the `main` push CI results before declaring v0.9 merged

## Limits retained

- Lab enforcement is meaningful **only for the test namespace, workloads, policies, ports, and observed network paths**. It does not establish cluster-wide completeness, continuous monitoring, admission control, runtime protection, or a formal security audit.
- Go provenance fields are typed assertions, not cryptographic attestations; saved evidence requires separate handling.
- The live GitHub job uses version-pinned tools, but container images and GitHub Actions have not all been independently digest/full-commit pinned. Treat upgrades as changes requiring review.
- Container or runner failure can interrupt cleanup. The workflow is designed to fail closed and relies on disposable GitHub-hosted runners for isolation.
- Terraform intent reconciliation and general-purpose drift monitoring are **not** proven by the v0.9 NetworkPolicy test harness.

## Sign-off

Do not fill this section based on expected outcomes. Record the latest commit and GitHub Actions URL **after** checking the actual workflow logs:

- PR head commit: `________________`
- Live job URL / run ID: `________________`
- Latest required checks reviewed: `________________`
- Reviewed by / date: `________________`
- Merge decision and reason: `________________`
