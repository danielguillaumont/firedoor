# FD-004 / 10C3E2 — Private deployment SOURCE staging (not state ownership)

PR #17 merged at `788ad8c`; 10C3E1 independently verified an unused private candidate location. This step can **allocate exactly one** new private directory and copy the same 12 pinned Terraform sources plus hardened variables from the preserved 10C3D1 source-only stage. The old 10C3D2 saved plan is not deployable and MUST NEVER be applied.

The E2 candidate is a **source reservation only**. Its manifest reports no backend initialization, no state, no state-bound plan, no apply authorization and no recovery proof. A directory and signed/fingerprinted manifest do not themselves prove Terraform state ownership. Terraform will generate a distinct, new plan in a later explicitly authorized workflow.

## Safety and execution

1. Run `scripts/v10/Test-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1` on Windows PowerShell 5.1. This is offline and does not access private evidence.
2. Preview `scripts/v10/Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1` with no parameters.
3. Only after checks pass, run it with `-Prepare -ConfirmCluster firedoor-rh-3c138b60a075`. Type the full `STAGE-firedoor-rh-3c138b60a075-OWNED-SOURCE-ONLY` consent phrase.
4. Record the new candidate path and manifest SHA-256 in your private development notes. Neither private manifests nor kubeconfig, Terraform binary plans or state go into Git.
5. A failed preparation must be inspected and preserved. **Do not delete or retry** its private directory.

The runner rejects protected-cluster input before private reads, requires the exact six E1 and six E2 untracked source files on the intended branch, rechecks historical SHA-256 pins, tests for unexpected environment overrides, and refuses any existing owned-state root. After explicit consent it creates a new directory, copies only pinned Terraform configuration and variables, writes an explicitly non-authorizing manifest, and runs an independent read-only Go inventory verifier.

No Terraform `init`, `plan`, `apply`, `destroy`, Kubernetes changes, state files, binary plans or backend initialization are performed. The later E3 checkpoint must authenticate the live disposable cluster, determine exclusive state/back-end authority, initialize its new isolated backend, generate a new state-specific plan, and separately admit that plan. Applying any earlier saved plan is forbidden.
