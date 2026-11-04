# Original 15 control requirements — mapping baseline

IDs are retained from the v1.1 plan. New Firedoor IDs will be introduced only alongside implemented evaluators and an explicit mapping table (v0.2 onward).

| Legacy ID | Required control | Domain |
|---|---|---|
| C-001 | Dedicated workload ServiceAccount | Identity |
| C-002 | Disable unnecessary SA token automount | Identity |
| C-003 | Least-privilege Kubernetes RBAC | Identity |
| C-004 | Default-deny network profile | Network |
| C-005 | Required ingress/egress exceptions | Network |
| C-006 | Run as non-root | Workload |
| C-007 | Disable privilege escalation | Workload |
| C-008 | Disallow privileged containers | Admission |
| C-009 | Minimize Linux capabilities | Workload |
| C-010 | Read-only root filesystem where compatible | Workload |
| C-011 | Pod Security labels/standards | Admission |
| C-012 | Appropriate seccomp profile | Workload |
| C-013 | Narrow secret permissions and mounts | Secrets |
| C-014 | Resource requests and limits | Resilience |
| C-015 | Pinned/controlled images; optional scan evidence | Supply chain |

Presence of a configured control does **not** imply that its security goal is behaviorally enforced.
