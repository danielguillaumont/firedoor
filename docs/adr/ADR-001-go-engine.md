# ADR-001 — Go engine and CLI

Status: Accepted for v0.0 design (implementation staged)

## Context
Firedoor needs typed Kubernetes API access, reproducible CLI output, explicit timeouts/cancellation, and one shared verdict implementation for all clients.

## Decision
Use one Go module with Cobra CLI (`cmd/firedoor`) and internal packages for doctor, cluster, controls, validation, evidence, API and orchestration as they become necessary. `client-go` will be introduced with a real cluster integration slice. Keep a modular monolith rather than multiple services or an operator.

## Consequences
Strong typing and native client-go support; initial Go learning curve. Do not add unneeded services, concurrent orchestration or a separate Python backend.
