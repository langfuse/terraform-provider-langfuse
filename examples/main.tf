terraform {
  required_version = ">= 1.5"

  required_providers {
    langfuse = {
      source  = "langfuse/langfuse"
      version = ">= 0.1.0"
    }
  }
}

# Where your Langfuse instance lives
variable "host" {
  type        = string
  description = "Base URL of the Langfuse control plane."
}

# Admin-level API key.  If you prefer, just export LANGFUSE_ADMIN_KEY instead of passing this variable.
variable "admin_api_key" {
  type        = string
  sensitive   = true
  description = "Admin API key for the Langfuse host. Optional when LANGFUSE_ADMIN_KEY is set."
  default     = null
}

provider "langfuse" {
  host          = var.host
  admin_api_key = var.admin_api_key
}

resource "langfuse_organization" "org" {
  name = "ExampleCorp"
  
  metadata = {
    environment = "production"
    team        = "platform"
    cost_center = "engineering"
    region      = "us-east-1"
  }
}

# Import an existing organization
import {
  to = langfuse_organization.existing_org
  id = "1"
}

resource "langfuse_organization" "existing_org" {
  name = "Existing Corp"
  
  metadata = {
    environment = "production"
    team        = "platform"
    cost_center = "engineering"
    region      = "us-east-1"
  }
}

resource "langfuse_organization_api_key" "org_key" {
  organization_id = langfuse_organization.org.id
}

resource "langfuse_project" "project" {
  name                     = "example-project"
  organization_id          = langfuse_organization.org.id
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
  retention_days           = 90
  
  metadata = {
    environment     = "production"
    application     = "chatbot"
    owner_team      = "ai-engineering"
    data_retention  = "quarterly"
    compliance_tier = "high"
  }
}

resource "langfuse_project_api_key" "project_key" {
  project_id = langfuse_project.project.id
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}

output "org_api_secret_key" {
  value     = langfuse_organization_api_key.org_key.secret_key
  sensitive = true
}

output "org_api_public_key" {
  value     = langfuse_organization_api_key.org_key.public_key
  sensitive = true
}

output "project_api_secret_key" {
  value     = langfuse_project_api_key.project_key.secret_key
  sensitive = true
}

output "project_api_public_key" {
  value     = langfuse_project_api_key.project_key.public_key
  sensitive = true
}
