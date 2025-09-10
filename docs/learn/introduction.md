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
- **Performance**: Optimized design with minimal allocations

## Core Philosophy

pipz follows these principles:

1. **Interface-First Design**: Everything implements `Chainable[T]` - a single, simple interface
2. **Your Code, Your Way**: Implement the interface directly or use our convenience wrappers
3. **Composition over Configuration**: Build complex behavior by combining simple pieces
4. **Type Safety First**: No `interface{}`, no runtime type assertions
5. **Errors are Values**: Explicit error handling at every step
6. **Context Awareness**: Full support for cancellation and timeouts

## Key Concepts

### The Chainable Interface
The foundation of pipz - everything implements this simple interface:
```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Name() string
}
```

Any type implementing this interface can be used in a pipeline. This gives you complete flexibility:
- Implement it directly for custom processors
- Use the provided wrapper functions for common patterns
- Mix both approaches in the same pipeline

### Processors
The atomic units that transform data. You can create them by:
1. **Direct Implementation**: Implement `Chainable[T]` for full control
2. **Wrapper Functions**: Use `Transform`, `Apply`, `Effect`, etc. for convenience

### Connectors
Mutable components that combine any `Chainable[T]` implementations into more complex behaviors:
- `NewSequence`: Run processors in order
- `NewSwitch`: Route to different processors based on conditions
- `NewConcurrent`: Run multiple processors in parallel
- `NewRace`: Use the first successful result
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
- Includes battle-tested patterns (retry, timeout, fallback)
- Returns rich error context showing the exact failure path
- Supports both declarative and dynamic pipeline construction
- Treats **errors as data flowing through pipelines** - use the same Switch, Concurrent, Sequence patterns for sophisticated error recovery

## A Unique Approach to Error Handling

Most frameworks treat errors as exceptions or callbacks. pipz treats them as **data that flows through pipelines**. This means you can build sophisticated error recovery flows using the exact same tools you use for regular data processing:

```go
// Error recovery pipeline - same tools, same patterns!
errorRecovery := pipz.NewSequence[*pipz.Error[Order]](PipelineErrorRecovery,
    pipz.Transform(ProcessorCategorize, categorizeError),
    pipz.NewSwitch(RouterSeverity, routeBySeverity),
    pipz.NewConcurrent(ConnectorParallelRecovery, notifyCustomer, updateInventory),
)

// Errors flow through this pipeline automatically
robustPipeline := pipz.NewHandle(HandleOrderProcessing, mainPipeline, errorRecovery)
```

This pattern enables type-safe, composable, and testable error handling that scales with your application complexity. See [Safety and Reliability](../guides/safety-reliability.md) for the full power of this approach.

## Next Steps

- [Quick Start](../tutorials/quickstart.md) - Build your first pipeline in minutes
- [Installation](../tutorials/installation.md) - Get pipz installed
- [Core Concepts](./core-concepts.md) - Deep dive into pipz concepts and composition patterns