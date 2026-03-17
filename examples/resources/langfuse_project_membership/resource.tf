# Project membership management example

terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse"
    }
  }
}

provider "langfuse" {
  # You can set the host if using a self-hosted instance
  # host = "https://your-langfuse-instance.com"

  # Admin API key for organization management
  # Can also be set via LANGFUSE_ADMIN_KEY environment variable
  # admin_api_key = "your-admin-api-key"
}

# Create an organization
resource "langfuse_organization" "example_org" {
  name = "My Organization"
  metadata = {
    department = "engineering"
    region     = "us-east-1"
  }
}

# Create an organization API key
resource "langfuse_organization_api_key" "example_org_key" {
  organization_id = langfuse_organization.example_org.id
}

# Create a project within the organization
resource "langfuse_project" "example_project" {
  name                     = "example-project"
  organization_id          = langfuse_organization.example_org.id
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
  retention_days           = 90
}

# Add a user to the project as an admin
resource "langfuse_project_membership" "admin_user" {
  project_id               = langfuse_project.example_project.id
  email                    = "admin@example.com"
  role                     = "ADMIN"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Add a user to the project as a member
resource "langfuse_project_membership" "member_user" {
  project_id               = langfuse_project.example_project.id
  email                    = "member@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Add a user to the project as a viewer
resource "langfuse_project_membership" "viewer_user" {
  project_id               = langfuse_project.example_project.id
  email                    = "viewer@example.com"
  role                     = "VIEWER"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Add multiple users to the project using for_each
resource "langfuse_project_membership" "team" {
  for_each = toset([
    "dev1@example.com",
    "dev2@example.com",
    "qa@example.com"
  ])

  project_id               = langfuse_project.example_project.id
  email                    = each.value
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Outputs
output "project_id" {
  description = "The ID of the created project"
  value       = langfuse_project.example_project.id
}

output "admin_membership_id" {
  description = "The ID of the admin user membership"
  value       = langfuse_project_membership.admin_user.id
}

output "admin_user_id" {
  description = "The user ID of the admin user"
  value       = langfuse_project_membership.admin_user.user_id
}

output "member_user_id" {
  description = "The user ID of the member user"
  value       = langfuse_project_membership.member_user.user_id
}

output "viewer_user_id" {
  description = "The user ID of the viewer user"
  value       = langfuse_project_membership.viewer_user.user_id
}
