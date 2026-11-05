# Firedoor v0.4 — Slice 4: Offline attribution reporting

## Goal
Expose the two completed owned-kind A/B/A policy experiments as a legible, verifiable `firedoor attribution` CLI result. This is intentionally the **last feature slice of v0.4**; the remaining work is a closeout review, reproducible documentation, CI, commit and PR.

## Safety and evidence limits

- `firedoor attribution` reads four explicitly supplied local files (two JSON live reports and two JSONL recovery journals); it **does not connect to Kubernetes, run traffic probes, or create/delete policies**.
- The importer checks exact approved experiment names, context and profile, both A/B/A phase sequences, bounded attempts, exact HTTP destination marker proof, server-assigned UID consistency, cleanup and restoration flags, released lock claim, journal event ordering and matching recovery summaries.
- It **fails closed** for unknown fields, incomplete JSON, missing paths, swapped reports, unresolved recovery, mismatched UIDs and corrupted event logs.
- The files remain **unauthenticated local evidence**. An attacker with write access can alter both reports and logs. This is not cryptographic attestation, policy enforcement proof or an independent current-state check.
- Before publishing any result, also run `scripts/v04/Audit-FiredoorV04WitnessRecovery.ps1` against the pinned owned kind context. The audit only establishes current witness absence; it cannot retroactively authenticate the experiment.
- Do not commit `docs/evidence/local/` or raw `*.jsonl` journals containing local machine paths. For a public demo, create separately reviewed, sanitized fixtures that are explicitly labeled illustrative.

## Commands

Run source and repository checks:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice4.ps1
```

Validate and display the two actual saved live reports in your `docs/evidence/local/` folder (read-only):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Show-FiredoorV04Attribution.ps1
```

To use the CLI directly, pass the exact previously saved filenames in four explicit flags:

```text
firedoor attribution --edge-report <path.json> --edge-journal <path.jsonl> \
  --data-report <path.json> --data-journal <path.jsonl> --format human|json
```

## Acceptance criteria for the next and final slice

1. Full Go and CLI regressions pass on Windows with user toolchain and actual repository.
2. Two saved authentic local lab reports import and produce two `MATCH` rows.
3. Both witness policies absent in the independent read-only audit, no recovery lock.
4. `git diff --check` clean; `.gitattributes` LF policy reviewed; stage only intended source/docs, not local evidence.
5. Final README example, CI gate, commit on `feat/v04-policy-attribution`, PR to `main`, clean local branch after merge.
