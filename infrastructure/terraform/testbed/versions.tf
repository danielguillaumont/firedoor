terraform {
  required_version = ">= 1.17.0, < 1.18.0"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
}
