# Connectors

Connectors are functions that combine processors to create more complex behaviors. They're the key to pipz's composability.

## Core Connectors

### Sequential

Runs processors in order, passing output to input:

```go
pipeline := pipz.Sequential(
    validateOrder,
    calculateTax,
    applyDiscount,
    chargePayment,
)
```

- Stops on first error
- Each processor receives the output of the previous one
- Most common connector for step-by-step workflows

### Switch

Routes to different processors based on conditions:

```go
// Define route types for type safety
type PaymentRoute string
const (
    RouteCard   PaymentRoute = "card"
    RouteCrypto PaymentRoute = "crypto"
    RouteWire   PaymentRoute = "wire"
)

pipeline := pipz.Switch(
    func(ctx context.Context, p Payment) PaymentRoute {
        switch p.Method {
        case "bitcoin", "ethereum":
            return RouteCrypto
        case "wire", "ach":
            return RouteWire
        default:
            return RouteCard
        }
    },
    map[PaymentRoute]pipz.Chainable[Payment]{
        RouteCard:   processCardPayment,
        RouteCrypto: processCryptoPayment,
        RouteWire:   processWireTransfer,
    },
)
```

- Uses comparable types for keys (not just strings)
- No default/catch-all - unmatched routes return an error
- Route map can be modified at runtime for dynamic routing

### Concurrent

Runs multiple processors in parallel with isolated data:

```go
// Type must implement Cloner interface
type Notification struct {
    OrderID string
    Email   string
}

func (n Notification) Clone() Notification {
    return n // Simple struct, no pointers
}

// Fire-and-forget parallel operations
notify := pipz.Concurrent(
    pipz.Effect("email", sendEmailNotification),
    pipz.Effect("sms", sendSMSNotification),
    pipz.Effect("push", sendPushNotification),
    pipz.Effect("analytics", trackNotification),
)
```

- Each processor gets a clone of the input
- All processors run regardless of individual failures
- Returns original input unchanged
- Requires input type to implement `Cloner[T]`

### Race

First successful result wins:

```go
// Try multiple providers, use fastest response
fetchData := pipz.Race(
    fetchFromCache,
    fetchFromPrimaryDB,
    fetchFromReplicaDB,
)
```

- Cancels other processors when one succeeds
- Returns error only if all fail
- Good for reducing latency with redundancy
- Also requires `Cloner[T]` interface

### Fallback

Try primary, fall back on error:

```go
processPayment := pipz.Fallback(
    stripeProcessor,
    paypalProcessor,
)
```

- Only tries fallback if primary fails
- Different from Race - sequential not parallel
- Good for primary/backup patterns

### Retry

Retry on failure with configurable attempts:

```go
// Simple retry
saveToDatabase := pipz.Retry(databaseWriter, 3)

// With exponential backoff
callAPI := pipz.RetryWithBackoff(
    apiClient,
    5,                    // max attempts
    100*time.Millisecond, // initial delay
)
```

- `Retry`: Immediate retries
- `RetryWithBackoff`: Exponential delays (100ms, 200ms, 400ms...)
- Checks context cancellation between attempts

### Timeout

Enforce time limits:

```go
fetchUser := pipz.Timeout(
    userServiceCall,
    2*time.Second,
)
```

- Cancels operation via context
- Returns timeout error if exceeded
- Wrap any slow operations

### WithErrorHandler

Add error handling without changing behavior:

```go
processWithLogging := pipz.WithErrorHandler(
    riskyOperation,
    pipz.Effect("log_error", func(ctx context.Context, err error) error {
        log.Printf("Operation failed: %v", err)
        metrics.Increment("errors")
        return nil
    }),
)
```

- Error handler runs on failure
- Original error is still returned
- Good for observability

## Composition Patterns

### Nested Composition

Connectors can be nested arbitrarily:

```go
robust := pipz.Sequential(
    validate,
    pipz.Timeout(
        pipz.RetryWithBackoff(
            pipz.Fallback(
                primaryProcessor,
                backupProcessor,
            ),
            3,
            time.Second,
        ),
        30*time.Second,
    ),
    pipz.Concurrent(
        notifySuccess,
        updateMetrics,
        logResult,
    ),
)
```

### Conditional Branches

Complex routing logic:

```go
type RouteKey int
const (
    RouteValidate RouteKey = iota
    RouteTransform
    RouteError
)

pipeline := pipz.Switch(
    func(ctx context.Context, data Data) RouteKey {
        if data.NeedsValidation {
            return RouteValidate
        }
        if data.HasErrors {
            return RouteError
        }
        return RouteTransform
    },
    map[RouteKey]pipz.Chainable[Data]{
        RouteValidate: pipz.Sequential(
            validate,
            pipz.Switch(...), // Nested switch
        ),
        RouteTransform: transform,
        RouteError: errorHandler,
    },
)
```

### Dynamic Routing

Switch routes can be modified at runtime:

```go
routes := make(map[string]pipz.Chainable[Request])
routes["v1"] = oldAPIHandler
routes["v2"] = newAPIHandler

router := pipz.Switch(
    func(ctx context.Context, req Request) string {
        return req.Version
    },
    routes,
)

// Later: Add new version without rebuilding pipeline
routes["v3"] = betaAPIHandler
```

## Creating Custom Connectors

You can create your own connectors:

```go
// ThrottlePerKey limits concurrent processing per key
func ThrottlePerKey[T any, K comparable](
    keyFunc func(T) K,
    limit int,
    processor pipz.Chainable[T],
) pipz.Chainable[T] {
    limiters := make(map[K]chan struct{})
    var mu sync.Mutex
    
    return pipz.ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
        key := keyFunc(data)
        
        mu.Lock()
        limiter, exists := limiters[key]
        if !exists {
            limiter = make(chan struct{}, limit)
            limiters[key] = limiter
        }
        mu.Unlock()
        
        select {
        case limiter <- struct{}{}:
            defer func() { <-limiter }()
            return processor.Process(ctx, data)
        case <-ctx.Done():
            return data, ctx.Err()
        }
    })
}
```

## Best Practices

1. **Start Simple**: Use Sequential for most cases
2. **Add Resilience Gradually**: Wrap with Retry/Timeout as needed
3. **Type-Safe Routes**: Use custom types for Switch keys
4. **Resource Awareness**: Concurrent processing multiplies resource usage
5. **Context Propagation**: All connectors properly propagate context

## Next Steps

- [Pipelines](./pipelines.md) - Managing processor sequences
- [Error Handling](./error-handling.md) - Building resilient systems
- [Type Safety](./type-safety.md) - Leveraging Go generics