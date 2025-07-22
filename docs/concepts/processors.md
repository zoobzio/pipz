# Processors

Processors are the fundamental building blocks of pipz. Every data transformation, validation, or side effect is implemented as a processor.

## The Processor Interface

At its core, a processor is anything that implements this simple interface:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
}
```

This means a processor:
- Takes a context and data of type T
- Returns transformed data of the same type T
- Can return an error to stop the pipeline

## Processor Adapters

pipz provides several adapters to create processors from functions:

### Transform

Pure transformations that cannot fail:

```go
// Transform modifies data without the possibility of error
double := pipz.Transform("double", func(ctx context.Context, n int) int {
    return n * 2
})

// Usage
result, _ := double.Process(context.Background(), 21) // Always returns 42, nil
```

Use Transform when:
- The operation always succeeds
- You're doing simple data manipulation
- No external resources are involved

### Apply

Transformations that might fail:

```go
// Apply can return errors
divide := pipz.Apply("divide", func(ctx context.Context, n float64) (float64, error) {
    if n == 0 {
        return 0, fmt.Errorf("cannot divide by zero")
    }
    return 100 / n, nil
})

// Usage
result, err := divide.Process(context.Background(), 0) // Returns 0, error
```

Use Apply when:
- The operation might fail
- You're calling external services
- Validation might reject the input

### Effect

Side effects that don't modify data:

```go
// Effect performs side effects but returns input unchanged
logger := pipz.Effect("log", func(ctx context.Context, user User) error {
    log.Printf("Processing user: %s", user.Email)
    metrics.Increment("users.processed")
    return nil
})

// Usage
result, err := logger.Process(ctx, user) // result == user (unchanged)
```

Use Effect when:
- Logging or metrics
- Sending notifications
- Any operation that shouldn't modify the data

### Mutate

Conditional modifications:

```go
// Mutate changes data only when condition is true
autoVerify := pipz.Mutate("auto_verify",
    func(ctx context.Context, user User) bool {
        return strings.HasSuffix(user.Email, "@trusted.com")
    },
    func(ctx context.Context, user User) User {
        user.Verified = true
        user.VerifiedAt = time.Now()
        return user
    },
)
```

Use Mutate when:
- Changes depend on conditions
- You want clear separation of decision and action
- The transformation is optional

### Enrich

Best-effort data enhancement:

```go
// Enrich adds data but doesn't fail the pipeline on error
enrichLocation := pipz.Enrich("add_location", 
    func(ctx context.Context, user User) (User, error) {
        // Try to geocode address
        location, err := geocodeAPI.Lookup(ctx, user.Address)
        if err != nil {
            return user, err // Enrich will ignore this error
        }
        user.Location = location
        return user, nil
    },
)

// Pipeline continues even if enrichment fails
```

Use Enrich when:
- Additional data is nice-to-have
- External services might be unavailable
- You don't want to stop processing on failure

## Creating Custom Processors

You can implement the Chainable interface directly for complex processors:

```go
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

// Usage
limiter := &RateLimiter[Order]{
    name:    "order_limiter",
    limiter: rate.NewLimiter(rate.Every(time.Second), 100),
}
```

## ProcessorFunc Adapter

For quick anonymous processors:

```go
// ProcessorFunc converts a function to a Chainable
validator := pipz.ProcessorFunc[User](func(ctx context.Context, u User) (User, error) {
    if u.Age < 18 {
        return u, fmt.Errorf("user must be 18 or older")
    }
    return u, nil
})
```

## Concurrent Processing Considerations

When using connectors like `Concurrent` or `Race`, each processor receives a copy of the input to avoid data races:

```go
// For concurrent processing, types must implement Cloner
type Order struct {
    ID    string
    Items []Item
}

func (o Order) Clone() Order {
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    return Order{
        ID:    o.ID,
        Items: items,
    }
}

// Now safe for concurrent processing
pipeline := pipz.Concurrent(
    pipz.Effect("send_email", notifyCustomer),
    pipz.Effect("update_inventory", updateStock),
    pipz.Effect("log_analytics", trackOrder),
)
```

The `Cloner` interface ensures that concurrent processors can't interfere with each other by requiring explicit copying semantics.

## Processor Naming

Processor names are used for debugging and monitoring:

```go
// Names appear in error messages
pipz.Apply("validate_credit_card", validateCard)
// Error: "failed at 'validate_credit_card': invalid card number"

// Names can help identify bottlenecks
pipz.Apply("external_api_call", callSlowAPI)
```

## Context Usage

All processors receive a context for cancellation and timeouts:

```go
func slowProcessor(ctx context.Context, data Data) (Data, error) {
    select {
    case <-ctx.Done():
        return data, ctx.Err()
    case result := <-someSlowOperation(data):
        return result, nil
    }
}
```

## Next Steps

- [Connectors](./connectors.md) - Learn how to compose processors
- [Error Handling](./error-handling.md) - Build resilient pipelines
- [Pipelines](./pipelines.md) - Manage sequences of processors