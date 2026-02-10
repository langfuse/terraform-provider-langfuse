terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse"
    }
  }
}

# Provider for Organization 1
provider "langfuse" {
  alias           = "org1"
  org_public_key  = var.org1_public_key
  org_private_key = var.org1_private_key
}

# Provider for Organization 2
provider "langfuse" {
  alias           = "org2"
  org_public_key  = var.org2_public_key
  org_private_key = var.org2_private_key
}

# Resources for Organization 1
resource "langfuse_project" "org1_project" {
  provider        = langfuse.org1
  name            = "Org 1 Project"
  organization_id = var.org1_id
}

resource "langfuse_organization_membership" "org1_member" {
  provider = langfuse.org1
  email    = "user@org1.com"
  role     = "MEMBER"
}

# Resources for Organization 2
resource "langfuse_project" "org2_project" {
  provider        = langfuse.org2
  name            = "Org 2 Project"
  organization_id = var.org2_id
}

resource "langfuse_organization_membership" "org2_member" {
  provider = langfuse.org2
  email    = "user@org2.com"
  role     = "MEMBER"
}

output "org1_project_id" {
  value = langfuse_project.org1_project.id
}

output "org2_project_id" {
  value = langfuse_project.org2_project.id
}
