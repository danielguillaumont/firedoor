# Security policy and lab scope

Firedoor is experimental and not currently a production security tool. Please report vulnerabilities privately through GitHub's private vulnerability reporting where available, or by opening a non-sensitive issue; **never post secrets or working exploit details publicly**.

## Hard requirements

- v1 mutations will be restricted to an explicitly identified **local Firedoor-owned kind cluster**. Never infer ownership solely from the current kubectl context name.
- `kind` owns cluster lifecycle and the verified CNI. Terraform owns all long-lived Kubernetes resources and their state. Go owns only bounded, labelled, disposable probes and its evidence store.
- Never run real credential theft, unrestricted scanning, container breakouts or unauthorized target testing. Test data must be synthetic.
- Review Terraform plans and cluster identity before any apply/destroy; Terraform resources are destroyed **before** kind deletion.
- Do not commit `.tfstate`, generated plan files, kubeconfigs, tokens, credentials or real secrets. Export only sanitized summaries / hashes.
- A failed connection is not conclusive containment evidence without a healthy destination, verified CNI enforcement, and successful independent positive controls.
- No cluster mutation is implemented in v0.0. Proposed `setup`, `apply`, `destroy`, `validate` and proof operations are **not implemented commands** yet.

See [ADR-003](docs/adr/ADR-003-kind-cni-safety.md).
