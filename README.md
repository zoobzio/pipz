# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

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
- **Zero dependencies**: Just standard library
- **Battle-tested patterns**: Retry, timeout, fallback, error recovery built-in
- **Testable**: Every component is independently testable
- **Fast**: Minimal allocations, optimized for performance
- **Rich error context**: Know exactly where failures occur with complete path tracking
- **Dynamic or static**: Build pipelines declaratively or modify them at runtime
- **Errors are pipelines too**: Build sophisticated error recovery using the same pipeline tools

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

- [Introduction](./docs/introduction.md) - Why pipz and core philosophy
- [Quick Start Guide](./docs/quick-start.md) - Build your first pipeline
- [Concepts](./docs/concepts/processors.md) - Deep dive into processors and connectors
- [Examples](./examples/) - Real-world implementations and patterns
- [API Reference](./docs/api/) - Complete API documentation

## Examples

The [`examples/`](./examples/) directory contains complete, runnable examples:

- **[Order Processing](./examples/order-processing/)** - E-commerce order processing from MVP to enterprise scale
- **[User Profile Update](./examples/user-profile-update/)** - Complex multi-step operations with external services
- **[Customer Support](./examples/customer-support/)** - Intelligent ticket routing and prioritization
- **[Event Orchestration](./examples/event-orchestration/)** - Event routing with compliance and safety measures
- **[Shipping Fulfillment](./examples/shipping-fulfillment/)** - Multi-provider shipping with smart carrier selection

Each example includes comprehensive tests, detailed documentation, and demonstrates real-world patterns.

## Built with pipz

A growing ecosystem of tools that leverage pipz's flexibility:

### **[flume](https://github.com/zoobzio/flume)** - Dynamic Pipeline Factory
Build pipelines from YAML/JSON configuration files with hot-reloading.

**What it does:** Stores processors, predicates, and conditions in a registry. Builds pipelines dynamically from schemas that describe the pipeline structure.

**How pipz enables it:** Flume stores `pipz.Chainable[T]` implementations and uses pipz's connectors (`NewSequence`, `NewSwitch`, etc.) to construct pipelines based on schemas. Any `Chainable[T]` can be registered and composed.

### **[zlog](https://github.com/zoobzio/zlog)** - Signal-Based Structured Logging
Route events by meaning, not severity levels - send different signals to different destinations.

**What it does:** Routes events to different sinks based on signals. Each sink can process events independently with its own error handling and capabilities.

**How pipz enables it:** Sinks wrap `pipz.Chainable[Event[Fields]]`, allowing them to be enhanced with pipz patterns (retry, timeout, circuit breaker). The routing system uses `pipz.Switch` for signal-based dispatch and `pipz.Scaffold` for parallel sink processing.

### **[sctx](https://github.com/zoobzio/sctx)** - Zero-Trust Security Framework
Transform mTLS certificates into cryptographic security tokens with permissions.

**What it does:** Processes certificates through a configurable pipeline to generate security contexts with permissions, expiry times, and metadata.

**How pipz enables it:** Certificate processing uses pipz processors (`GrantPermissions`, `SetExpiry`) composed into pipelines. Uses flume for schema-driven pipeline configuration, allowing dynamic security policies.

### **[sentinel](https://github.com/zoobzio/sentinel)** - Struct Metadata Extraction
Extract and cache metadata from struct tags with pluggable processing.

**What it does:** Reflects on structs to extract metadata from tags (json, validate, custom tags) and caches the results. Provides a plugin point for custom extraction logic.

**How pipz enables it:** Uses `pipz.Sequence[*ExtractionContext]` as the extraction pipeline, allowing users to add their own processors to enrich or validate metadata during extraction.

---

*These packages demonstrate pipz's versatility - from configuration management to logging infrastructure to security systems to reflection tooling.*

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