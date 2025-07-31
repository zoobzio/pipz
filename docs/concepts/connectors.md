# Connectors

Connectors are mutable components that compose processors and other chainables to create complex processing behaviors. They're the key to pipz's composability and power.

## Understanding Connectors

Unlike processors (which are immutable values), connectors are:
- Created with `New*` functions that return pointers
- Mutable with configuration methods
- Thread-safe with internal synchronization
- Stateful, maintaining configuration and state

All connectors implement `Chainable[T]`, just like processors, allowing seamless composition.

## Core Connectors

### Sequence

The primary way to build sequential pipelines with runtime modification:

```go
// Create a mutable sequence
seq := pipz.NewSequence[Order]("order-processing")

// Register processors
seq.Register(
    validateOrder,
    calculateTax,
    applyDiscount,
    chargePayment,
)

// Can modify at runtime
if config.RequireApproval {
    seq.InsertAt(3, requireApproval)
}
```

- Executes processors in order
- Stops on first error
- Supports runtime modification (add, remove, reorder)
- Thread-safe for concurrent use

### Switch

Routes to different processors based on conditions:

```go
// Create a router with a condition function
router := pipz.NewSwitch("payment-router", 
    func(ctx context.Context, p Payment) string {
        switch p.Method {
        case "bitcoin", "ethereum":
            return "crypto"
        case "wire", "ach":
            return "bank"
        default:
            return "card"
        }
    },
)

// Add routes
router.AddRoute("card", processCardPayment)
router.AddRoute("crypto", processCryptoPayment)
router.AddRoute("bank", processBankTransfer)
```

- Type-safe routing with generic keys
- No match returns input unchanged (pass-through behavior)
- Routes can be added dynamically
- Routes are append-only (no removal or modification)

### Concurrent

Runs multiple processors in parallel with isolated data:

> **⚠️ Important:** Concurrent is designed specifically for **independent I/O operations with latency**. The overhead of cloning data for goroutine isolation means it should NOT be used for fast operations like validation or simple calculations.

#### When to Use Concurrent

✅ **Good Use Cases:**
- Multiple API calls that don't depend on each other
- Writing to different external systems (logs, metrics, notifications)
- File uploads to different services
- Database operations to different tables/systems
- Any operation where network/disk I/O dominates processing time

❌ **Bad Use Cases:**
- Fast validation checks (string validation, range checks)
- Simple calculations or data transformations
- Sequential business logic
- Operations that complete in microseconds
- CPU-bound processing

#### Performance Considerations

Concurrent creates a **deep copy** of your data for each processor to ensure goroutine safety. This cloning has overhead:

```go
// ❌ BAD: Fast operations - cloning overhead exceeds benefit
validate := pipz.NewConcurrent("validations",
    validateAge,        // 1μs operation
    validateEmail,      // 1μs operation
    validatePhone,      // 1μs operation
)
// Clone overhead: ~100μs, Total savings: 2μs = NET LOSS!

// ✅ GOOD: I/O operations with real latency
notify := pipz.NewConcurrent("notifications",
    sendEmailAPI,       // 200ms operation
    sendSlackWebhook,   // 150ms operation  
    writeToS3,          // 300ms operation
    updateDatabase,     // 100ms operation
)
// Total time: 300ms (slowest) instead of 750ms sequential = 450ms saved!
```

#### Example Implementation

```go
// Type must implement Cloner interface
type Notification struct {
    OrderID string
    Email   string
    Data    map[string]any
}

func (n Notification) Clone() Notification {
    // Deep copy the map
    data := make(map[string]any, len(n.Data))
    for k, v := range n.Data {
        data[k] = v
    }
    return Notification{
        OrderID: n.OrderID,
        Email:   n.Email,
        Data:    data,
    }
}

// Fire-and-forget parallel I/O operations
notify := pipz.NewConcurrent("notifications",
    pipz.Effect("email", sendEmailNotification),      // 200ms
    pipz.Effect("sms", sendSMSNotification),         // 150ms
    pipz.Effect("push", sendPushNotification),       // 100ms
    pipz.Effect("analytics", trackNotification),     // 250ms
)
// Total execution time: 250ms instead of 700ms
```

