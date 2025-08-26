# Terraform Provider for Langfuse

A Terraform provider for managing [Langfuse](https://langfuse.com) resources programmatically.

Langfuse is an open-source LLM engineering platform that provides observability, analytics, prompt management, and evaluations for LLM applications. This provider allows you to manage organizations, projects, and API keys using Infrastructure as Code (IaC) principles.

## Features

- 🏢 **Organization Management** - Create and manage Langfuse organizations
- 🔑 **API Key Management** - Generate and manage organization and project API keys
- 📦 **Project Management** - Create and configure projects within organizations
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