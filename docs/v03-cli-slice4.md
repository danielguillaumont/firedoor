# Firedoor v0.3 — Slice 4: Public verification CLI

## Goal

Wire the v0.3 six-route, evidence-backed verification engine into the real `firedoor` Go CLI. The previous Slice 3 `labverify` harness remains a development aid, not the supported interface. This slice does **not** run new Kubernetes probes or change Terraform.

## Public command

```powershell
go run ./cmd/firedoor verify --profile hardened --input ./docs/evidence/local/v03-slice2-hardened-live-observations.json --format human
```

Machine-readable JSON:

```powershell
go run ./cmd/firedoor verify --profile hardened --input ./docs/evidence/local/v03-slice2-hardened-live-observations.json --format json --require-match
```

The command has exactly one data input, the previously captured local observation JSON. It accepts only `baseline` and `hardened` plans, with the same fixed six routes and expected outcomes as Slice 1. The `--input` option is required; unknown fields, trailing JSON, malformed records, evidence/profile mismatches, and inputs over 2 MiB are rejected. It does not accept user-provided network targets or arbitrary commands.

### Exit behavior

- **Invalid, missing, tampered, or inconsistent input:** nonzero exit, no verification verdict.
- **Valid matched/mismatched/inconclusive report:** default exit 0 and emit the complete human/JSON report; callers MUST read `status`, not equate exit 0 with a match.
- **With `--require-match`:** emit the complete report, then exit nonzero if `status` is `mismatched` or `inconclusive`. This opt-in behavior is suitable for CI gates.

Do not confuse `firedoor verify` (v0.3 saved live-observation verification) with `firedoor evidence verify` (v0.1 before-and-after evidence comparison), or `firedoor audit` (v0.2 configuration inspection).

## Local checks (Windows PowerShell)

1. Verify the source and CLI registration (no cluster reads):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorVerifyCLI.ps1
```

2. Run the public CLI against the actual saved hardened observations (offline):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Confirm-FiredoorV03CLI.ps1 -Profile hardened
```

The second script writes `docs/evidence/local/v03-slice4-hardened-cli-verification.json`, which must remain Git-ignored. It verifies the six-route matrix and that the real CLI outputs the required JSON schema. Both checks should run on `feat/v03-live-verification`.

## Evidence and safety limitations

A `matched` verdict only says recorded HTTP behavior agrees with the fixed baseline/hardened route expectations. **It does not prove** Calico caused failed connections, establish effective RBAC, cryptographically authenticate the JSON file, or attest to Terraform state at capture time. The profile label is operator-supplied; Slice 2 separately checked Terraform's active profile at collection time. `firedoor verify` reads existing evidence only and never invokes `kubectl`, `kind`, `terraform`, or live HTTP probes.

**Next:** v0.3 Slice 5 — final full regression, README/progress update, approved staging, PR, CI, and merge.
