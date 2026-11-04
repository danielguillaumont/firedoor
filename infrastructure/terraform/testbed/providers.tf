# v0.0 placeholder: DO NOT APPLY. The provider will be used for long-lived
# Firedoor-owned resources beginning in v0.1 after cluster identity checks.
provider "kubernetes" {
  config_path    = pathexpand(var.kubeconfig_path)
  config_context = var.kube_context
}
