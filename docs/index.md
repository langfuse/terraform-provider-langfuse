---
page_title: "Langfuse Provider"
description: |-
  Manage Langfuse organizations, projects, API keys, memberships, and LLM connections.
---

# Langfuse Provider

Manage [Langfuse](https://langfuse.com) organizations, projects, API keys, memberships, and LLM connections. The provider works with Langfuse Cloud and self-hosted Langfuse, but the organization resources are self-hosted only.

## Authentication

Resources authenticate with different keys:

- `langfuse_organization` and `langfuse_organization_api_key` use the [Instance Management API](https://langfuse.com/self-hosting/administration/instance-management-api). Set `admin_api_key` (or `LANGFUSE_ADMIN_KEY`) to the `ADMIN_API_KEY` configured on your instance. This API is only available on self-hosted Langfuse with an Enterprise Edition license, not on Langfuse Cloud.
- `langfuse_project`, `langfuse_project_api_key`, `langfuse_organization_membership`, and `langfuse_project_membership` use an [organization-scoped API key](https://langfuse.com/docs/administration/scim-and-org-api), passed to each resource as `organization_public_key` and `organization_private_key`.
- `langfuse_llm_connection` uses a project API key, passed as `project_public_key` and `project_secret_key`.

### Langfuse Cloud

On Langfuse Cloud, omit `admin_api_key` from the provider block. Create an organization-scoped API key in your organization settings and pass it to the project, project API key, and membership resources. `langfuse_organization` and `langfuse_organization_api_key` can't be used on Cloud.

```terraform
provider "langfuse" {
  host = "https://cloud.langfuse.com" # or https://us.cloud.langfuse.com
}

resource "langfuse_project" "project" {
  name                     = "example-project"
  organization_id          = var.organization_id
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_secret_key
}
```

## Example Usage

This example targets a self-hosted instance and creates the organization and its API key through the Instance Management API.

```terraform
terraform {
  required_version = ">= 1.5"

  required_providers {
    langfuse = {
      source  = "langfuse/langfuse"
      version = "~> 0.8"
    }
  }
}

# Where your Langfuse instance lives
variable "host" {
  type        = string
  description = "Base URL of the Langfuse control plane."
}

# Instance Management API key, i.e. the ADMIN_API_KEY configured on a self-hosted
# Langfuse instance. Only needed for langfuse_organization and
# langfuse_organization_api_key. The Instance Management API is not available on
# Langfuse Cloud; omit admin_api_key there and pass an organization API key to the
# resources instead. You can also export LANGFUSE_ADMIN_KEY instead of passing this variable.
variable "admin_api_key" {
  type        = string
  sensitive   = true
  description = "Instance Management API key of the self-hosted Langfuse instance. Optional when LANGFUSE_ADMIN_KEY is set."
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
  project_id               = langfuse_project.project.id
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `admin_api_key` (String, Sensitive) Instance Management API key, i.e. the ADMIN_API_KEY configured on a self-hosted Langfuse instance. Only needed for langfuse_organization and langfuse_organization_api_key. The Instance Management API is not available on Langfuse Cloud, so omit this there. Can also come from LANGFUSE_ADMIN_KEY.
- `host` (String) Base URI of the Langfuse instance (defaults to https://app.langfuse.com).
- `tls_server_name` (String) Hostname to verify the TLS certificate against, also sent as the SNI value. Set this when the instance is reached through a port forward or tunnel, where the host above (typically localhost) cannot match the certificate. Certificate verification remains enabled.
