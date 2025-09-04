.PHONY: test testacc test-setup test-teardown

# Run unit tests (fast, no external dependencies)
test:
	go test ./... -v -run "^Test[^A].*"

# Run acceptance tests (requires docker)
testacc: test-setup
	TF_ACC=1 LANGFUSE_HOST=http://localhost:3000 LANGFUSE_ADMIN_KEY=test_admin_key go test ./internal/provider -v -run TestAcc

# Run all tests (unit + acceptance)
test-all: test testacc

# Set up test environment
test-setup:
	@echo "Starting Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml up -d
	@echo "Waiting for Langfuse to be ready..."
	./scripts/wait-for-langfuse.sh localhost 3000 600 /

# Tear down test environment
test-teardown:
	@echo "Stopping Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml down -v

# Generate mocks
generate:
	go generate ./...