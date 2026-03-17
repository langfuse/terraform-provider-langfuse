# Terraform Provider for Langfuse

A Terraform provider for managing [Langfuse](https://langfuse.com) resources programmatically.

Langfuse is an open-source LLM engineering platform that provides observability, analytics, prompt management, and evaluations for LLM applications. This provider allows you to manage organizations, projects, and API keys using Infrastructure as Code (IaC) principles.

## Features

- 🏢 **Organization Management** - Create and manage Langfuse organizations
- 🔑 **API Key Management** - Generate and manage organization and project API keys
- 📦 **Project Management** - Create and configure projects within organizations
- 🤖 **LLM Connections Management** - Configure and manage LLM API connections (OpenAI, Bedrock, Vertex AI, etc.)
- 🛡️ **Enterprise Support** - Full support for Langfuse Enterprise features
- ⚡ **Terraform Integration** - Native integration with Terraform workflows

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.5
- [Go](https://golang.org/doc/install) >= 1.24 (for development)
- Enterprise license key (if managing organizations and organization api keys)

## Installation

### Terraform Registry (Recommended)

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_providers {
    langfuse = {
      source  = "langfuse/langfuse"
      version = "~> 0.1.0"
    }
  }
}
```

### Local Development

For development and testing:

```bash
# Clone the repository
git clone https://github.com/langfuse/terraform-provider-langfuse
cd terraform-provider-langfuse

# Build the provider
go build -o terraform-provider-langfuse

```

## Configuration

### Provider Configuration

```hcl
provider "langfuse" {
  host          = "https://cloud.langfuse.com"  # Optional, defaults to https://app.langfuse.com
  admin_api_key = var.admin_api_key             # Optional, can use LANGFUSE_ADMIN_KEY env var
}
```

### Environment Variables

- `LANGFUSE_ADMIN_KEY` - Admin API key (alternative to `admin_api_key`)
- `LANGFUSE_EE_LICENSE_KEY` - Enterprise license key (required for admin operations)

## Usage

### Complete Example

```hcl
terraform {
  required_providers {
    langfuse = {
      source  = "langfuse/langfuse"
      version = "~> 0.1.0"
    }
  }
}

# Variables for configuration
variable "host" {
  type        = string
  description = "Base URL of the Langfuse control plane"
  default     = "https://cloud.langfuse.com"
}

variable "admin_api_key" {
  type        = string
  sensitive   = true
  description = "Admin API key for Langfuse (or set LANGFUSE_ADMIN_KEY)"
}

# Configure the provider
provider "langfuse" {
  host          = var.host
  admin_api_key = var.admin_api_key
}

# Create an organization
resource "langfuse_organization" "example" {
  name = "My Organization"
}

# Create organization API keys
resource "langfuse_organization_api_key" "example" {
  organization_id = langfuse_organization.example.id
}

# Create a project within the organization
resource "langfuse_project" "example" {
  name            = "my-project"
  organization_id = langfuse_organization.example.id
  retention_days  = 90  # Optional: data retention period

  organization_public_key  = langfuse_organization_api_key.example.public_key
  organization_private_key = langfuse_organization_api_key.example.secret_key
}

# Create project API keys
resource "langfuse_project_api_key" "example" {
  project_id = langfuse_project.example.id

  organization_public_key  = langfuse_organization_api_key.example.public_key
  organization_private_key = langfuse_organization_api_key.example.secret_key
}

# Output the API keys (marked as sensitive)
output "org_public_key" {
  value     = langfuse_organization_api_key.example.public_key
  sensitive = true
}

output "project_secret_key" {
  value     = langfuse_project_api_key.example.secret_key
  sensitive = true
}
```

## Resources

### `langfuse_organization`

Manages Langfuse organizations.

#### Arguments

- `name` (String, Required) - The display name of the organization

#### Attributes

- `id` (String) - The unique identifier of the organization

### `langfuse_organization_api_key`

Manages API keys for organizations.

#### Arguments

- `organization_id` (String, Required) - The ID of the organization

#### Attributes

- `id` (String) - The unique identifier of the API key
- `public_key` (String, Sensitive) - The public API key value
- `secret_key` (String, Sensitive) - The secret API key value

**Note:** API key values are only returned during creation and cannot be retrieved later.

### `langfuse_project`

Manages projects within organizations.

#### Arguments

- `name` (String, Required) - The display name of the project
- `organization_id` (String, Required) - The ID of the parent organization
- `organization_public_key` (String, Required, Sensitive) - Organization public key for authentication
- `organization_private_key` (String, Required, Sensitive) - Organization private key for authentication
- `retention_days` (Number, Optional) - Data retention period in days. If not set or 0, data is stored indefinitely

#### Attributes

- `id` (String) - The unique identifier of the project

### `langfuse_project_api_key`

Manages API keys for projects.

#### Arguments

- `project_id` (String, Required) - The ID of the project
- `organization_public_key` (String, Required, Sensitive) - Organization public key for authentication
- `organization_private_key` (String, Required, Sensitive) - Organization private key for authentication

#### Attributes

- `id` (String) - The unique identifier of the API key
- `public_key` (String, Sensitive) - The public API key value
- `secret_key` (String, Sensitive) - The secret API key value

### `langfuse_organization_membership`

Manages organization membership - invites users to organizations and manages their roles. This resource automatically creates users in the Langfuse system via the SCIM endpoint if they don't already exist.

#### Arguments

- `email` (String, Required, ForceNew) - The email address of the user to add to the organization
- `role` (String, Required) - The role to assign to the user. Valid values:`OWNER`, `ADMIN`, `MEMBER`, `VIEWER` or `NONE`
- `organization_public_key` (String, Required, Sensitive, ForceNew) - Organization public key for authentication
- `organization_private_key` (String, Required, Sensitive, ForceNew) - Organization private key for authentication

#### Attributes

- `id` (String) - The unique identifier of the membership
- `user_id` (String) - The unique identifier of the user
- `status` (String) - The status of the membership (e.g., "ACTIVE")
- `username` (String) - The username of the user

#### Behavior

- **Automatic User Creation**: If the user doesn't exist in the organization, the resource automatically creates them using the SCIM endpoint before adding them to the organization
- **Role Updates**: The role can be updated after creation using Terraform `apply` with the updated role value
- **Deletion**: When the resource is destroyed, the user is removed from the organization (but not deleted from the Langfuse system)
- **Resource ID**: The resource ID is set to the user's `userId` from the Langfuse system, which uniquely identifies the membership within the organization

#### Example Usage

```hcl
# Create organization membership with automatic user creation
resource "langfuse_organization_membership" "engineer" {
  email                    = "engineer@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}

# Update user role
resource "langfuse_organization_membership" "admin" {
  email                    = "admin@example.com"
  role                     = "ADMIN"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}

# Multiple users in organization
resource "langfuse_organization_membership" "team" {
  for_each = toset([
    "dev1@example.com",
    "dev2@example.com",
    "qa@example.com"
  ])

  email                    = each.value
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}
```

### `langfuse_project_membership`

Manages project membership - adds users to projects and manages their project-level roles. Users must already exist in the organization before being added to a project.

#### Arguments

- `project_id` (String, Required, ForceNew) - The ID of the project to add the user to
- `email` (String, Required, ForceNew) - The email address of the user to add to the project
- `role` (String, Required) - The role to assign to the user. Valid values: `OWNER`, `ADMIN`, `MEMBER`, `VIEWER` or `NONE`
- `organization_public_key` (String, Required, Sensitive, ForceNew) - Organization public key for authentication
- `organization_private_key` (String, Required, Sensitive, ForceNew) - Organization private key for authentication

#### Attributes

- `id` (String) - The unique identifier of the project membership
- `user_id` (String) - The unique identifier of the user
- `name` (String) - The name of the user

#### Behavior

- **Role Updates**: The role can be updated after creation using Terraform `apply` with the updated role value
- **Deletion**: When the resource is destroyed, the user is removed from the project (but not from the organization)
- **Import Format**: `project_id,user_id,organization_public_key,organization_private_key`

```hcl
# Add a user to a project as admin
resource "langfuse_project_membership" "admin" {
  project_id               = langfuse_project.project.id
  email                    = "admin@example.com"
  role                     = "ADMIN"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}

# Add a user to a project as member
resource "langfuse_project_membership" "member" {
  project_id               = langfuse_project.project.id
  email                    = "member@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
}

# Add multiple users to a project
resource "langfuse_project_membership" "team" {
  for_each = toset([
    "dev1@example.com",
    "dev2@example.com",
    "qa@example.com"
  ])

  project_id               = langfuse_project.project.id
  email                    = each.value
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.org_key.public_key
  organization_private_key = langfuse_organization_api_key.org_key.secret_key
```

### `langfuse_llm_connection`

Manages LLM API connections for a Langfuse project. Supports OpenAI, Bedrock, Azure, Google Vertex AI, and custom providers. Each connection is uniquely identified by its `provider` name. Connections are upserted (created or updated) via the provider name; destroying the resource only removes it from Terraform state (the connection remains in Langfuse).

#### Arguments

- `project_public_key` (String, Required, Sensitive) - Project public key for authentication
- `project_secret_key` (String, Required, Sensitive) - Project secret key for authentication
- `provider_name` (String, Required, ForceNew) - Unique name for the LLM connection within the project. Changing this value destroys and recreates the resource.
- `adapter` (String, Required) - LLM adapter type. Valid values: `anthropic`, `openai`, `azure`, `bedrock`, `google-vertex-ai`, `google-ai-studio`
- `secret_key` (String, Required, Sensitive) - API key for the LLM provider
- `base_url` (String, Optional) - Custom base URL for the LLM API
- `custom_models` (List of String, Optional) - List of custom model names
- `extra_headers` (Map of String, Optional, Sensitive) - Additional HTTP headers for LLM API requests
- `with_default_models` (Bool, Optional, Computed) - Whether to include default models (defaults to true)
- `config` (String, Optional) - Adapter-specific configuration as a JSON string

#### Attributes

- `id` (String) - The unique identifier of the LLM connection (set to the provider value)
- `adapter` (String) - The LLM adapter type
- `provider_name` (String) - The provider name
- `base_url` (String) - The base URL used
- `custom_models` (List of String) - Custom model names
- `with_default_models` (Bool) - Whether default models are included
- `config` (String) - Adapter-specific config as JSON
- `created_at` (String) - Timestamp when the connection was created
- `updated_at` (String) - Timestamp when the connection was last updated

#### Behavior

- **Upsert Semantics**: Creating or updating this resource always upserts the connection in Langfuse by provider name. Changing `provider` destroys and recreates the resource.
- **No Delete Endpoint**: Destroying the resource only removes it from Terraform state; the connection remains in Langfuse.
- **Write-only Fields**: Sensitive fields like `secret_key` and `extra_headers` are preserved from the plan/state and not returned by the API.
- **Config Validation**: Adapter-specific config requirements are enforced:
  - **Bedrock**: `config` must be JSON with a `region` key, e.g. `{ "region": "us-east-1" }`
  - **Google Vertex AI**: `config` may be omitted, but if provided must include a `location` key, e.g. `{ "location": "us-central1" }`
  - **Other adapters**: `config` must be null or omitted
- **Pagination**: Read operation paginates through all LLM connections to find the target provider.

#### Example Usage

```hcl
# OpenAI connection
resource "langfuse_llm_connection" "openai_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "openai-prod"
  adapter             = "openai"
  secret_key          = "sk-..."
  with_default_models = true
}

# Bedrock connection with custom config
resource "langfuse_llm_connection" "bedrock_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "bedrock-prod"
  adapter             = "bedrock"
  secret_key          = "AK..."
  base_url            = "https://bedrock.aws.example.com"
  custom_models       = ["my-bedrock-model"]
  config              = jsonencode({ region = "us-east-1" })
  with_default_models = false
}

# Google Vertex AI connection
resource "langfuse_llm_connection" "vertex_ai_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "vertex-ai-prod"
  adapter             = "google-vertex-ai"
  secret_key          = "vertex-key"
  config              = jsonencode({ location = "us-central1" })
  with_default_models = true
}

