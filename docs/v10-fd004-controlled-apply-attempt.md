# FD-004 — 10C3F3 controlled first apply attempt

Scope: exact saved E4 hardened Terraform plan, `0200CE0E…`, on disposable
`firedoor-rh-3c138b60a075`. The protected `firedoor-local` KIND cluster is
outside scope. The retained older plan is forbidden.

## Workflow

1. Install the F3 source patch on the clean `feat/v10-fd004-controlled-apply`
   branch at `d9e70cc`. The one F1 preflight source edit is explicitly pinned.
2. Run F3 offline acceptance. This **never** calls `terraform apply`.
3. Run F3 `-Inspect`, which repeats F1 read-only live checks and verifies the
   single pinned F2 review receipt; no state or private attempt is created.
4. **Separate human decision required:** Only a later, explicit `-Apply` call
   can begin a deployment. It demands a newly typed, apply-specific phrase,
   strict Go scope validation, *another* F1 preflight, and absence of state
   or any prior apply attempt. Review consent alone is **not** apply consent.
5. Before executing the saved plan, create an exclusive, non-overwriting private
   attempt root and write a durable intent. Run Terraform exactly once using
   the initialized local backend, save stdout/stderr privately, and record an
   outcome (including nonzero exit or a partial state file).
6. Any failure is an irreversible stop for this one-shot runner: **no retry,
   refresh, replan, cleanup, force-unlock or destroy**. Inspect private state
   and cluster resources under a separately reviewed recovery procedure.
7. Even exit code 0 is *not* post-deployment resource verification, network
   enforcement proof, or recovery proof. Those require separate read-only F4/F5
   checks after deployment.

## Operational boundaries

- `-Inspect` is safe to repeat while no apply attempt exists.
- The attempt root is `%LOCALAPPDATA%\Firedoor\live-recovery-owned-apply-attempts`.
  Its existence permanently blocks further attempts through this runner.
- Never commit private Terraform plans, state, kubeconfigs, review receipts,
  apply logs, or attempt receipts.
- F3 does not modify, stop, or delete either KIND cluster outside Terraform's
  explicitly authorized 14-create saved plan.
- Windows PowerShell 5.1 acceptance and a reviewed live F3 inspection are
  required *before* considering actual `-Apply` execution.