**Key characteristics:**
- Each processor gets a clone of the input
- All processors run regardless of individual failures
- Returns original input unchanged
- **Requires** input type to implement `Cloner[T]`
- Can add/remove processors at runtime
- Best for operations measured in milliseconds, not microseconds

**When to use Concurrent:**
✅ External API calls (email, SMS, webhooks)
✅ Database queries that don't depend on each other
✅ File I/O operations
✅ Any operation with network latency

**When NOT to use Concurrent:**
❌ Simple validations (use Sequence)
❌ In-memory transformations
❌ CPU-bound calculations
❌ Operations that complete in < 10ms

The overhead of cloning data and goroutine coordination can exceed the benefit for fast operations.

### Race

First successful result wins:

```go
// Try multiple sources, use fastest response
fetch := pipz.NewRace("data-fetch",
    fetchFromCache,
    fetchFromPrimaryDB,
    fetchFromReplicaDB,
)

```

- Returns first successful result
- Cancels other processors when one succeeds
- Returns error only if all fail
- **Requires** `Cloner[T]` interface
- Useful for reducing latency with redundancy

### Contest

First result meeting a condition wins:

```go
// Find cheapest shipping rate under time constraint
rateContest := pipz.NewContest("rate-shopping",
    func(_ context.Context, rate Rate) bool {
        return rate.Cost < 50.00 && rate.DeliveryDays <= 3
    },
    fedexRates,
    upsRates,
    uspsRates,
)
```

- Combines Race's speed with conditional selection
- First result that meets condition wins and cancels others
- Returns error if no results meet condition
- **Requires** `Cloner[T]` interface
- Perfect for "fastest acceptable" patterns

### Fallback

Try primary, fall back on error:

```go
payment := pipz.NewFallback("payment-processor",
    stripeProcessor,
    paypalProcessor,
)
```

- Only tries fallback if primary fails
- Sequential, not parallel (unlike Race)
- Perfect for primary/backup patterns
- Simple two-option failover

### Retry

Simple retry on failure:

```go
// Retry up to 3 times
save := pipz.NewRetry("reliable-save", databaseWriter, 3)

// Can adjust at runtime
save.SetMaxAttempts(5)
```

- Immediate retries (no delay)
- Configurable max attempts
- Checks context cancellation between attempts
- Returns last error if all attempts fail

### Backoff

Retry with exponential backoff delays:

```go
// Retry with exponential backoff
api := pipz.NewBackoff("api-call",
    apiClient,
    5,                    // max attempts
    100*time.Millisecond, // base delay
)

// Delays: 100ms, 200ms, 400ms, 800ms
// Can adjust at runtime
api.SetMaxAttempts(3)
api.SetBaseDelay(200 * time.Millisecond)
```

- Exponential delays between attempts
- Configurable base delay and max attempts
- Respects context cancellation during delays
- Good for external service calls

### Timeout

Enforce time limits:

```go
fetch := pipz.NewTimeout("fetch-user",
    userServiceCall,
    2*time.Second,
)
```

- Creates new context with timeout
- Cancels operation if time exceeded
- Essential for external calls
- Fixed timeout duration

### Handle

Process errors through their own pipeline:

```go
// Error handling is just another pipeline!
errorPipeline := pipz.NewSequence[*pipz.Error[Order]]("error-handler",
    pipz.Effect("log", func(ctx context.Context, err *pipz.Error[Order]) error {
        log.Printf("Order %s failed at %v: %v", 
            err.InputData.ID, err.Path, err.Err)
        metrics.Increment("errors", "path", strings.Join(err.Path, "."))
        return nil
    }),
    pipz.Apply("categorize", func(ctx context.Context, err *pipz.Error[Order]) (*pipz.Error[Order], error) {
        if err.Timeout {
            err.InputData.Status = "retry"
        }
        return err, nil
    }),
)

observed := pipz.NewHandle("monitored-operation",
    riskyOperation,
    errorPipeline,
)
```

