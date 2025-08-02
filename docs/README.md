# pipz Documentation

Welcome to the pipz documentation! This guide covers everything you need to know about building composable data pipelines with pipz.

## Documentation Structure

### Getting Started
- [Introduction](./introduction.md) - What is pipz and why use it
- [Quick Start](./quick-start.md) - Get up and running in 5 minutes
- [Installation](./installation.md) - Installation and setup instructions

### Core Concepts
- [Processors](./concepts/processors.md) - The building blocks of pipelines
- [Connectors](./concepts/connectors.md) - Composing processors together
- [Pipelines](./concepts/pipelines.md) - Managing and running pipelines
- [Error Handling](./concepts/error-handling.md) - Handling failures gracefully
- [Type Safety](./concepts/type-safety.md) - Leveraging Go generics

### Guides
- [Building Your First Pipeline](./guides/first-pipeline.md)
- [Common Patterns](./guides/patterns.md) - Rate limiters, circuit breakers, and more
- [Error Recovery Patterns](./guides/error-recovery.md)
- [Performance Optimization](./guides/performance.md)
- [Testing Pipelines](./guides/testing.md)
- [Best Practices](./guides/best-practices.md)


### API Reference

#### Processors
- [Transform](./api/transform.md) - Pure transformations that cannot fail
- [Apply](./api/apply.md) - Operations that can fail
- [Effect](./api/effect.md) - Side effects without modifying data
- [Mutate](./api/mutate.md) - Conditional modifications
- [Enrich](./api/enrich.md) - Optional enhancements

#### Connectors
- [Sequence](./api/sequence.md) - Sequential execution with runtime modification
- [Concurrent](./api/concurrent.md) - Parallel execution of processors
- [Scaffold](./api/scaffold.md) - Fire-and-forget parallel execution
- [Race](./api/race.md) - First successful result wins
- [Contest](./api/contest.md) - First result meeting condition wins
- [Fallback](./api/fallback.md) - Try alternatives on error
- [Retry](./api/retry.md) - Retry with attempts and backoff
- [Handle](./api/handle.md) - Error observation and recovery
- [Switch](./api/switch.md) - Conditional routing
- [Timeout](./api/timeout.md) - Enforce time constraints
- [Filter](./api/filter.md) - Conditional processor execution
- [RateLimiter](./api/ratelimiter.md) - Token bucket rate limiting
- [CircuitBreaker](./api/circuitbreaker.md) - Circuit breaker pattern

## Quick Links

- [GitHub Repository](https://github.com/zoobzio/pipz)
- [Examples Directory](https://github.com/zoobzio/pipz/tree/main/examples)
- [Contributing Guide](./CONTRIBUTING.md)
- [Performance Benchmarks](./PERFORMANCE.md)