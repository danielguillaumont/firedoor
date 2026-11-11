# FD-004 / 10C3D3 — Read-only fresh-plan reconciliation

The 10C3D2 session in a **new private run** completed on 2026-11-11. It created a Terraform saved plan, JSON, shape-admission report and session receipt, with 14 additions and no changes/destroys. The original historical FAILED plan remains blocked forever.

**This slice only verifies existing bytes.** It does not create a new plan, infer Terraform state ownership, execute apply, modify a Kubernetes resource, or claim network enforcement. Its sole operational Terraform command is `terraform show -json` against the new, SHA-pinned plan, which is compared byte-for-byte with the saved JSON captured by the original runner. The independent Go verifier checks the source-stage manifest, original source hashes, session/admission claims, and rejects any unexpected private run artifacts or known local managed state files. No private files are written.

The original planning command used `terraform init -backend=false`. Therefore **the saved plan MUST NOT be applied**: it has no independent authoritative state ownership or deployment authorization. A later, separate workflow must establish a state backend and safe ownership boundary, then produce a fresh, state-bound plan. Never try to adapt this already-saved plan for deployment.

## Source and evidence constraints

- Git branch `feat/v10-fd004-fresh-plan-preparation`, HEAD `60a227d`, exactly 15 new source files, zero tracked changes.
- Private plan-run folder: `live-recovery-fresh-plan-runs/20261111-190904908-c7f196bb8eb7` (under `%LOCALAPPDATA%/Firedoor`); source-stage folder `live-recovery-fresh-plans/20261111-183950682-af260cea2bed`.
- Session receipt SHA-256 `7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D`.
- Saved binary plan SHA-256 `DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E`.
- Source-only stage manifest SHA-256 `6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C`.
- Original bootstrap reconciliation SHA-256 `B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2` and historical plan reconciliation SHA-256 `7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60` remain unchanged.

## Operator workflow

Run the guarded installer (dry run then `-Apply`) and the offline acceptance script. Then run the new script without parameters for preview, and use `-Inspect -ConfirmCluster firedoor-rh-3c138b60a075` only after offline acceptance. No typed deployment consent is asked or accepted. Report only the safely printed state; don't upload private binary plans, Terraform JSON, kubeconfigs, receipts or paths containing credentials.

A passing result means the previously generated plan and session reconcile **as an immutable, planning-only artifact**. It does **not** prove current cluster health at a later time, state authority, or NetworkPolicy enforcement. Preserve both KIND clusters and all private evidence.
