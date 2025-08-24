# Testing Infrastructure for pipz

This directory contains the comprehensive testing infrastructure for the pipz package, designed to support robust development and maintenance of pipeline components.

## Directory Structure

```
testing/
├── README.md             # This file - testing strategy overview
├── helpers.go            # Shared test utilities and mocks
├── integration/          # Integration and end-to-end tests
│   ├── README.md        # Integration testing documentation
│   ├── pipeline_flows_test.go      # Core pipeline composition tests
│   ├── resilience_patterns_test.go # Resilience and failure handling
│   ├── real_world_test.go          # Real-world scenario tests
│   └── performance_integration_test.go # Performance integration scenarios
└── benchmarks/          # Performance benchmarks
    ├── README.md        # Benchmark documentation
    ├── core_performance_test.go     # Core component benchmarks
    ├── composition_performance_test.go # Pipeline composition benchmarks
    └── comparison_test.go           # Performance vs traditional approaches

```

## Testing Strategy

### Unit Tests (Root Package)
- **Location**: Alongside source files (`*_test.go`)
- **Purpose**: Test individual components in isolation
- **Coverage Goal**: >95% for core business logic
- **Current Coverage**: 97.8% (excellent baseline)

### Integration Tests (`testing/integration/`)
- **Purpose**: Test component interactions and real-world pipeline compositions
- **Scope**: Multi-component workflows, complex scenarios, failure patterns
- **Focus**: Ensure components work together correctly under various conditions

### Benchmarks (`testing/benchmarks/`)
- **Purpose**: Measure and track performance characteristics
- **Scope**: Individual components, composition patterns, real-world scenarios
- **Focus**: Prevent performance regressions and optimize hot paths

### Test Helpers (`testing/helpers.go`)
- **Purpose**: Provide reusable testing utilities for pipz users
- **Scope**: MockProcessor, assertion helpers, chaos testing tools
- **Focus**: Make testing pipz-based applications easier and more thorough

## Running Tests

### All Tests
```bash
# Run all tests with coverage
go test -v -coverprofile=coverage.out ./...

# Generate coverage report
go tool cover -html=coverage.out
```

### Unit Tests Only
```bash
# Run only unit tests (root package)
go test -v .
```

### Integration Tests Only
```bash
# Run integration tests
go test -v ./testing/integration/...
```

### Benchmarks Only
```bash
# Run all benchmarks
go test -v -bench=. ./testing/benchmarks/...

# Run specific benchmark category
go test -v -bench=BenchmarkCore ./testing/benchmarks/...
```

### Performance Monitoring
```bash
# Compare benchmarks over time
go test -bench=. -count=5 ./testing/benchmarks/... > current.txt
# ... after changes ...
go test -bench=. -count=5 ./testing/benchmarks/... > new.txt
benchcmp current.txt new.txt
```

## Test Dependencies

### Required for Basic Testing
- Go 1.21+ (for generics)
- Standard library only for most tests

### Required for Advanced Testing
- `testify` (if used in integration tests)
- Race detector: `go test -race`
- Coverage tools: `go tool cover`

### Optional Performance Tools
- `benchcmp` for comparing benchmark results
- `pprof` for performance profiling

## Test Data and Fixtures

### Test Data Patterns
- Use table-driven tests for multiple scenarios
- Create test data that represents real-world usage
- Include edge cases and boundary conditions
- Use consistent test data across integration tests

### Fixture Guidelines
- Keep fixtures small and focused
- Make fixtures self-documenting
- Use builder patterns for complex test data
- Avoid external file dependencies where possible

## Best Practices

### Test Organization
1. **Hierarchical naming**: Use descriptive test names that form a hierarchy
2. **Consistent structure**: Follow the same pattern across all test files
3. **Isolated tests**: Each test should be completely independent
4. **Fast tests**: Keep unit tests fast, put slow tests in integration

### Error Testing
1. **Test both success and failure paths**
2. **Verify error messages and types**
3. **Test error propagation through pipelines**
4. **Include timeout and cancellation scenarios**

### Concurrency Testing
1. **Use `-race` flag regularly**
2. **Test concurrent access patterns**
3. **Verify proper cleanup in failure scenarios**
4. **Test context cancellation behavior**

### Performance Testing
1. **Benchmark realistic workloads**
2. **Include both CPU and memory allocation metrics**
3. **Test scaling behavior**
4. **Compare against baseline implementations**

## Continuous Integration

### Pre-commit Checks
```bash
# Run this before committing
make test-all     # All tests with race detection
make lint         # Code quality checks  
make coverage     # Coverage verification
```

### CI Pipeline Requirements
- All tests must pass
- Coverage must remain >95% for main package
- No race conditions detected
- Benchmarks must not regress significantly

## Contributing Test Infrastructure

### Adding New Tests
1. Determine appropriate test category (unit/integration/benchmark)
2. Follow existing naming conventions
3. Update relevant README files
4. Ensure tests are deterministic and fast

### Adding New Helpers
1. Add to `testing/helpers.go`
2. Include comprehensive documentation
3. Provide usage examples
4. Ensure thread safety if applicable

### Performance Considerations
- Unit tests should complete in milliseconds
- Integration tests should complete in seconds
- Benchmarks should run long enough for stable measurements
- Avoid sleeps in tests unless testing timing behavior

---

This testing infrastructure ensures pipz remains reliable, performant, and easy to use while providing comprehensive tools for users to test their own pipz-based applications.