output "namespace_name" {
  value = kubernetes_namespace_v1.this.metadata[0].name
}

output "service_dns" {
  value = "${kubernetes_service_v1.this.metadata[0].name}.${kubernetes_namespace_v1.this.metadata[0].name}.svc.cluster.local:8080"
}
