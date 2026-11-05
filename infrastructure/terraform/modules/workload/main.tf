# A deliberately lightweight HTTP-only test workload per zone.
# This is a synthetic lab endpoint, not a real edge proxy, API, or database.

locals {
  name = "firedoor-${var.zone}"
  labels = {
    "app.kubernetes.io/part-of" = "firedoor"
    "app.kubernetes.io/name"    = local.name
    "firedoor.dev/managed-by"   = "terraform"
    "firedoor.dev/zone"         = var.zone
  }
}

resource "kubernetes_namespace_v1" "this" {
  metadata {
    name = local.name
    labels = merge(local.labels, {
      "pod-security.kubernetes.io/enforce" = "baseline"
    })
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "kubernetes_service_account_v1" "this" {
  metadata {
    name      = "${local.name}-sa"
    namespace = kubernetes_namespace_v1.this.metadata[0].name
    labels    = local.labels
  }
  automount_service_account_token = false
}

resource "kubernetes_deployment_v1" "this" {
  metadata {
    name      = local.name
    namespace = kubernetes_namespace_v1.this.metadata[0].name
    labels    = local.labels
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app.kubernetes.io/name" = local.name
      }
    }

    template {
      metadata {
        labels = local.labels
      }
      spec {
        service_account_name            = kubernetes_service_account_v1.this.metadata[0].name
        automount_service_account_token = false

        container {
          name              = "demo-http"
          image             = "busybox:1.36.1"
          image_pull_policy = "IfNotPresent"
          command           = ["sh", "-c"]
          args              = ["mkdir -p /tmp/firedoor; echo FIREDOOR_${upper(var.zone)}_OK > /tmp/firedoor/index.html; exec httpd -f -p 8080 -h /tmp/firedoor"]

          port {
            name           = "http"
            container_port = 8080
          }

          readiness_probe {
            http_get {
              path = "/"
              port = 8080
            }
            initial_delay_seconds = 1
            period_seconds        = 3
          }

          resources {
            requests = {
              cpu    = "10m"
              memory = "16Mi"
            }
            limits = {
              cpu    = "100m"
              memory = "64Mi"
            }
          }

          security_context {
            allow_privilege_escalation = false
            run_as_non_root            = true
            run_as_user                = 10001
            capabilities {
              drop = ["ALL"]
            }
          }
        }
      }
    }
  }

  wait_for_rollout = true
}

resource "kubernetes_service_v1" "this" {
  metadata {
    name      = local.name
    namespace = kubernetes_namespace_v1.this.metadata[0].name
    labels    = local.labels
  }

  spec {
    selector = {
      "app.kubernetes.io/name" = local.name
    }

    port {
      name        = "http"
      port        = 8080
      target_port = "8080"
      protocol    = "TCP"
    }
    type = "ClusterIP"
  }
}
