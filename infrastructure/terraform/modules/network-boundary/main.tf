# Fixed resource address: baseline/hardened changes rule selectors, not states.
resource "kubernetes_network_policy_v1" "this" {
  metadata {
    name      = "firedoor-${var.zone}-ingress"
    namespace = var.namespace
    labels = {
      "app.kubernetes.io/part-of" = "firedoor"
      "firedoor.dev/managed-by"   = "terraform"
    }
  }

  spec {
    pod_selector {
      match_labels = {
        "app.kubernetes.io/name" = "firedoor-${var.zone}"
      }
    }

    ingress {
      from {
        namespace_selector {
          # Empty selector = all namespaces. NOT a Kubernetes default-deny profile.
          match_labels = var.security_profile == "baseline" ? {} : {
            "firedoor.dev/zone" = var.allowed_source_zone
          }
        }
      }

      ports {
        port     = "8080"
        protocol = "TCP"
      }
    }

    policy_types = ["Ingress"]
  }
}
