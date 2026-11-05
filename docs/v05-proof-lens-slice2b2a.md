# Firedoor v0.5 — Proof Lens Slice 2B2A: Speculative Terraform UI stream reducer

## Purpose

Separate the Terraform `plan -json` **machine-readable UI stream** from the richer Terraform `show -json <planfile>` **plan representation** handled by Slice 2B1. They are distinct schemas and must never be confused.

This slice implements and tests a bounded, fail-closed `SummarizeTerraformUIStream` reducer in `internal/prooflens/terraform_plan_stream.go`. The CLI preview uses **synthetic** event data; it does **not** invoke Terraform, read state, inspect a live cluster, or produce a saved plan. No real plan is executed by this slice.

## Retained information

- Terraform and UI format versions, a fixed source-tree SHA-256 from the previously verified Slice 2A source manifest.
- Integer counts `add`, `change`, `remove` from exactly one final `operation: plan` change summary.
- Number of UI events, number of unrecognized/ignored informational events, and a hash of a newline-normalized in-memory event stream.
- `proof_verdict: inconclusive` and explicit non-removable limitations.

No `@message`, resource addresses, log details, diagnostic text, output names/types/values, input variables, or state objects appear in the output. `outputs` events are ignored rather than displayed; **Terraform machine-readable UI can contain sensitive output values**.

## Rejections and limits

- Requires a valid source manifest and `type: version` first with supported UI major 1.
- Requires exactly one complete `type: change_summary` for `operation: plan`.
- Rejects invalid/duplicate summaries, malformed JSON, negative counts, error-level and diagnostic events, and apply/provision events.
- Limits each JSON event line to 512 KiB, total input to 12 MiB, and event count to 10,000.
- Never upgrades the security verdict: successful parsing does not prove Terraform applied anything or that any Kubernetes policy is enforced.

## Run on Windows

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2B2A.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 2B2A failed. Stop.' }

go run ./scripts/v05/cmd/planstreampreview --root .
```

## Next step

Review a separately gated, fixed-profile `terraform plan -json` collector that **streams** Terraform UI messages directly to the reducer instead of ever saving a raw plan file or printing unfiltered JSON. Before a real plan, independently verify the owned Kubernetes kind lab, Terraform local state assumptions, and the pinned hardened profile. Such a speculative plan may read provider state or acquire a transient backend lock; it is **not** the same as completely offline execution. A crash can leave an interrupted operation requiring inspection. Never use `terraform apply`, `terraform output -json`, `terraform show -json` on state, or `terraform plan -out` in this slice.
