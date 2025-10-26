# pipz

[![CI Status](https://github.com/zoobzio/pipz/workflows/CI/badge.svg)](https://github.com/zoobzio/pipz/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/pipz/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/pipz)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/pipz)](https://goreportcard.com/report/github.com/zoobzio/pipz)
[![CodeQL](https://github.com/zoobzio/pipz/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/pipz/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/pipz.svg)](https://pkg.go.dev/github.com/zoobzio/pipz)
[![License](https://img.shields.io/github/license/zoobzio/pipz)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/pipz)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/pipz)](https://github.com/zoobzio/pipz/releases)

Type-safe, composable data pipelines for Go with minimal dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain.

## The Power of Interfaces

At its core, pipz is built on a single, simple interface:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Name() string
}
```

**Any type that implements this interface can be used in a pipeline.** This means you can:
- Use the built-in processor wrappers for common patterns
- Implement your own custom processors
- Mix and match both approaches seamlessly

```go
// Custom processor implementing Chainable[T]
type RateLimiter[T any] struct {
    name    string
    limiter *rate.Limiter
}

func (r *RateLimiter[T]) Process(ctx context.Context, data T) (T, error) {
    if err := r.limiter.Wait(ctx); err != nil {
        return data, fmt.Errorf("rate limit: %w", err)
    }
    return data, nil
}

func (r *RateLimiter[T]) Name() string { return r.name }

// Use it directly in a pipeline alongside built-in processors
limiter := rate.NewLimiter(rate.Every(time.Second/100), 10) // 100 req/s, burst 10
pipeline := pipz.NewSequence("api-flow",
    pipz.Apply("validate", validateFunc),          // Built-in wrapper
    &RateLimiter[Order]{                          // Custom implementation
        name:    "rate-limit",
        limiter: limiter,
    },
    pipz.Transform("format", formatFunc),          // Built-in wrapper
)
```

## Quick Start

While you can implement `Chainable[T]` directly, pipz provides convenient processor wrappers for common patterns:

```go
// Transform: Pure transformations that can't fail
uppercase := pipz.Transform("uppercase", func(_ context.Context, s string) string {
    return strings.ToUpper(s)
})

// Apply: Transformations that might return errors
validate := pipz.Apply("validate", func(_ context.Context, order Order) (Order, error) {
    if order.Total <= 0 {
        return order, errors.New("invalid order total")
    }
    return order, nil
})

// Compose into a pipeline
pipeline := pipz.NewSequence("order-processing", validate, uppercase)

// Process data
result, err := pipeline.Process(ctx, order)
```

## Why pipz?

- **Type-safe**: Full compile-time type checking with Go generics
- **Composable**: Build complex pipelines from simple, reusable parts
- **Minimal dependencies**: Just standard library plus [clockz](https://github.com/zoobzio/clockz) and optional [capitan](https://github.com/zoobzio/capitan)
- **Battle-tested patterns**: Retry, timeout, fallback, error recovery built-in
- **Observable**: Emit typed signals for state changes (CircuitBreaker, RateLimiter, WorkerPool) via hooks
- **Testable**: Every component is independently testable
- **Fast**: Minimal allocations, optimized for performance
- **Rich error context**: Know exactly where failures occur with complete path tracking
- **Dynamic or static**: Build pipelines declaratively or modify them at runtime
- **Errors are pipelines too**: Build sophisticated error recovery using the same pipeline tools
- **Panic-safe**: Automatic panic recovery with security sanitization prevents crashes

## Installation

```bash
go get github.com/zoobzio/pipz
```

Requirements: Go 1.21+ (for generics)

## Quick Example

```go
package main

import (
    "context"
    "errors"
    "time"
    "github.com/zoobzio/pipz"
)

type Order struct {
    ID          string
    Total       float64
    ProcessedAt time.Time
}

func main() {
    ctx := context.Background()
    
    // Define processors
    validate := pipz.Apply("validate", func(ctx context.Context, order Order) (Order, error) {
        if order.Total <= 0 {
            return order, errors.New("invalid order total")
        }
        return order, nil
    })

    enrichOrder := pipz.Transform("enrich", func(ctx context.Context, order Order) Order {
        order.ProcessedAt = time.Now()
        return order
    })

    // Compose into pipeline (single line)
    pipeline := pipz.NewSequence("order-processing", validate, enrichOrder)

    // Process data
    order := Order{ID: "ORDER-123", Total: 99.99}
    result, err := pipeline.Process(ctx, order)
    if err != nil {
        // Handle error with full context
        panic(err)
    }
    // result.ProcessedAt is now set
    _ = result
}
```

## Core Concepts

**The Chainable Interface**: Everything in pipz implements `Chainable[T]`. You can:
- Implement it directly for custom processors
- Use the provided processor wrappers for common patterns
- Mix both approaches in the same pipeline

**Processor Wrappers** (optional conveniences for common patterns):
- `Transform` - Pure transformations that can't fail
- `Apply` - Transformations that might return errors  
- `Effect` - Side effects that don't modify data
- `Mutate` - Conditional modifications
- `Enrich` - Best-effort data enhancement

**Connectors** compose any Chainable[T] implementations:
- `Sequence` - Run processors in order with dynamic modification
- `Concurrent` - Run in parallel, wait for completion (requires `Cloner` interface)
- `WorkerPool` - Bounded parallelism with fixed worker count (requires `Cloner` interface)
- `Scaffold` - Fire-and-forget parallel execution (requires `Cloner` interface)
- `Switch` - Route based on conditions
- `Fallback` - Try primary, fall back on error
- `Race` - First success wins
- `Contest` - First result meeting condition wins
- `Retry` / `Backoff` - Retry on failure with optional delays
- `Timeout` - Enforce time limits
- `Handle` - Process errors through their own pipeline
- `Filter` - Conditionally execute processor
- `RateLimiter` - Token bucket rate limiting for resource protection
- `CircuitBreaker` - Prevent cascading failures with circuit breaker pattern

**Error Handling**:
- Rich error context with complete path tracking
- Shows exactly where failures occurred in the pipeline
- Includes timing information and input data
- Distinguishes between timeouts, cancellations, and failures
- Automatic panic recovery with security-focused sanitization
- Panics converted to errors with sensitive information stripped

**Design Philosophy**:
- Processors are immutable values (simple, predictable)
- Connectors are mutable pointers (configurable, stateful)
- Errors carry full context for debugging

## Real-World Example

Here's a more complex example showing how pipz handles real-world scenarios like payment processing:

```go
// Define your processing functions
validatePayment := func(_ context.Context, p Payment) (Payment, error) {
    if p.Amount <= 0 { return p, errors.New("invalid amount") }
    return p, nil
}

checkFraud := func(_ context.Context, p Payment) (Payment, error) {
    // Fraud detection logic here
    return p, nil
}

chargeStripe := func(_ context.Context, p Payment) (Payment, error) {
    // Stripe API call here
    p.TransactionID = "stripe_" + generateID()
    return p, nil
}

chargePayPal := func(_ context.Context, p Payment) (Payment, error) {
    // PayPal API call here  
    p.TransactionID = "paypal_" + generateID()
    return p, nil
}

emailReceipt := func(_ context.Context, p Payment) error {
    // Send email receipt
    return sendEmail(p.Email, "Receipt", p.TransactionID)
}

// Build a payment processing pipeline
pipeline := pipz.NewSequence("payment-flow",
    pipz.Apply("validate", validatePayment),
    pipz.Apply("check_fraud", checkFraud),
    pipz.NewFallback("payment-gateway",
        pipz.Apply("charge_primary", chargeStripe),
        pipz.Apply("charge_backup", chargePayPal),
    ),
    pipz.Effect("send_receipt", emailReceipt),
)

// Process with full context support
result, err := pipeline.Process(ctx, payment)
if err != nil {
    // Rich error context shows exactly where it failed
    var pipeErr *pipz.Error[Payment]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Failed at %s after %v: %v\n", 
            strings.Join(pipeErr.Path, "->"), // "payment-flow->payment-gateway->charge_primary"
            pipeErr.Duration,                 // 230ms
            pipeErr.Err)                     // stripe: insufficient funds
    }
}
```

## Documentation

📚 **[Full Documentation](./docs/README.md)**

- [Introduction](./docs/learn/introduction.md) - Why pipz and core philosophy
- [Quick Start Guide](./docs/tutorials/quickstart.md) - Build your first pipeline
- [Concepts](./docs/learn/core-concepts.md) - Deep dive into processors and connectors
- [Examples](./examples/) - Real-world implementations and patterns
- [API Reference](./docs/reference/) - Complete API documentation

## Examples

The [`examples/`](./examples/) directory contains complete, runnable examples:

- **[Order Processing](./examples/order-processing/)** - E-commerce order processing from MVP to enterprise scale
- **[User Profile Update](./examples/user-profile-update/)** - Complex multi-step operations with external services
- **[Customer Support](./examples/customer-support/)** - Intelligent ticket routing and prioritization
- **[Event Orchestration](./examples/event-orchestration/)** - Event routing with compliance and safety measures
- **[Shipping Fulfillment](./examples/shipping-fulfillment/)** - Multi-provider shipping with smart carrier selection

Each example includes comprehensive tests, detailed documentation, and demonstrates real-world patterns.


## Performance

pipz is designed for performance:

- Minimal allocations where possible
- Efficient error propagation
- No reflection or runtime type assertions
- Optimized for common paths

Run benchmarks:
```bash
make bench
```

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Run tests
make test

# Run linter
make lint

# Run benchmarks
make bench
```

## License

MIT License - see [LICENSE](LICENSE) file for details.