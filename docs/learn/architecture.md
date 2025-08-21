# Architecture

## Overview

pipz is designed as a composable data processing library built on a single, uniform interface. The architecture emphasizes type safety, immutability, and clean separation of concerns to enable developers to build maintainable and testable data pipelines.

## Core Design Principles

1. **Single Interface Pattern**: Everything implements `Chainable[T]`, enabling seamless composition
2. **Type Safety**: Leverages Go generics (1.21+) for compile-time type checking
3. **Immutable Processors**: Adapter functions are immutable values ensuring thread safety
4. **Mutable Connectors**: Container types that manage state and configuration
5. **Fail-Fast Execution**: Processing stops at the first error, simplifying error handling
6. **Context-Aware**: All operations support context for cancellation and timeout control

## System Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         Application                          │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│                    Chainable[T] Interface                    │
│                 Process(ctx, T) → (T, *Error)                │
│                        Name() → Name                         │
└────────────┬─────────────────────────────┬───────────────────┘
             │                             │
             ▼                             ▼
┌────────────────────────────┐ ┌──────────────────────────────┐
│      Processors (Values)   │ │    Connectors (Pointers)     │
├────────────────────────────┤ ├──────────────────────────────┤
│ • Transform - Pure function│ │ • Sequence - Sequential flow │
│ • Apply - Can fail         │ │ • Concurrent - Parallel exec │
│ • Effect - Side effects    │ │ • Race - First success       │
│ • Mutate - Conditional     │ │ • Contest - First meeting    │
│ • Enrich - Optional enhance│ │ • Switch - Conditional branch│
│ • Filter - Pass/block data │ │ • Fallback - Error recovery │
│ • Handle - Error transform │ │ • Retry - Retry on failure   │
│ • Scaffold - Development   │ │ • CircuitBreaker - Fail fast │
│                            │ │ • RateLimiter - Control flow │
│                            │ │ • Timeout - Time boundaries  │
└────────────────────────────┘ └──────────────────────────────┘
```

## Component Relationships

### Processors (Adapters)

Processors are lightweight wrappers around user functions that implement the `Chainable` interface:

```go
type processor[T any] struct {
    name Name
    fn   func(context.Context, T) (T, error)
}
```

Key characteristics:
- **Immutable**: Once created, cannot be modified
- **Stateless**: No internal state, pure function wrappers
- **Thread-Safe**: Can be used concurrently without synchronization
- **Composable**: Can be combined using connectors

### Connectors (Composition)

Connectors manage the composition and execution flow of multiple Chainables:

```go
type connector[T any] struct {
    name       Name
    processors []Chainable[T]
    // Additional state (mutex, config, etc.)
}
```

Key characteristics:
- **Mutable**: Can be modified at runtime (add/remove processors)
- **Stateful**: May maintain internal state (circuit breaker state, rate limits)
- **Configurable**: Support runtime configuration changes
- **Orchestrators**: Control execution flow and error handling

## Data Flow Architecture

### Sequential Processing

```
Input → [Processor 1] → [Processor 2] → [Processor 3] → Output
         ↓ error          ↓ error          ↓ error
         Return           Return           Return
```

The `Sequence` connector processes data through each step sequentially, stopping at the first error.

### Parallel Processing

```
        ┌→ [Processor 1] →┐
Input →─┼→ [Processor 2] →┼→ Aggregation → Output
        └→ [Processor 3] →┘
```

Parallel connectors (`Concurrent`, `Race`, `Contest`) require `T` to implement `Cloner[T]` for safe concurrent processing.

### Conditional Processing

```
         ┌─[condition]─→ [Branch A] →┐
Input →─┤                            ├→ Output
         └─[default]──→ [Branch B] →┘
```

The `Switch` connector routes data based on conditions, similar to a switch statement.

### Execution Flow Diagram

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Context   │────→│  Pipeline   │────→│  Processor  │
└─────────────┘     └─────────────┘     └─────────────┘
      │                    │                    │
      │                    ▼                    ▼
      │             ┌─────────────┐     ┌─────────────┐
      └────────────→│   Router    │────→│  Transform  │
                    └─────────────┘     └─────────────┘
                           │                    │
                           ▼                    ▼
                    ┌─────────────┐     ┌─────────────┐
                    │Error Handler│←────│   Result    │
                    └─────────────┘     └─────────────┘
```

