# Introduction to pipz

pipz is a Go library for building type-safe, composable data pipelines. It leverages Go's generics to provide compile-time type safety while offering a simple, functional API for complex data transformations.

## Why pipz?

### The Problem

Building data processing pipelines in Go often involves:
- Writing boilerplate code for error handling
- Managing complex control flow (retries, timeouts, fallbacks)
- Dealing with `interface{}` and runtime type assertions
- Difficulty in testing individual components
- Hard-to-reuse processing logic

### The Solution

pipz provides:
- **Type Safety**: Full compile-time type checking with generics
- **Composability**: Small, reusable components that combine easily
- **Error Handling**: Built-in patterns for retries, fallbacks, and recovery
- **Testability**: Each component is independently testable
- **Performance**: Zero-allocation design for core operations

## Core Philosophy

pipz follows these principles:

1. **Everything is a Processor**: All components implement the same simple interface
2. **Composition over Configuration**: Build complex behavior by combining simple pieces
3. **Type Safety First**: No `interface{}`, no runtime type assertions
4. **Errors are Values**: Explicit error handling at every step
5. **Context Awareness**: Full support for cancellation and timeouts

## Key Concepts

### Processors
The atomic units that transform data:
```go
type Processor[T any] interface {
    Process(context.Context, T) (T, error)
}
```

### Connectors
Functions that combine processors into more complex behaviors:
- `Sequential`: Run processors in order
- `Switch`: Route to different processors based on conditions
- `Concurrent`: Run multiple processors in parallel
- `Race`: Use the first successful result
- And many more...

### Pipelines
Managed sequences of processors with introspection and modification capabilities.

## Use Cases

pipz excels at:
- ETL (Extract, Transform, Load) pipelines
- API request/response processing
- Event stream processing
- Payment processing with failover
- Content moderation pipelines
- Data validation workflows
- Microservice orchestration

## What Makes pipz Different?

Unlike traditional pipeline libraries, pipz:
- Uses Go generics for complete type safety
- Requires no code generation or reflection
- Has zero external dependencies
- Provides both functional and object-oriented APIs
- Includes battle-tested patterns (retry, timeout, circuit breaker)

## Next Steps

- [Quick Start](./quick-start.md) - Build your first pipeline in minutes
- [Installation](./installation.md) - Get pipz installed
- [Concepts](./concepts/processors.md) - Deep dive into pipz concepts