- Error handler receives full `*Error[T]` with context
- Can access original input data, error path, and timing
- Error pipeline can use all pipz features (Switch, Concurrent, etc.)
- Original error is still returned to caller
- Perfect for sophisticated error recovery patterns

## Composition Patterns

### Nested Composition

Connectors compose naturally:

```go
robust := pipz.NewSequence[Order]("robust-pipeline")
robust.Register(
    validate,
    pipz.NewTimeout("protected-operation",
        pipz.NewBackoff("retriable-operation",
            pipz.NewFallback("redundant-operation",
                primaryProcessor,
                backupProcessor,
            ),
            3,
            time.Second,
        ),
        30*time.Second,
    ),
    pipz.NewConcurrent("notifications",
        notifySuccess,
        updateMetrics,
        logResult,
    ),
)
```

### Building Complex Flows

```go
// Create components
validation := pipz.NewSequence[User]("validation")
validation.Register(checkEmail, checkPassword, checkAge)

enrichment := pipz.NewConcurrent[User]("enrichment",
    fetchProfile,
    fetchPreferences,
    fetchHistory,
)

// Compose them
pipeline := pipz.NewSequence[User]("user-flow")
pipeline.Register(
    validation,  // Sequences can be used directly as Chainable[T]
    pipz.NewTimeout("enrichment-timeout", enrichment, 5*time.Second),
    saveUser,
)
```

### Dynamic Configuration

All connectors support runtime modification:

```go
type Pipeline struct {
    sequence *pipz.Sequence[Data]
    router   *pipz.Switch[Data, string]
    retry    *pipz.Retry[Data]
}

func (p *Pipeline) Configure(cfg Config) {
    // Adjust sequence
    if cfg.Debug {
        p.sequence.PushHead(debugLogger)
        p.sequence.PushTail(debugLogger)
    }
    
    // Update routing
    for route, processor := range cfg.Routes {
        p.router.AddRoute(route, processor)
    }
    
    // Tune retry
    p.retry.SetMaxAttempts(cfg.MaxRetries)
}
```

## Connector vs Processor

Choose connectors when you need:
- Runtime configuration changes
- Stateful behavior (retry counts, routes)
- Complex composition patterns
- Thread-safe modifications

Choose processors when you need:
- Simple, pure functions
- Maximum performance
- Immutable behavior
- Functional composition

## Creating Custom Connectors

Custom connectors should follow the pattern:

```go
type MyConnector[T any] struct {
    mu        sync.RWMutex
    name      string
    processor Chainable[T]
    config    MyConfig
}

func NewMyConnector[T any](name Name, processor Chainable[T]) *MyConnector[T] {
    return &MyConnector[T]{
        name:      name,
        processor: processor,
        config:    DefaultConfig(),
    }
}

func (c *MyConnector[T]) Process(ctx context.Context, data T) (T, error) {
    c.mu.RLock()
    config := c.config
    processor := c.processor
    c.mu.RUnlock()
    
    // Your custom logic using config and processor
    return processor.Process(ctx, data)
}

func (c *MyConnector[T]) Name() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.name
}

func (c *MyConnector[T]) SetConfig(cfg MyConfig) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.config = cfg
}
```

## Best Practices

1. **Start with Sequence** - It's the most versatile connector
2. **Add resilience gradually** - Wrap with Retry/Timeout as needed
3. **Implement Cloner correctly** - Deep copy all reference types
4. **Use meaningful names** - They appear in error paths
5. **Minimize lock time** - Copy config values, don't hold locks during processing
6. **Document mutability** - Make it clear what can change at runtime

## Next Steps

- [Pipelines](./pipelines.md) - Composing processors into workflows
- [Error Handling](./error-handling.md) - Building resilient systems
- [Type Safety](./type-safety.md) - Leveraging Go generics
- [API Reference](../api/sequence.md) - Individual connector documentation