# Organization membership management example

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

# Invite a user to the organization as an admin
resource "langfuse_organization_membership" "admin_user" {
  email                    = "admin@example.com"
  role                     = "ADMIN"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Invite a user to the organization as a member
resource "langfuse_organization_membership" "member_user" {
  email                    = "member@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Invite a user to the organization as a viewer
resource "langfuse_organization_membership" "viewer_user" {
  email                    = "viewer@example.com"
  role                     = "VIEWER"
  organization_public_key  = langfuse_organization_api_key.example_org_key.public_key
  organization_private_key = langfuse_organization_api_key.example_org_key.secret_key
}

# Outputs
output "organization_id" {
  description = "The ID of the created organization"
  value       = langfuse_organization.example_org.id
}

output "admin_membership_id" {
  description = "The ID of the admin user membership"
  value       = langfuse_organization_membership.admin_user.id
}

output "admin_membership_status" {
  description = "The status of the admin user membership"
  value       = langfuse_organization_membership.admin_user.status
}

output "member_membership_status" {
  description = "The status of the member user membership"
  value       = langfuse_organization_membership.member_user.status
}

output "viewer_membership_status" {
  description = "The status of the viewer user membership"
  value       = langfuse_organization_membership.viewer_user.status
}
