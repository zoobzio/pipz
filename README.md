# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain.

```go
// Build a payment processing pipeline in one line
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
    fmt.Printf("Failed at %s after %v: %v\n", 
        err.Path,      // ["payment-flow", "payment-gateway", "charge_primary"]
        err.Duration,  // 230ms
        err.Err)       // stripe: insufficient funds
}
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
import "github.com/zoobzio/pipz"

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

// Or build dynamically
pipeline := pipz.NewSequence[Order]("order-processing")
if config.ValidateOrders {
    pipeline.Register(validate)
}
pipeline.Register(enrichOrder)

// Process data
result, err := pipeline.Process(ctx, order)
```

## Core Concepts

**Processors** transform data:
- `Transform` - Pure transformations that can't fail
- `Apply` - Transformations that might return errors  
- `Effect` - Side effects that don't modify data
- `Mutate` - Conditional modifications
- `Enrich` - Best-effort data enhancement

**Connectors** compose processors:
- `Sequence` - Run processors in order with dynamic modification
- `Concurrent` - Run in parallel (requires `Cloner` interface)
- `Switch` - Route based on conditions
- `Fallback` - Try primary, fall back on error
- `Race` - First success wins
- `Retry` / `Backoff` - Retry on failure with optional delays
- `Timeout` - Enforce time limits

**Error Handling**:
- Rich error context with complete path tracking
- Shows exactly where failures occurred in the pipeline
- Includes timing information and input data
- Distinguishes between timeouts, cancellations, and failures

**Design Philosophy**:
- Processors are immutable values (simple, predictable)
- Connectors are mutable pointers (configurable, stateful)
- Errors carry full context for debugging

## Documentation

📚 **[Full Documentation](./docs/README.md)**

- [Introduction](./docs/introduction.md) - Why pipz and core philosophy
- [Quick Start Guide](./docs/quick-start.md) - Build your first pipeline
- [Concepts](./docs/concepts/processors.md) - Deep dive into processors and connectors
- [Examples Walkthrough](./docs/examples/payment-processing.md) - Learn from real implementations
- [API Reference](./docs/api/processors.md) - Complete API documentation

## Examples

The [`examples/`](./examples/) directory contains complete, runnable examples:

- **[Payment Processing](./examples/payment/)** - Multi-provider payments with smart failover
- **[ETL Pipeline](./examples/etl/)** - Extract, transform, load with error recovery
- **[Event Processing](./examples/events/)** - Event routing and deduplication
- **[AI/LLM Integration](./examples/ai/)** - AI service integration with caching
- **[Middleware](./examples/middleware/)** - HTTP middleware patterns
- **[Content Moderation](./examples/moderation/)** - Multi-stage content filtering
- **[Webhook Router](./examples/webhook/)** - Provider-agnostic webhook handling
- **[Validation](./examples/validation/)** - Complex business rule validation
- **[Security](./examples/security/)** - Authentication and authorization pipelines

Each example includes comprehensive tests showing usage patterns.

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