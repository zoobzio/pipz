# Integration Tests for pipz

This directory contains integration tests that verify how pipz components work together in real-world scenarios. These tests focus on component interactions, complex pipeline compositions, and end-to-end workflows.

## Test Categories

### Pipeline Flow Tests (`pipeline_flows_test.go`)
Tests fundamental pipeline composition patterns:
- Sequential processing chains
- Branching and conditional flows  
- Dynamic pipeline modification
- Error propagation through complex pipelines

### Resilience Pattern Tests (`resilience_patterns_test.go`)
Tests how resilience patterns work together:
- Circuit breaker + retry combinations
- Rate limiting with backoff strategies
- Timeout protection across pipeline stages
- Fallback chains and error recovery

### Real-World Scenario Tests (`real_world_test.go`)
Tests realistic application scenarios:
- API client with full resilience stack
- Event processing with validation and transformation
- Batch processing with error handling
- Data validation and enrichment pipelines

### Performance Integration Tests (`performance_integration_test.go`)
Tests performance characteristics of complex pipelines:
- Memory usage of long pipeline chains
- Concurrent processing efficiency
- Resource cleanup and garbage collection
- Scaling behavior under load

## Running Integration Tests

### All Integration Tests
```bash
go test -v ./testing/integration/...
```

### With Race Detection
```bash
go test -v -race ./testing/integration/...
```

### Specific Test Categories
```bash
# Test pipeline flows
go test -v -run TestPipelineFlows ./testing/integration/...

# Test resilience patterns
go test -v -run TestResilience ./testing/integration/...

# Test real-world scenarios
go test -v -run TestRealWorld ./testing/integration/...
```

### Performance Integration Tests
```bash
# Run with extended timeout for performance tests
go test -v -timeout=5m -run TestPerformanceIntegration ./testing/integration/...
```

## Test Data and Fixtures

Integration tests use shared test data types and fixtures:

### Common Test Types
- `TestUser`: Represents user data for validation pipelines
- `TestOrder`: Represents order processing scenarios
- `TestEvent`: Represents event processing scenarios

### Test Fixtures
- Small, focused data sets for specific scenarios
- Realistic data that exercises edge cases
- Consistent across different test files

## Guidelines for Adding Integration Tests

### When to Add Integration Tests
- Testing interactions between 3+ components
- Testing complex error scenarios
- Verifying end-to-end workflows
- Testing performance characteristics

### Test Structure
1. **Setup**: Create realistic test data and pipeline components
2. **Execute**: Run the complete workflow
3. **Verify**: Check both intermediate states and final results
4. **Cleanup**: Ensure proper resource cleanup

### Naming Conventions
- `TestPipelineFlows_<Scenario>`
- `TestResilience_<Pattern>`
- `TestRealWorld_<UseCase>`
- `TestPerformanceIntegration_<Aspect>`

### Best Practices
- Use table-driven tests for multiple scenarios
- Test both success and failure paths
- Include timeout and cancellation scenarios
- Verify proper error propagation
- Check resource cleanup and memory usage

## Integration Test Dependencies

### Required
- Go 1.21+ for generics
- Context support for cancellation testing

### Optional
- Memory profiling tools for performance tests
- Load testing utilities for stress testing

## Performance Considerations

- Integration tests should complete in seconds, not minutes
- Use realistic but small data sets
- Focus on interaction patterns rather than raw throughput
- Include memory usage verification for long pipelines

## Debugging Integration Tests

### Verbose Output
```bash
go test -v -run TestSpecificCase ./testing/integration/...
```

### With Test Tracing
```bash
go test -v -trace=trace.out ./testing/integration/...
go tool trace trace.out
```

### Memory Profiling
```bash
go test -memprofile=mem.prof ./testing/integration/...
go tool pprof mem.prof
```

---

Integration tests ensure that pipz components work correctly together and provide confidence that real-world usage patterns will function as expected.