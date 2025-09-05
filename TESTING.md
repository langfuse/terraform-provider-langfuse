# Testing Guide

This document describes how to run tests for the Terraform Langfuse Provider.

## Unit Tests

Unit tests use mocks and don't require external dependencies. They test individual resource logic in isolation:

```bash
make test
# or
go test ./... -v -run "^Test[^A].*"
```

Unit tests are located in `*_unit_test.go` files and include:
- Metadata validation
- Schema validation  
- CRUD operations with mocked dependencies
- Fast execution (< 1 second per test)

## Acceptance Tests

Acceptance tests run against a real Langfuse instance using Docker Compose.

### Test Strategy

We use a **single comprehensive workflow test** (`TestAccLangfuseWorkflow`) that tests all resources in their natural dependency order:

1. **Organization** (foundational resource)
2. **Organization API Key** (requires organization)  
3. **Project** (requires organization API key)
4. **Project API Key** (requires project and organization API key)

This approach provides:
- ✅ **Realistic testing** of actual user workflows
- ✅ **Dependency validation** between resources
- ✅ **End-to-end integration** testing
- ✅ **Faster execution** with single setup/teardown
- ✅ **Better coverage** of cross-resource interactions
- ✅ **Import functionality** validation for existing resources

### Prerequisites

- Docker and Docker Compose
- `curl` (for health checks)
- **Langfuse Enterprise License Key** - Required for admin API access
  - Set the `LANGFUSE_EE_LICENSE_KEY` environment variable
  - Contact your team lead or check AWS Secrets Manager for the key

### Running Acceptance Tests

First, set your enterprise license key:

```bash
export LANGFUSE_EE_LICENSE_KEY="your_actual_license_key_here"
```

Then run the tests using the Makefile:

```bash
make testacc
```

This will:
1. Start the Langfuse test environment using Docker Compose
2. Wait for Langfuse to be healthy
3. Run the acceptance tests
4. Leave the environment running (use `make test-teardown` to clean up)

To run both unit and acceptance tests:

```bash
make test-all
```

### Manual Steps

If you prefer to run steps manually:

```bash
# 1. Start the test environment
make test-setup

# 2. Run acceptance tests
TF_ACC=1 LANGFUSE_HOST=http://localhost:3000 LANGFUSE_ADMIN_KEY=test_admin_key \
  go test ./internal/provider -v -run TestAcc

# 3. Clean up
make test-teardown
```

### Environment Variables

The acceptance tests require these environment variables:

- `TF_ACC=1` - Enables acceptance testing
- `LANGFUSE_HOST` - Base URL of the Langfuse instance (default: http://localhost:3000)
- `LANGFUSE_ADMIN_KEY` - Admin API key for authentication

### Test Infrastructure

The test setup uses:

- **PostgreSQL 15** - Primary database
- **ClickHouse 23.3** - Analytics database  
- **Redis 7** - Caching layer
- **MinIO** - S3-compatible object storage
- **Langfuse** - Main application

All services run in Docker containers with health checks to ensure they're ready before tests run.

## Test Structure

### Unit Tests
- Located in `internal/provider/*_unit_test.go` files
- Use gomock for mocking dependencies  
- Fast execution, no external dependencies
- Test individual resource logic in isolation

### Acceptance Tests  
- Located in `internal/provider/provider_acceptance_test.go`
- Use the `terraform-plugin-testing` framework
- Test against real Terraform configurations and live Langfuse instance
- Verify actual resource lifecycle (Create, Read, Update, Delete, Import)

### Test Utilities
- `testdata/docker-compose.yml` - Test environment definition
- `scripts/wait-for-langfuse.sh` - Health check script

## Troubleshooting

### Tests fail with "connection refused"
The Langfuse service might not be ready yet. The `wait-for-langfuse.sh` script should handle this, but you can manually check:

```bash
curl http://localhost:3000/api/health
```

### Docker containers won't start
Check Docker logs:

```bash
docker compose -f testdata/docker-compose.yml logs
```

### Port conflicts
If port 3000 is already in use, you can modify the docker-compose.yml to use a different port and update the `LANGFUSE_HOST` environment variable accordingly.
