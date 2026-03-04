terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse"
    }
  }
}

# Configure provider with organization-level credentials
# This allows resources to inherit these credentials instead of repeating them
provider "langfuse" {
  org_public_key  = var.org_public_key
  org_private_key = var.org_private_key
}

# Create project without needing to specify credentials
# Credentials are inherited from the provider
resource "langfuse_project" "example" {
  name            = "Example Project"
  organization_id = var.organization_id
  retention_days  = 30
}

# Create organization membership without needing to specify credentials
# Credentials are inherited from the provider
resource "langfuse_organization_membership" "member" {
  email = "user@example.com"
  role  = "MEMBER"
}

# Create project API key without needing to specify credentials
# Credentials are inherited from the provider
resource "langfuse_project_api_key" "example_key" {
  project_id = langfuse_project.example.id
}

output "project_id" {
  value = langfuse_project.example.id
}

output "project_api_public_key" {
  value     = langfuse_project_api_key.example_key.public_key
  sensitive = true
}
