# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Resource: `langfuse_evaluator`**
  - Create, read, update, delete and import evaluators via the stable `/api/public/v2/evaluators` API
  - Supports `llm_as_judge` (prompt, model config, default variable mapping, structured output definition) and `code` (source code and runtime language) evaluators
  - Definition changes create a new version on the same stable `id`; name/description changes do not
  - Computed `version`, `version_id`, `status` and `variables` attributes
- **Resource: `langfuse_evaluation_rule`**
  - Create, read, update, delete and import evaluation rules via the stable `/api/public/v2/evaluation-rules` API
  - Typed `filter` conditions, `sampling`, `enabled`, and `evaluator_assignments` with optional per-rule variable mapping overrides
- Acceptance test coverage for both resources; the docker compose test stack now seeds a project with fixed API keys so project-scoped resources can be tested without an enterprise license

## [0.1.0] - 2025-08-26

### Added
- **Initial release** of the Terraform Provider for Langfuse
- **Provider Configuration**
  - `host` - Configurable Langfuse instance URL (defaults to https://app.langfuse.com)
  - `admin_api_key` - Admin API key for authentication (supports LANGFUSE_ADMIN_KEY env var)
- **Resource: `langfuse_organization`**
  - Create, read, update, and delete Langfuse organizations
  - Required `name` attribute for organization display name
  - Optional `metadata` attribute for key-value pairs (map of strings)
  - Computed `id` attribute containing organization identifier
  - Graceful handling of deletion when organization has existing projects
- **Resource: `langfuse_organization_api_key`**
  - Generate and manage API keys for organizations
  - Required `organization_id` attribute linking to parent organization
  - Computed sensitive `public_key` and `secret_key` attributes
  - API keys only returned at creation time (UseStateForUnknown plan modifier)
  - Automatic resource replacement when organization changes
- **Resource: `langfuse_project`**
  - Create, read, update, and delete projects within organizations
  - Required attributes: `name`, `organization_id`, `organization_public_key`, `organization_private_key`
  - Optional `retention_days` attribute for data retention configuration
  - Optional `metadata` attribute for key-value pairs (map of strings)
  - Computed `id` attribute containing project identifier
  - Uses organization client authentication for project operations
- **Resource: `langfuse_project_api_key`**
  - Generate and manage API keys for projects
  - Required attributes: `project_id`, `organization_public_key`, `organization_private_key`
  - Computed sensitive `public_key` and `secret_key` attributes
  - API keys only returned at creation time (UseStateForUnknown plan modifier)
  - Automatic resource replacement when project changes

### Technical Features
- **Terraform Plugin Framework** - Built using the modern Terraform Plugin Framework v1.15+
- **Client Architecture** - Modular client factory pattern with separate admin and organization clients
- **Mock Generation** - Complete mock infrastructure using gomock for unit testing
- **Comprehensive Testing**
  - Unit tests for all resources with mocked dependencies
  - Acceptance tests using real Langfuse instance via Docker Compose
  - Single comprehensive workflow test covering all resource dependencies
  - Test infrastructure with PostgreSQL, ClickHouse, Redis, and MinIO
  - Health check scripts for reliable test setup
- **Development Tools**
  - Makefile with test automation and environment management
  - Docker Compose setup for local development and testing
  - Automatic mock generation with `go generate`

### Documentation
- Comprehensive README.md with installation, configuration, and usage examples
- Detailed testing guide (TESTING.md) with unit and acceptance test instructions
- Complete resource documentation with all arguments and attributes
- Example Terraform configuration demonstrating full workflow

### Dependencies
- Go 1.24+ support
- Terraform >= 1.5 compatibility
- HashiCorp Terraform Plugin Framework v1.15.1
- HashiCorp Terraform Plugin Testing v1.13.3
- Enterprise license key requirement for admin operations

### Security
- Sensitive attribute handling for all API keys and credentials
- Environment variable support for secure credential management
- No API key values logged or exposed in terraform plan/apply output

---

**Full Changelog**: https://github.com/langfuse/terraform-provider-langfuse/commits/v0.1.0
