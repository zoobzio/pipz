# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain. Perfect for ETL, API middleware, event processing, and any multi-stage data transformation.

```go
// Build a payment processing pipeline
pipeline := pipz.Sequential(
    pipz.Apply("validate", validatePayment),
    pipz.Apply("check_fraud", checkFraud),
    pipz.Fallback(
        pipz.Apply("charge_primary", chargeStripe),
        pipz.Apply("charge_backup", chargePayPal),
    ),
    pipz.Effect("send_receipt", emailReceipt),
)

// Process with full context support
result, err := pipeline.Process(ctx, payment)
```

## Why pipz?

- **Type-safe**: Full compile-time type checking with Go generics
- **Composable**: Build complex pipelines from simple, reusable parts  
- **Zero dependencies**: Just standard library
- **Battle-tested patterns**: Retry, timeout, fallback, circuit breaker built-in
- **Testable**: Every component is independently testable
- **Fast**: Zero-allocation core operations

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

// Compose into pipeline
pipeline := pipz.Sequential(validate, enrichOrder)

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
- `Sequential` - Run in order
- `Concurrent` - Run in parallel (requires `Cloner` interface)
- `Switch` - Route based on conditions
- `Fallback` - Try primary, fall back on error
- `Race` - First success wins
- `Retry` / `RetryWithBackoff` - Retry on failure
- `Timeout` - Enforce time limits

**Pipelines** provide introspection and management of processor sequences.

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

- Zero allocations in core operations
- Minimal interface overhead
- Efficient error handling
- No reflection or runtime type assertions

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