# Firedoor v0.9B2 — Disposable Calico CNI readiness

## Scope

Extends the tested v0.9B1 disposable-kind pattern to a two-node, separate, uniquely named `firedoor-v09-*` kind cluster with an isolated kubeconfig. Uses `kindest/node:v1.36.4` and Calico `v3.33.0`. Never writes to `firedoor-local`, never calls Terraform, and never claims NetworkPolicy enforcement.

## Three explicit operating modes

1. **Plan (default)** — prints actions; performs no network, Docker, kind, kubectl or Terraform operation.
2. **`-PrepareManifest`** — downloads Calico's pinned [v3.33.0 manifest](https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml) to the user's Downloads folder, outside the Git repository, and prints its SHA256. It refuses to overwrite an existing file. Review the YAML before use. No cluster calls.
3. **`-Execute -CalicoManifestSha256 <64-character-hash>`** — requires exact local file SHA match, checks the two-node config, creates a new cluster, checks Docker control-plane ownership and the dedicated kubeconfig, verifies API reachability, creates Calico only in that new cluster, waits for `calico-node` readiness and both expected nodes to be Ready, then deletes the generated cluster and its kubeconfig in `finally`.

## PowerShell 5.1 commands

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9B2.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 -PrepareManifest
# Review Downloads/Firedoor-Calico-v3.33.0.yaml, record SHA256 from output.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 -Execute -CalicoManifestSha256 '<sha256 from preparation>'
```

## Explicit limitations

- The first download requires HTTPS access to raw.githubusercontent.com. Calico images require Internet registry access at runtime. A timeout or registry error is a failed/inconclusive run, not a passing policy check.
- Manifest SHA256 approval detects changes **between** review and execution, but is not a vendor signature or independent publisher attestation. Production CI must eventually pin and verify a trusted artifact digest or vendor attestation.
- `kind` cleanup on `finally` cannot run after a hard kill, system crash or power loss. Investigate any printed `firedoor-v09-*` identifier before retrying; never delete clusters based on an unverified name.
- The operation is local and interactive, not an automatic GitHub-hosted runner test. Passing 9B2 does **not** mean a network policy has actually blocked traffic.
- Exact cluster name is unique per run. Kubeconfig is isolated. Kubernetes requests never consult the user's default context.
- Uses Calico's single-manifest install path rather than the Tigera Operator (official docs support both); operator lifecycle management may be preferred for production.

## References

- https://docs.tigera.io/calico/latest/getting-started/kubernetes/kind
- https://docs.tigera.io/calico/latest/getting-started/kubernetes/requirements
- https://kind.sigs.k8s.io/docs/user/configuration/
