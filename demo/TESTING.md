# Testing Guide

This document explains the testing strategy for pipz demos.

## Overview

The demos serve two purposes:
1. **Educational**: Interactive demonstrations for users (via CLI)
2. **Testing**: Automated tests for CI/CD (via `go test`)

## Test Structure

```
demo/
├── processors/          # Business logic with tests
│   ├── security.go
│   ├── security_test.go
│   ├── payment.go
│   ├── payment_test.go
│   └── ...
├── integration_test.go  # Integration tests
├── cmd_*.go            # Demo commands (user-facing)
└── Makefile            # Automation
```

## Running Tests

### For CI/CD

```bash
# Quick CI check (unit + integration)
make ci

# Full CI check (includes race detection)
make ci-full

# Just unit tests
make test-unit

# Just integration tests
make test-integration

# With coverage
make coverage
```

### Test Categories

#### Unit Tests (`processors/*_test.go`)
- Test individual business logic functions
- Fast execution (< 1ms per test)
- No external dependencies
- Example: `TestSecurityPipeline`, `TestPaymentValidation`

#### Integration Tests (`integration_test.go`)
- Test type universe isolation
- Test pipeline composition
- Test concurrent access
- Example: `TestTypeUniverseIsolation`, `TestPipelineVersioning`

## CI vs Demo Execution

### CI Tests (Fast, Automated)
```bash
# This runs actual Go tests
make ci
# Output: PASS/FAIL with coverage
```

### User Demos (Interactive, Educational)
```bash
# This runs the interactive CLI
make demo
# Output: Colorful terminal UI with explanations
```

## Key Principles

1. **Same Logic**: Tests exercise the exact same business logic as demos
2. **No Fluff**: Tests have no UI, animations, or pauses
3. **Fast**: Full test suite runs in < 1 second
4. **Isolated**: Each test creates its own type universe

## Writing New Tests

When adding new demo functionality:

1. **Extract business logic** to `processors/` package
2. **Write unit tests** alongside the logic
3. **Add integration tests** for type universe concepts
4. **Keep demos** for interactive user education

Example:
```go
// processors/newfeature.go
func ProcessNewFeature(data Data) (Data, error) {
    // Business logic here
}

// processors/newfeature_test.go
func TestProcessNewFeature(t *testing.T) {
    // Test the business logic
}

// cmd_newfeature.go
func runNewFeatureDemo(cmd *cobra.Command, args []string) {
    // Interactive demo using ProcessNewFeature
}
```

## Benchmarking

Run benchmarks to detect performance regressions:

```bash
make bench
```

Current baseline:
- Validation pipeline: ~187μs per operation
- Memory: ~42KB per operation

## Race Detection

Always run with race detector in CI:

```bash
make test-race
```

This catches concurrent access issues in the global pipeline registry.