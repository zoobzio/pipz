# pipz Cookbook

Practical recipes for common data processing patterns using pipz.

## Recipes

### [Resilient API Calls](./resilient-api-calls.md)
Build production-ready API integrations with automatic retry, circuit breaking, and timeouts. Learn how to layer resilience patterns for external service calls.

**You'll learn:**
- Retry strategies with exponential backoff
- Circuit breaker configuration
- Rate limiting best practices
- Timeout protection
- Fallback mechanisms

---

### [Data Validation Pipeline](./data-validation-pipeline.md)
Create comprehensive data validation pipelines with clear error messages and progressive enhancement. Handle complex validation rules elegantly.

**You'll learn:**
- Multi-layer validation strategies
- Cross-field validation
- Optional enrichment patterns
- Error reporting for APIs
- Performance optimization

---

### [Event Processing System](./event-processing.md)
Build scalable event processing pipelines with routing, enrichment, and error handling. Process events from multiple sources efficiently.

**You'll learn:**
- Event routing by type
- Parallel event distribution
- Dead letter queue patterns
- Event deduplication
- Stream processing techniques

---

### [ETL Pipelines](./etl-pipelines.md)
Create robust Extract-Transform-Load pipelines for data processing at scale. Handle large datasets with error recovery and monitoring.

**You'll learn:**
- Multi-source extraction
- Data transformation patterns
- Batch processing strategies
- Change data capture (CDC)
- Error recovery mechanisms

---

### [Bounded Parallelism](./bounded-parallelism.md)
Control resource usage with WorkerPool for predictable parallel processing. Handle rate-limited APIs and resource-constrained environments efficiently.

**You'll learn:**
- WorkerPool configuration strategies
- API rate limit management
- Database connection pooling
- Memory-constrained processing
- Dynamic worker adjustment

---

### [Common Patterns](./patterns.md)
Additional patterns and techniques for advanced pipz usage, including custom connectors and specialized processing strategies.

**You'll learn:**
- Bulkhead pattern implementation
- Custom processor creation
- Advanced error handling
- Performance optimization
- Testing strategies

## Quick Pattern Reference

### Resilience Patterns

```go
// Complete resilient setup
resilient := pipz.NewRateLimiter("rate",
    pipz.NewCircuitBreaker("breaker",
        pipz.NewTimeout("timeout",
            pipz.NewRetry("retry", processor, 3),
            5*time.Second,
        ),
        5, 30*time.Second,
    ),
    100, time.Second,
)
```

### Parallel Processing

```go
// Unbounded parallel operations
concurrent := pipz.NewConcurrent[T]("parallel",
    sendEmail,
    updateDatabase,
    publishEvent,
    recordMetrics,
)

// Bounded parallel operations (e.g., rate-limited API)
pool := pipz.NewWorkerPool[T]("limited", 5,
    callAPI1,
    callAPI2,
    callAPI3,
    // ... many more, but only 5 run concurrently
)
```

### Conditional Processing

```go
// Route based on conditions
router := pipz.NewSwitch[T]("router", routeFunc).
    AddRoute("premium", premiumPipeline).
    AddRoute("standard", standardPipeline).
    AddDefault(defaultPipeline)
```

### Error Handling

```go
// Handle errors with cleanup
withCleanup := pipz.NewHandle("cleanup",
    riskyOperation,
    pipz.Effect("release", releaseResources),
)
```

## Best Practices

1. **Use constants for processor names** - Easier debugging and tracing
2. **Implement Clone() properly** - Deep copy all reference types
3. **Respect context cancellation** - Check context in long operations
4. **Create singletons for stateful connectors** - RateLimiter, CircuitBreaker
5. **Test error paths** - Verify your error handling works
6. **Monitor pipeline health** - Add metrics and logging
7. **Document your pipelines** - Clear names and comments

## Getting Help

- Check the [API Reference](../reference/) for detailed documentation
- Read the [Core Concepts](../learn/core-concepts.md) for understanding
- Review the [Testing Guide](../guides/testing.md) for testing strategies
- See the [Cheatsheet](../reference/cheatsheet.md) for quick reference

## Contributing Recipes

Have a useful pattern? We welcome contributions! 

1. Create a new markdown file in `/docs/cookbook/`
2. Follow the existing recipe format
3. Include practical, runnable examples
4. Add tests for your patterns
5. Update this README with your recipe

Submit a pull request with your recipe and we'll review it for inclusion.