## Error Handling Architecture

### Error Type Hierarchy

```go
type Error[T any] struct {
    Stage Name    // Where the error occurred
    Cause error   // The underlying error
    State T       // Data state at error time
}
```

Error handling patterns:

1. **Fail-Fast**: Default behavior, stop on first error
2. **Recovery**: Use `Fallback` for error recovery with alternate processing
3. **Transformation**: Use `Handle` to transform errors into valid data
4. **Resilience**: Use `Retry`, `CircuitBreaker`, `RateLimiter` for fault tolerance

## Memory Model

### Cloner Interface

For concurrent processing, data must be cloneable:

```go
type Cloner[T any] interface {
    Clone() T
}
```

This ensures:
- Thread safety in parallel operations
- Data isolation between concurrent branches
- Prevention of race conditions

### Context Propagation

Every operation receives a context, enabling:
- Request-scoped values
- Cancellation propagation
- Timeout enforcement
- Tracing and monitoring integration

## Extension Points

### Custom Processors

Create custom processors by wrapping functions:

```go
func CustomProcessor[T any](name Name, fn func(context.Context, T) (T, error)) Chainable[T] {
    return Apply(name, fn)
}
```

### Custom Connectors

Implement the `Chainable` interface for custom composition logic:

```go
type CustomConnector[T any] struct {
    // Your fields
}

func (c *CustomConnector[T]) Process(ctx context.Context, data T) (T, error) {
    // Your logic
}

func (c *CustomConnector[T]) Name() Name {
    return c.name
}
```

### Integration Points

Common integration patterns:

1. **HTTP Middleware**: Wrap pipelines as HTTP handlers
2. **Message Queue Consumers**: Process messages through pipelines
3. **Batch Processing**: Use pipelines in batch job frameworks
4. **Stream Processing**: Integrate with streaming platforms
5. **Service Mesh**: Use as sidecar processing logic

## Performance Considerations

### Optimization Strategies

1. **Processor Granularity**: Balance between too many small processors (overhead) and large monolithic ones (reduced reusability)
2. **Parallel Execution**: Use `Concurrent` for independent operations
3. **Early Filtering**: Place `Filter` processors early to reduce downstream processing
4. **Resource Pooling**: Reuse expensive resources across pipeline executions
5. **Context Timeouts**: Set appropriate timeouts to prevent hanging operations

### Benchmarking Results

The library includes comprehensive benchmarks showing:
- Minimal overhead for processor wrapping (~2-5ns)
- Linear scaling for sequential processing
- Near-linear scaling for parallel processing with proper data isolation

## Security Considerations

### Input Validation

Always validate input at pipeline boundaries:

```go
pipeline := NewSequence("secure",
    Transform("sanitize", sanitizeInput),
    Apply("validate", validateData),
    // ... rest of pipeline
)
```

### Resource Limits

Protect against resource exhaustion:
- Use `Timeout` for time boundaries
- Use `RateLimiter` for throughput control
- Use `CircuitBreaker` for cascading failure prevention

### Error Information

Be careful with error details in production:
- The `Error[T]` type includes the data state at failure
- Consider sanitizing sensitive data in error states
- Use structured logging for audit trails

## Future Architecture Considerations

### Planned Enhancements

1. **Distributed Execution**: Support for distributed pipeline execution
2. **Persistent State**: Durable state management for long-running pipelines
3. **Visual Pipeline Builder**: Tool for visual pipeline composition
4. **Metrics Collection**: Built-in observability and metrics
5. **Schema Evolution**: Support for data schema versioning

### API Stability

The core `Chainable[T]` interface is stable and will remain backward compatible. New features will be added through:
- New adapter functions
- New connector types
- Optional interfaces for advanced features
- Configuration options on existing types

## Summary

The pipz architecture provides a clean, composable foundation for building data processing pipelines. By adhering to a single interface and clear separation between processors and connectors, it enables developers to build complex data flows from simple, testable components while maintaining type safety and performance.