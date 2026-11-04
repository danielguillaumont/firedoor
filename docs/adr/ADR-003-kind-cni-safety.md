# ADR-003 — Local kind testbed, CNI and failure policy

Status: Accepted for v0.0 design (implementation v0.1)

## Decision
Use a dedicated named kind cluster only, plus an explicitly selected, version-pinned CNI known to enforce Kubernetes NetworkPolicy. The cluster uses synthetic workloads in `edge`, `app`, `data`, `firedoor-system` namespaces.

## Gates
Before declaring any boundary: healthy destination; working DNS; verified CNI; independent allowed connectivity; independent denied connectivity; correct identity/port/source; collected object snapshot. Otherwise verdict `inconclusive`.

## Teardown
Destroy Terraform resources before deleting the kind cluster. Ensure a failed partial setup can be inspected and cleaned up with explicit owner verification.

This document is not an authorization to create a cluster in v0.0.
