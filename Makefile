.PHONY: test bench bench-all lint coverage clean all help test-examples run-example test-integration test-benchmarks test-reliability test-all ci

# Default target
all: test lint

# Display help
help:
	@echo "pipz Development Commands"
	@echo "========================"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test            - Run unit tests with race detector"
	@echo "  make test-examples   - Run tests for all examples"
	@echo "  make test-integration- Run integration tests with race detector"
	@echo "  make test-benchmarks - Run performance benchmarks"
	@echo "  make test-reliability- Run reliability/resilience tests"
	@echo "  make test-all        - Run all test suites (unit + integration + reliability)"
	@echo "  make bench           - Run core library benchmarks"
	@echo "  make bench-all       - Run all benchmarks (core + examples)"
	@echo "  make lint            - Run linters"
	@echo "  make lint-fix        - Run linters with auto-fix"
	@echo "  make coverage        - Generate coverage report (HTML)"
	@echo "  make check           - Run tests and lint (quick check)"
	@echo "  make ci              - Full CI simulation (all tests + quality checks)"
	@echo ""
	@echo "Other:"
	@echo "  make run-example EXAMPLE=name - Run an example's main.go"
	@echo "  make install-tools- Install required development tools"
	@echo "  make clean        - Clean generated files"
	@echo "  make all          - Run tests and lint (default)"

# Run tests with race detector
test:
	@echo "Running core tests..."
	@go test -v -race ./...

# Run tests for all examples
test-examples:
	@echo "Running example tests..."
	@for dir in examples/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Testing $$dir"; \
			(cd "$$dir" && go test -v -race ./...); \
		fi \
	done

# Run core benchmarks
bench:
	@echo "Running core benchmarks..."
	@go test -bench=. -benchmem -benchtime=1s .

# Run all benchmarks including examples
bench-all:
	@echo "Running all benchmarks..."
	@echo "=== Core Library Benchmarks ==="
	@go test -bench=. -benchmem -benchtime=1s ./...
	@echo ""
	@for dir in examples/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "=== Benchmarks for $$dir ==="; \
			(cd "$$dir" && go test -bench=. -benchmem -benchtime=1s ./... 2>/dev/null) || true; \
			echo ""; \
		fi \
	done


# Run a specific example's main.go (usage: make run-example EXAMPLE=validation)
run-example:
	@if [ -z "$(EXAMPLE)" ]; then \
		echo "Usage: make run-example EXAMPLE=<example-name>"; \
		echo "Available examples:"; \
		ls -1 examples/ | grep -v README.md; \
	else \
		if [ -f "examples/$(EXAMPLE)/main.go" ]; then \
			echo "Running $(EXAMPLE) example..."; \
			(cd examples/$(EXAMPLE) && go run .); \
		else \
			echo "No main.go found in examples/$(EXAMPLE)/"; \
			echo "This example might not have a standalone runner."; \
		fi \
	fi

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run --config=.golangci.yml --timeout=5m

# Run linters with auto-fix
lint-fix:
	@echo "Running linters with auto-fix..."
	@golangci-lint run --config=.golangci.yml --fix

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out $$(go list ./... | grep -v '/examples/')
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated: coverage.html"

# Clean generated files
clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html
	@find . -name "*.test" -delete
	@find . -name "*.prof" -delete
	@find . -name "*.out" -delete

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

# Quick check - run tests and lint
check: test lint
	@echo "All checks passed!"

# Integration tests - component interaction verification
test-integration:
	@echo "Running integration tests..."
	@go test -v -race -timeout=10m ./testing/integration/...

# Benchmark tests - performance regression detection
test-benchmarks:
	@echo "Running all benchmarks..."
	@go test -v -bench=. -benchmem -benchtime=1s -timeout=10m ./testing/benchmarks/...

# Reliability tests - resilience pattern verification
test-reliability:
	@echo "Running reliability tests..."
	@go test -v -race -timeout=10m -run TestResilience ./testing/integration/...
	@go test -v -race -timeout=5m -run TestPanicRecovery ./testing/integration/...
	@go test -v -race -timeout=10m -run TestResourceLeak ./testing/integration/...
	@go test -v -race -timeout=5m -run TestConcurrentModification ./testing/integration/...

# Comprehensive test suite - all tests with race detection
test-all: test test-integration test-reliability
	@echo "All test suites completed!"

# CI simulation - what CI runs locally
ci: clean lint test test-integration test-benchmarks test-reliability coverage
	@echo "Full CI simulation complete!"