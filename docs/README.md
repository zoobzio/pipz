# pipz Documentation

Welcome to pipz - a type-safe, composable pipeline library for Go.

## Start Here

- **[Getting Started](./tutorials/getting-started.md)** - Build your first pipeline in 10 minutes
- **[Core Concepts](./learn/core-concepts.md)** - Understand the fundamentals
- **[Cheatsheet](./reference/cheatsheet.md)** - Quick reference for decisions

## Learn

- **[Core Concepts](./learn/core-concepts.md)** - The mental model and fundamentals
- **[Architecture](./learn/architecture.md)** - System design and internals
- **[Introduction](./learn/introduction.md)** - Why pipz exists and what problems it solves

## Tutorials

- **[Getting Started](./tutorials/getting-started.md)** - Complete introduction with examples
- **[Installation](./tutorials/installation.md)** - Setup and requirements
- **[Quickstart](./tutorials/quickstart.md)** - Your first pipeline in 5 minutes
- **[First Pipeline](./tutorials/first-pipeline.md)** - Complete worked example

## Guides

- **[Connector Selection](./guides/connector-selection.md)** - Choose the right connector for your use case
- **[Testing](./guides/testing.md)** - pipz-specific testing patterns
- **[Performance](./guides/performance.md)** - Optimization and benchmarking
- **[Best Practices](./guides/best-practices.md)** - Production-ready patterns
- **[Clone Implementation](./guides/cloning.md)** - Implement Clone() correctly for concurrent processing

## Reference

### Quick Reference
- **[Cheatsheet](./reference/cheatsheet.md)** - Decision trees and common patterns

### Types
- [Error[T]](./reference/types/error.md) - Rich error context for pipeline failures

### Processors
- [Transform](./reference/processors/transform.md) - Pure transformations
- [Apply](./reference/processors/apply.md) - Fallible transformations
- [Effect](./reference/processors/effect.md) - Side effects
- [Mutate](./reference/processors/mutate.md) - Conditional modifications
- [Enrich](./reference/processors/enrich.md) - Optional enhancements
- [Handle](./reference/processors/handle.md) - Error observation
- [Scaffold](./reference/processors/scaffold.md) - Fire-and-forget execution

### Connectors
- [Sequence](./reference/connectors/sequence.md) - Sequential processing
- [Concurrent](./reference/connectors/concurrent.md) - Parallel execution
- [Race](./reference/connectors/race.md) - First success wins
- [Contest](./reference/connectors/contest.md) - First matching result
- [Switch](./reference/connectors/switch.md) - Conditional routing
- [Fallback](./reference/connectors/fallback.md) - Error recovery
- [Retry](./reference/connectors/retry.md) - Retry failures
- [Backoff](./reference/connectors/backoff.md) - Retry with exponential backoff
- [Filter](./reference/connectors/filter.md) - Conditional processing
- [CircuitBreaker](./reference/connectors/circuitbreaker.md) - Prevent cascading failures
- [RateLimiter](./reference/connectors/ratelimiter.md) - Control throughput
- [Timeout](./reference/connectors/timeout.md) - Bound execution time

## Cookbook

- **[Cookbook Index](./cookbook/README.md)** - Recipe collection for common patterns
- **[Resilient API Calls](./cookbook/resilient-api-calls.md)** - Production-ready external service integration
- **[Data Validation](./cookbook/data-validation-pipeline.md)** - Comprehensive validation patterns
- **[Event Processing](./cookbook/event-processing.md)** - Scalable event handling
- **[ETL Pipelines](./cookbook/etl-pipelines.md)** - Extract-Transform-Load patterns
- **[Common Patterns](./cookbook/patterns.md)** - Additional implementation patterns

## Examples

Complete, runnable examples showing real-world usage:

- **[Order Processing](../examples/order-processing/)** - E-commerce order system evolution from MVP to enterprise
- **[User Profile Update](../examples/user-profile-update/)** - Multi-step operations with external services
- **[Customer Support](../examples/customer-support/)** - Intelligent ticket routing and prioritization
- **[Event Orchestration](../examples/event-orchestration/)** - Complex event routing and processing
- **[Shipping Fulfillment](../examples/shipping-fulfillment/)** - Multi-provider shipping integration

Run any example:
```bash
cd examples/order-processing
go run .
```

## Additional Resources

- **[Troubleshooting](./troubleshooting.md)** - Debug common issues and gotchas
- **[Performance Benchmarks](./performance.md)** - Measured overhead and optimization
- **[Examples Guide](./examples.md)** - Overview of runnable examples

## Quick Links

- [GitHub Repository](https://github.com/zoobzio/pipz)
- [Go Package Documentation](https://pkg.go.dev/github.com/zoobzio/pipz)
- [Contributing Guidelines](../CONTRIBUTING.md)