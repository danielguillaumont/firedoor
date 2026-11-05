# Firedoor v0.1 Slice 3: a single root and single local state.
# Terraform owns only the three named Firedoor namespaces and their objects.
# Calico/kind and the disposable firedoor-netcheck namespace are NOT Terraform-owned.

locals {
  zones = toset(["edge", "app", "data"])

  allowed_sources = {
    app  = "edge"
    data = "app"
  }
}

module "workloads" {
  source   = "../modules/workload"
  for_each = local.zones

  zone = each.key
}

# The same two policy resources persist in Terraform state across both profiles.
# Baseline: an empty namespace selector matches ALL namespaces.
# Hardened: each policy allows only one source namespace by its managed label.
module "network_boundaries" {
  source   = "../modules/network-boundary"
  for_each = local.allowed_sources

  zone                = each.key
  namespace           = module.workloads[each.key].namespace_name
  allowed_source_zone = each.value
  security_profile    = var.security_profile
}
