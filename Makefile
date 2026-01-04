.PHONY: test test-unit test-integration test-bench test-reliability test-all bench bench-all lint lint-fix coverage clean all check ci install-tools install-hooks help

.DEFAULT_GOAL := help

all: test lint ## Run tests and lint

## Testing & Quality
test: ## Run all tests with race detector
	@go test -v -race ./...

test-unit: ## Run unit tests only (short mode)
	@go test -v -race -short ./...

test-integration: ## Run integration tests with race detector
	@go test -v -race -timeout=10m ./testing/integration/...

test-bench: ## Run performance benchmarks
	@go test -v -bench=. -benchmem -benchtime=100ms -timeout=15m ./testing/benchmarks/...

test-reliability: ## Run reliability/resilience tests
	@go test -v -race -timeout=10m -run TestResilience ./testing/integration/...
	@go test -v -race -timeout=5m -run TestPanicRecovery ./testing/integration/...
	@go test -v -race -timeout=10m -run TestResourceLeak ./testing/integration/...
	@go test -v -race -timeout=5m -run TestConcurrentModification ./testing/integration/...

test-all: test test-integration test-reliability ## Run all test suites
	@echo "All test suites completed!"

bench: ## Run core library benchmarks
	@go test -bench=. -benchmem -benchtime=100ms -timeout=15m .

bench-all: ## Run all benchmarks
	@go test -bench=. -benchmem -benchtime=100ms -timeout=15m ./...

lint: ## Run linters
	@golangci-lint run --config=.golangci.yml --timeout=5m

lint-fix: ## Run linters with auto-fix
	@golangci-lint run --config=.golangci.yml --fix

coverage: ## Generate coverage report (HTML)
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated: coverage.html"

check: test lint ## Quick validation (test + lint)
	@echo "All checks passed!"

ci: clean lint test test-integration test-bench test-reliability coverage ## Full CI simulation
	@echo "Full CI simulation complete!"

## Setup
install-tools: ## Install required development tools
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2

install-hooks: ## Install git pre-commit hooks
	@mkdir -p .git/hooks
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make check' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed"

## Other
clean: ## Remove generated files
	@rm -f coverage.out coverage.html
	@find . -name "*.test" -delete
	@find . -name "*.prof" -delete
	@find . -name "*.out" -delete

help: ## Display available commands
	@echo "pipz Development Commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
