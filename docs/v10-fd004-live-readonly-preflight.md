# Firedoor 10C3C2 — Read-only live deployment preflight

This slice follows the 10C3C1 historical evidence gate. Historical plan integrity
**does not** authorize an apply. This new gate makes a separate current-time
observation of the disposable KIND/Calico lab after a laptop restart.

## What gets checked

- Exact source branch and base commit, exactly 12 untracked source files,
  and no changes to tracked files.
- Original stage intent, immutable bootstrap reconciliation, literal saved
  `$planPath` binary and the separate plan-reconciliation receipt.
- 10C3C1's independent Go historical gate still reports **deployment blocked**.
- Private retained kubeconfig SHA256 and stage Terraform provider lockfile.
- No local `terraform.tfstate`, backup or lock-info file at either of the two
  expected Terraform working-directory roots. **Absence at those locations
  does not prove Terraform state is empty or verified.**
- KIND inventory retains both the disposable cluster and the protected
  `firedoor-local` cluster; no API calls are made to the protected cluster.
- Exactly two owned, running disposable Docker containers; the kubeconfig
  API endpoint must match the Docker control plane's **current** localhost port.
- Explicit `kubectl --kubeconfig ... --context ... get` calls check exactly
  two Ready nodes, Calico DaemonSet and controller, two Ready CoreDNS pods,
  and absence of the three Firedoor-managed namespaces.
- Go `internal/recoverylivegate` independently evaluates the bounded JSON
  snapshot using strict (including duplicate-key rejecting) parsing and a
  fail-closed set of topology, identity, policy, and no-apply conditions.

The live reader does **not** persist the snapshot or resulting report and
never runs Terraform, including `init`, `plan`, `state`, or `apply`. Only the
specified disposable-cluster Kubernetes API and read-only Docker/KIND commands
are queried. Go may update its ordinary build cache. No cluster creation,
container restart, default context switch, evidence rewriting, or cleanup.

## Acceptance

Windows PowerShell 5.1, on the uncommitted deployment feature branch:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1
```

The offline test does not access Docker, KIND, Kubernetes, Terraform, or private
historical evidence. Older 10C3C1's exact-six-source inventory test is not
rerun at this intermediate 12-untracked-file checkpoint. A later integration
slice will update the shared Git guard and preserve pre- and post-commit tests.

## Separately run the live read-only check

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1 -Inspect -ConfirmCluster firedoor-rh-3c138b60a075
```

If Docker isn't available after the laptop restart, or the private kubeconfig
port no longer agrees with Docker, the reader **fails closed**. Do not repair,
recreate, delete, rebind, or replay anything automatically. Inspect the
reported failure before continuing.

A green report means `current_cluster_authenticated=true` and
`current_cluster_healthy=true` **at the time of the read-only observation**.
It still says `current_terraform_state_verified=false`,
`fresh_plan_bound_to_state=false`, `apply_authorized=false`, and
`deployment_blocked=true`. No stale historical plan should ever be applied.

## Next boundary

10C3C3 must separately establish a local/remote Terraform backend and state
ownership, provider consistency, and a new bounded saved plan, with new
admission evidence. Only a later, explicitly reviewed deployment gate could
issue an apply permit. No apply functionality is part of 10C3C1 or 10C3C2.
