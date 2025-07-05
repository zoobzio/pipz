# Makefile for pipz demos
# 
# Testing targets (for CI):
#   make test         - Run all tests
#   make ci           - Quick CI check
#   make ci-full      - Full CI check with race detection
#
# Demo targets (for users):
#   make demo         - Run all demos
#   make demo-security - Run specific demo
#   make demo-interactive - Run with typewriter effects

.PHONY: all test bench demo clean

# Run all tests
test:
	@echo "🧪 Running tests..."
	@go test ./... -v

# Run only unit tests (fast)
test-unit:
	@echo "⚡ Running unit tests..."
	@go test ./processors -v

# Run only integration tests
test-integration:
	@echo "🔗 Running integration tests..."
	@go test . -v

# Run benchmarks
bench:
	@echo "📊 Running benchmarks..."
	@go test ./... -bench=. -benchmem

# Run demo (non-interactive for CI)
demo:
	@echo "🎭 Running demos..."
	@go run . all

# Run interactive demo
demo-interactive:
	@echo "🎭 Running interactive demos..."
	@go run . all -i

# Run specific demo
demo-%:
	@echo "🎯 Running $* demo..."
	@go run . $*

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@go clean
	@rm -f coverage.out

# Run tests with coverage
coverage:
	@echo "📈 Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out

# Quick CI check (tests only, no demos)
ci: test-unit test-integration
	@echo "✅ CI checks passed!"

# Full CI check with race detection
ci-full: test-unit test-integration test-race
	@echo "✅ Full CI checks passed!"

# Run tests with race detector
test-race:
	@echo "🏃 Running race detection..."
	@go test ./... -race -short