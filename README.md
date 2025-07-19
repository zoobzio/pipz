# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain. Perfect for ETL, API middleware, event processing, and any multi-stage data transformation.

```go
// Build a payment processing pipeline
pipeline := pipz.Sequential(
    pipz.Apply("validate", validateOrder),
    pipz.Apply("check_inventory", checkInventory),
    pipz.Apply("check_fraud", checkFraud),
    pipz.Fallback(
        pipz.Apply("charge_primary", chargeCreditCard),
        pipz.Apply("charge_backup", chargePayPal),
    ),
    pipz.Effect("send_receipt", emailReceipt),
)

// Process with full context support
order, err := pipeline.Process(ctx, orderRequest)
```

## Why pipz?

### The Problem: Scattered Business Logic

Without pipz, complex data flows become tangled:

```go
func processOrder(ctx context.Context, order Order) (Order, error) {
    // Validation mixed with error handling
    if err := validateOrder(order); err != nil {
        log.Printf("Validation failed: %v", err)
        metrics.Inc("order.validation.failed")
        return order, fmt.Errorf("validation: %w", err)
    }
    
    // Inventory check with manual context passing
    if err := checkInventory(ctx, order); err != nil {
        log.Printf("Inventory check failed: %v", err)
        metrics.Inc("order.inventory.failed")
        return order, fmt.Errorf("inventory: %w", err)
    }
    
    // Fraud detection with nested logic
    score, err := checkFraud(ctx, order)
    if err != nil {
        log.Printf("Fraud check error: %v", err)
        // Don't fail on fraud check errors
    } else if score > 0.8 {
        alertSecurityTeam(order)
        return order, fmt.Errorf("high fraud risk: %.2f", score)
    }
    
    // Payment with fallback logic
    err = chargeCreditCard(ctx, order)
    if err != nil {
        log.Printf("Primary payment failed: %v", err)
        // Try backup
        err = chargePayPal(ctx, order)
        if err != nil {
            log.Printf("Backup payment failed: %v", err)
            metrics.Inc("order.payment.failed")
            return order, fmt.Errorf("all payment methods failed")
        }
    }
    
    // Success notification
    if err := emailReceipt(ctx, order); err != nil {
        log.Printf("Failed to send receipt: %v", err)
        // Don't fail the order for this
    }
    
    return order, nil
}
```

### The Solution: Composable Pipelines

With pipz, the same logic becomes clear and maintainable:

```go
// Define reusable processors
validateOrder := pipz.Validate("validate", func(ctx context.Context, o Order) error {
    if o.Total <= 0 {
        return errors.New("order total must be positive")
    }
    return nil
})

checkInventory := pipz.Validate("inventory", func(ctx context.Context, o Order) error {
    return inventory.Reserve(ctx, o.Items)
})

// Compose into a pipeline
orderPipeline := pipz.Sequential(
    validateOrder,
    checkInventory,
    pipz.Apply("fraud_check", fraudDetection),
    pipz.Fallback(
        pipz.Apply("charge_card", chargeCreditCard),
        pipz.Apply("charge_paypal", chargePayPal),
    ),
    pipz.Effect("notify", emailReceipt),
)

// Clean usage
order, err := orderPipeline.Process(ctx, orderRequest)
```

**Benefits:**
- ✅ **Readable** - The pipeline structure mirrors your business flow
- ✅ **Testable** - Each processor can be tested in isolation
- ✅ **Reusable** - Processors can be shared across pipelines
- ✅ **Maintainable** - Easy to add, remove, or reorder steps

## Core Concepts

### The Chainable Interface

Everything in pipz implements a simple interface:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
}
```

This uniform interface enables powerful composition patterns while maintaining type safety.

### Processors

Processors are the building blocks. pipz provides adapters to convert your functions into processors:

```go
// Apply - Transform data with possible errors
pipz.Apply("name", func(ctx context.Context, data T) (T, error))

// Validate - Check data without modifying
pipz.Validate("name", func(ctx context.Context, data T) error)

// Effect - Side effects that don't modify data
pipz.Effect("name", func(ctx context.Context, data T) error)
```

### Connectors

Connectors compose processors into complex flows:

#### Sequential
Process steps in order, stopping on first error:
```go
pipeline := pipz.Sequential(
    validateInput,
    enrichData,
    saveToDatabase,
)
```

#### Switch
Route to different processors based on data:
```go
router := pipz.Switch(
    func(ctx context.Context, req Request) string {
        return req.Type
    },
    map[string]pipz.Chainable[Request]{
        "api":    handleAPI,
        "webhook": handleWebhook,
        "batch":   handleBatch,
    },
)
```

#### Fallback
Try alternatives if the primary fails:
```go
resilient := pipz.Fallback(
    primaryDatabase,
    secondaryDatabase,
    cacheDatabase,
)
```

#### Retry
Retry with configurable attempts:
```go
reliable := pipz.Retry(
    unreliableService,
    3, // max attempts
)

