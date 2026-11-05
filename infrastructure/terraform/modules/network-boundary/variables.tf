variable "zone" {
  type = string
  validation {
    condition     = contains(["app", "data"], var.zone)
    error_message = "Only app or data may have a boundary in this slice."
  }
}

variable "namespace" {
  type = string
}

variable "allowed_source_zone" {
  type = string
  validation {
    condition     = contains(["edge", "app"], var.allowed_source_zone)
    error_message = "Source zone must be edge or app."
  }
}

variable "security_profile" {
  type = string
  validation {
    condition     = contains(["baseline", "hardened"], var.security_profile)
    error_message = "Invalid profile."
  }
}
