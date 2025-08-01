# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain.

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
    var pipeErr *pipz.Error[Payment]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Failed at %s after %v: %v\n", 
            strings.Join(pipeErr.Path, "->"), // "payment-flow->payment-gateway->charge_primary"
            pipeErr.Duration,                 // 230ms
            pipeErr.Err)                     // stripe: insufficient funds
    }
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

**Processors** transform data:
- `Transform` - Pure transformations that can't fail
- `Apply` - Transformations that might return errors  
- `Effect` - Side effects that don't modify data
- `Mutate` - Conditional modifications
- `Enrich` - Best-effort data enhancement

**Connectors** compose processors:
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