variable "security_profile" {
  description = "Intent for a future, single-state, Firedoor-owned testbed. No resources exist at v0.0."
  type        = string
  default     = "baseline"

  validation {
    condition     = contains(["baseline", "hardened"], var.security_profile)
    error_message = "security_profile must be baseline or hardened."
  }
}

variable "kubeconfig_path" {
  description = "Dedicated kubeconfig path; not read until provider operations are required."
  type        = string
  default     = "~/.kube/config"
}

variable "kube_context" {
  description = "Expected dedicated local kind context. This alone is not an authorization guard."
  type        = string
  default     = "kind-firedoor-local"
}
