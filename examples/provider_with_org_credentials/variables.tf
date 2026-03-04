variable "org_public_key" {
  description = "Organization public key"
  type        = string
  sensitive   = true
}

variable "org_private_key" {
  description = "Organization private key"
  type        = string
  sensitive   = true
}

variable "organization_id" {
  description = "Organization ID"
  type        = string
}