// Or with exponential backoff
withBackoff := pipz.RetryWithBackoff(
    apiCall,
    5,                // max attempts
    100*time.Millisecond, // base delay
)
```

#### Timeout
Enforce time limits:
```go
fast := pipz.Timeout(
    slowOperation,
    5*time.Second,
)
```

## Real-World Examples

### Webhook Processing
Multi-provider webhook handling with signature verification:

```go
webhookPipeline := pipz.Sequential(
    pipz.Apply("identify", identifyProvider),
    pipz.Apply("verify", verifySignature),
    pipz.Validate("prevent_replay", checkReplay),
    pipz.Switch(routeByProvider, map[string]pipz.Chainable[Webhook]{
        "stripe": handleStripeWebhook,
        "github": handleGitHubWebhook,
        "slack":  handleSlackWebhook,
    }),
    pipz.Effect("audit", logWebhook),
)
```

### ETL Pipeline
Data processing with validation and transformation:

```go
etlPipeline := pipz.Sequential(
    pipz.Apply("parse", parseCSV),
    pipz.Validate("schema", validateSchema),
    pipz.Apply("transform", transformFields),
    pipz.Apply("enrich", enrichFromAPI),
    pipz.Effect("progress", updateProgress),
    pipz.Apply("load", loadToDatabase),
)
```

### Content Moderation
Multi-stage content analysis with parallel processing:

```go
moderationPipeline := pipz.Sequential(
    pipz.Apply("analyze", parallelAnalysis), // Text + image analysis
    pipz.Apply("score", calculateRiskScore),
    pipz.Switch(routeByScore, map[string]pipz.Chainable[Content]{
        "safe":       pipz.Apply("approve", autoApprove),
        "suspicious": pipz.Apply("queue", queueForReview),
        "harmful":    pipz.Apply("block", blockContent),
    }),
    pipz.Effect("metrics", recordMetrics),
)
```

## Installation

```bash
go get github.com/zoobzio/pipz
```

## Quick Start

### 1. Define Your Data Types

```go
type Order struct {
    ID       string
    Customer string
    Items    []Item
    Total    float64
}
```

### 2. Create Processors

```go
// Validation
validateOrder := pipz.Validate("validate", func(ctx context.Context, o Order) error {
    if len(o.Items) == 0 {
        return errors.New("order must have items")
    }
    if o.Total <= 0 {
        return errors.New("order total must be positive")
    }
    return nil
})

// Transformation
applyTax := pipz.Apply("tax", func(ctx context.Context, o Order) (Order, error) {
    o.Total = o.Total * 1.08 // 8% tax
    return o, nil
})

// Side effects
logOrder := pipz.Effect("log", func(ctx context.Context, o Order) error {
    log.Printf("Processing order %s for %s", o.ID, o.Customer)
    return nil
})
```

### 3. Build Your Pipeline

```go
orderPipeline := pipz.Sequential(
    validateOrder,
    applyTax,
    logOrder,
)

// Process orders
order, err := orderPipeline.Process(ctx, newOrder)
if err != nil {
    // Handle error - pipeline stops at first error
}
```

## Testing

pipz makes testing straightforward:

```go
func TestOrderValidation(t *testing.T) {
    validator := pipz.Validate("test", validateOrder)
    
    // Test invalid order
    _, err := validator.Process(context.Background(), Order{})
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "must have items")
    
    // Test valid order
    validOrder := Order{
        Items: []Item{{Name: "Widget", Price: 10}},
        Total: 10,
    }
    _, err = validator.Process(context.Background(), validOrder)
    assert.NoError(t, err)
}
```

## Performance

pipz is designed for production use:

- **Zero allocations** in the happy path for most processors
- **Minimal overhead** - typically 10-50ns per processor
- **No reflection** at runtime
- **No global state** or locks

Example benchmarks from real pipelines:

```
BenchmarkValidation_Pipeline-12    2,915,101    432.1 ns/op    0 B/op    0 allocs/op
BenchmarkETL_Pipeline-12             763,939    1,564 ns/op    112 B/op    2 allocs/op
BenchmarkWebhook_Routing-12        1,851,421    649.2 ns/op    0 B/op    0 allocs/op
```

## Examples

The [examples](examples/) directory contains production-ready patterns:

- **[validation](examples/validation/)** - Business rule validation
- **[webhook](examples/webhook/)** - Multi-provider webhook processing
- **[etl](examples/etl/)** - CSV/JSON data transformation
- **[workflow](examples/workflow/)** - Complex e-commerce order flow
- **[moderation](examples/moderation/)** - Content moderation pipeline
- **[payment](examples/payment/)** - Payment processing with fallbacks
- And more...

Each example includes benchmarks and comprehensive tests.

## Best Practices

### 1. Name Your Processors
Always provide names for better error messages:
```go
// Good - errors will show "validate_age"
pipz.Apply("validate_age", checkAge)

// Less helpful - errors show function pointer
pipz.Apply("", checkAge)
```

### 2. Keep Processors Focused
Each processor should do one thing:
```go
// Good - single responsibility
validateEmail := pipz.Validate("email", checkEmail)
validateAge := pipz.Validate("age", checkAge)

// Bad - doing too much
validateEverything := pipz.Validate("all", checkEmailAndAgeAndName)
```

### 3. Use Context
Pass configuration and deadlines through context:
```go
func enrichWithAPI(ctx context.Context, data Data) (Data, error) {
    client := ctx.Value(clientKey).(*APIClient)
    timeout, _ := ctx.Deadline()
    // Use client and respect timeout...
}
```

### 4. Handle Errors Appropriately
- Use `Validate` for checks that should stop processing
- Use `Effect` for non-critical operations
- Use `Fallback` for operations with alternatives

## When to Use pipz

pipz excels at:
- ✅ **Multi-step data processing** - ETL, order processing, API requests
- ✅ **Middleware chains** - Authentication, validation, logging
- ✅ **Event processing** - Webhooks, message queues, event streams
- ✅ **Complex business workflows** - With branching and error handling
- ✅ **Resilient integrations** - With retries, timeouts, and fallbacks

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT - See [LICENSE](LICENSE) for details.