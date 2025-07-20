.PHONY: test bench bench-all lint coverage clean all help test-examples demo demo-list demo-run demo-all build install run-example try

# Default target
all: test lint

# Display help
help:
	@echo "pipz Development Commands"
	@echo "========================"
	@echo ""
	@echo "Quick Start:"
	@echo "  make try          - Run interactive demo menu (alias for 'make demo')"
	@echo ""
	@echo "Interactive Demos:"
	@echo "  make demo         - Run interactive demo menu"
	@echo "  make demo-list    - List available demos"
	@echo "  make demo-run DEMO=name - Run a specific demo"
	@echo "                      Examples: make demo-run DEMO=payment"
	@echo "                               make demo-run DEMO=etl"
	@echo "  make demo-all     - Run all demos sequentially"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test         - Run all tests with race detector"
	@echo "  make test-examples- Run tests for all examples"
	@echo "  make bench        - Run core library benchmarks"
	@echo "  make bench-all    - Run all benchmarks (core + examples)"
	@echo "  make lint         - Run linters"
	@echo "  make lint-fix     - Run linters with auto-fix"
	@echo "  make coverage     - Generate coverage report (HTML)"
	@echo "  make check        - Run tests and lint (quick check)"
	@echo ""
	@echo "Build & Install:"
	@echo "  make build        - Build the CLI tool (./pipz)"
	@echo "  make install      - Install pipz CLI to GOPATH/bin"
	@echo "  make clean        - Clean generated files"
	@echo ""
	@echo "Other:"
	@echo "  make run-example EXAMPLE=name - Run an example's main.go"
	@echo "  make install-tools- Install required development tools"
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
	@go test -bench=. -benchmem -benchtime=1s ./...

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

# Build the CLI tool
build:
	@echo "Building pipz CLI..."
	@cd cmd && go build -o ../pipz .

# Install the CLI tool to GOPATH/bin
install: build
	@echo "Installing pipz CLI..."
	@go install ./cmd/...

# Run interactive demo menu
demo: build
	@echo "Starting interactive demo..."
	@./pipz demo

# Quick start - alias for demo
try: demo

# List available demos
demo-list: build
	@./pipz demo --help | grep -A20 "Available examples:" || ./pipz demo --help

# Run a specific demo (usage: make demo-run DEMO=validation)
demo-run: build
	@if [ -z "$(DEMO)" ]; then \
		echo "Usage: make demo-run DEMO=<demo-name>"; \
		echo "Available demos:"; \
		./pipz demo --help | grep -A20 "Available examples:" | grep "  " || true; \
	else \
		echo "Running $(DEMO) demo..."; \
		./pipz demo $(DEMO); \
	fi

# Run all demos sequentially
demo-all: build
	@echo "Running all demos..."
	@./pipz demo --all

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
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated: coverage.html"

# Clean generated files
clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html pipz
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

# CI simulation - what CI runs
ci: clean lint test coverage bench
	@echo "CI simulation complete!"