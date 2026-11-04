# kind / CNI bootstrap: not implemented in v0.0

v0.1 will add a dedicated kind config and pinned network-policy-enforcing CNI after reviewing compatibility. Do not assume Kubernetes' default networking enforces NetworkPolicy. Verify service health, an independently allowed path, and a blocked path before scenario verdicts.

Bootstrap owns cluster creation/deletion. Terraform owns long-lived resources. Teardown order: Terraform destroy, then kind delete.
