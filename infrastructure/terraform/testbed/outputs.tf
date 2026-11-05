output "active_profile" {
  description = "Desired Terraform profile, NOT an observed containment verdict."
  value       = var.security_profile
}

output "owned_namespaces" {
  description = "Namespaces provisioned and tracked by this one state."
  value       = { for zone, workload in module.workloads : zone => workload.namespace_name }
}

output "services" {
  description = "DNS names of demonstration HTTP services (not production applications)."
  value       = { for zone, workload in module.workloads : zone => workload.service_dns }
}

output "managed_ingress_policies" {
  description = "Stable NetworkPolicy resource names across profile changes."
  value       = { for zone, boundary in module.network_boundaries : zone => boundary.name }
}
