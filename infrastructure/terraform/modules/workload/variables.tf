variable "zone" {
  description = "Firedoor zone identifier for a dedicated, owned namespace."
  type        = string

  validation {
    condition     = contains(["edge", "app", "data"], var.zone)
    error_message = "Only edge, app, and data zones are allowed."
  }
}
