# Processors

Processors are the fundamental building blocks of pipz. Every data transformation, validation, or side effect is implemented as a processor.

## The Power of the Chainable Interface

At its core, pipz is built on a single interface:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Name() string
}
```

**This is the key to pipz's flexibility**: Any type that implements this interface can be used in a pipeline. This means you have complete freedom in how you structure your processors:

### Direct Implementation

You can implement `Chainable[T]` directly for custom processors:

```go
// Custom rate limiter
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

// Custom circuit breaker
type CircuitBreaker[T any] struct {
    name    string
    breaker *gobreaker.CircuitBreaker
}

func (cb *CircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
    result, err := cb.breaker.Execute(func() (interface{}, error) {
        // Your processing logic here
        return data, nil
    })
    if err != nil {
        return data, err
    }
    return result.(T), nil
}

func (cb *CircuitBreaker[T]) Name() string { return cb.name }
```

### Using in Pipelines

Custom implementations work seamlessly with built-in processors:

```go
pipeline := pipz.NewSequence("api-pipeline",
    pipz.Apply("validate", validateFunc),           // Built-in wrapper
    &RateLimiter[Request]{...},                    // Custom implementation
    &CircuitBreaker[Request]{...},                 // Custom implementation
    pipz.Transform("format", formatFunc),          // Built-in wrapper
)
```

## Processor Wrappers (Optional Conveniences)

While you can always implement `Chainable[T]` directly, pipz provides wrapper functions for common patterns. These wrappers handle error wrapping and provide a consistent API:

### Transform

Pure transformations that cannot fail:

```go
// Best practice: use constants for processor names
const (
    ProcessorDouble = "double"
    ProcessorValidate = "validate"
    ProcessorEnrich = "enrich"
)

// Transform modifies data without the possibility of error
double := pipz.Transform(ProcessorDouble, func(ctx context.Context, n int) int {
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

## When to Use Each Approach

### Use Direct Implementation When:
- You need stateful processors (rate limiters, circuit breakers, caches)
- You're wrapping existing libraries or services
- You need fine-grained control over error handling
- You want to encapsulate complex logic with its own methods

### Use Wrapper Functions When:
- You have simple, stateless transformations
- You want the convenience of automatic error wrapping
- You're prototyping or building quick pipelines
- Your logic fits naturally into a single function

Both approaches are first-class citizens in pipz - choose based on your needs, not convention.


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

Processor names are used for debugging and monitoring. Use constants for consistency:

```go
// Define constants for processor names
const (
    ProcessorValidateCard = "validate_credit_card"
    ProcessorCallAPI     = "external_api_call"
    ProcessorSaveDB      = "save_to_database"
)

// Names appear in error messages
processor := pipz.Apply(ProcessorValidateCard, validateCard)
// Error: "failed at [payment-flow -> validate_credit_card]: invalid card number"

// Names help identify bottlenecks and trace errors
pipeline := pipz.NewSequence("payment-flow",
    pipz.Apply(ProcessorValidateCard, validateCard),
    pipz.Apply(ProcessorCallAPI, callPaymentAPI),
    pipz.Apply(ProcessorSaveDB, saveTransaction),
)
```

Using constants ensures:
- Consistent naming across your codebase
- Easy refactoring and searching
- No typos in error messages
- Clear processor identification in logs

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
- [Pipelines](./pipelines.md) - Composing processors into workflows