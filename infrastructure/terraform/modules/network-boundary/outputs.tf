output "name" {
  value = kubernetes_network_policy_v1.this.metadata[0].name
}