# Azure connection
resource "langfuse_llm_connection" "azure_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "azure-prod"
  adapter             = "azure"
  secret_key          = "azure-key"
  with_default_models = true
}

# Custom provider with extra headers
resource "langfuse_llm_connection" "my_gateway" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "my-gateway"
  adapter             = "openai"
  secret_key          = "sk-custom"
  extra_headers       = {
    "X-My-Header" = "custom-value"
  }
  with_default_models = true
}
```

## Development

### Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/langfuse/terraform-provider-langfuse
   cd terraform-provider-langfuse
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Generate mocks (for testing):
   ```bash
   make generate
   ```

### Testing

The project includes comprehensive unit and integration tests.

#### Unit Tests

Run fast unit tests with mocked dependencies:

```bash
make test
```

#### Acceptance Tests

Run integration tests against a real Langfuse instance:

```bash
# Set required environment variable
export LANGFUSE_EE_LICENSE_KEY="your_license_key"

# Run acceptance tests (starts Docker environment)
make testacc

# Clean up test environment
make test-teardown
```

For detailed testing instructions, see [TESTING.md](TESTING.md).

### Building

```bash
# Build for current platform
go build -o terraform-provider-langfuse

# Build for multiple platforms
goreleaser build --snapshot --clean
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes and add tests
4. Run tests: `make test-all`
5. Commit your changes: `git commit -am 'Add new feature'`
6. Push to the branch: `git push origin my-feature`
7. Create a Pull Request

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add unit tests for new functionality
- Update documentation as needed

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

- 📚 [Langfuse Documentation](https://langfuse.com/docs)
- 🐛 [Report Issues](https://github.com/langfuse/terraform-provider-langfuse/issues)
- 💬 [Community Discussions](https://github.com/langfuse/terraform-provider-langfuse/discussions)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release notes and version history.
