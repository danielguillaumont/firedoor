# Firedoor security policy and test-lab boundaries

Firedoor is a **local educational/portfolio security engineering project**, not a production security scanner, enforcement controller or independently attested audit service. Report vulnerabilities privately using GitHub's private vulnerability reporting where available. For non-sensitive questions, open an issue without secrets, private reports, working exploit details, credentials or personal data.

## Safe use and trust boundaries

- **Do not target systems you do not own or have explicit permission to test.** Use only synthetic data and the dedicated, positively verified Firedoor kind/Calico lab for opt-in probes and policy experiments. Context names alone do not prove ownership.
- **Keep infrastructure ownership separate.** kind manages cluster lifecycle; Terraform owns persistent testbed objects and state; guarded Go workflows may create only the specifically scoped temporary resources documented for an experiment. No uncontrolled background reconciliation is provided.
- **Review mutation and recovery paths.** Before an independently approved Terraform apply/destroy or historical A/B/A experiment, independently verify the owned cluster, saved plan, expected resource targets, recoverability and required cleanup. An existing saved plan must not be blindly reapplied. Never use the console to substitute for these safety gates.
- **Protect private material.** Never commit or publish `docs/evidence/local/`, Terraform `.tfstate`/`.tfplan`, kubeconfigs, credentials, real secrets, raw local reports or identifying cluster information. Git ignore reduces accidental staging but does not make a secret safe to share. Inspect all new files and screenshots before publishing.

## v0.6 browser/API security model

The Go HTTP API is intentionally fixed to **`127.0.0.1:8787`**, with exact GET-only paths, Host/Origin checks, strict bounded responses, and no live Kubernetes, Terraform or SQLite interfaces. The Vite development server binds to **`127.0.0.1:5173`** and proxies only seven fixed routes. The frontend has 32 KiB response limits, an eight-second request deadline, strict JSON parsing, no sent credentials and no redirect following.

Historical Proof Lens, Environment and History are disabled in default mode. Approved mode reads **one fixed, Git-ignored detached report** at server startup after explicit operator consent and local consistency checks. The browser still requires an explicit click to load the minimized projections. A missing or rejected report fails closed; this is **not proof that the original data is independently authenticated or that enforcement worked**.

**Loopback binding and an exact consent phrase are not authentication or OS privilege elevation.** Another process or user on the same trusted workstation may be able to access locally served approved historical summaries while the server is running. Never expose the API or Vite server through port forwarding, public ingress or a shared/untrusted host. Prefer normal default mode when not actively demonstrating historical material, and stop the approved server with `Ctrl+C` when finished.

## Evidence interpretation rules

- A NetworkPolicy exists **does not** mean the desired traffic was blocked by that policy.
- A failed connection is not independently proven containment; destinations, positive controls, CNI behavior, and alternative failure causes matter.
- An absent namespaced RBAC binding does not rule out effective permissions from other grants; unresolved findings remain **UNKNOWN**.
- A speculative Terraform plan is **not** an apply receipt. A single historical Kubernetes snapshot cannot establish configuration drift or current cluster health.
- Four historical artifacts are **not four Firedoor execution runs**. The approved v0.6 interpretation remains **INCONCLUSIVE** and drift **NOT ASSESSED**. Five scenario planning contracts remain **NOT TESTED**.

Security boundaries and historical observations are discussed in [the architecture](docs/architecture.md), [v0.3 probe safety](docs/v03-safety-boundaries.md), [v0.5 validation](docs/v05-validation-summary.md) and [v0.6 demo procedure](docs/v06-console-demo-guide.md).
