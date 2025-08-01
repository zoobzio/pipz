# Scaffold

Fire-and-forget parallel execution with context isolation.

## Function Signature

```go
func NewScaffold[T Cloner[T]](name Name, processors ...Chainable[T]) *Scaffold[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `processors` - Variable number of processors to run asynchronously

## Returns

Returns a `*Scaffold[T]` that implements `Chainable[T]`.

## Behavior

- **Fire-and-forget** - Returns immediately without waiting
- **Context isolation** - Uses `context.WithoutCancel()` to prevent parent cancellation
- **Data isolation** - Each processor receives a clone of the input
- **No error reporting** - Individual failures are not reported back
- **Returns original** - Always returns the original input data immediately
- **Trace preservation** - Preserves trace IDs and context values while removing cancellation
- **Background execution** - Processors continue even after parent context cancellation

## Example

```go
// Define a type that implements Cloner
type AuditEvent struct {
    UserID    string
    Action    string
    Timestamp time.Time
    Metadata  map[string]string
}

func (a AuditEvent) Clone() AuditEvent {
    metadata := make(map[string]string, len(a.Metadata))
    for k, v := range a.Metadata {
        metadata[k] = v
    }
    return AuditEvent{
        UserID:    a.UserID,
        Action:    a.Action,
        Timestamp: a.Timestamp,
        Metadata:  metadata,
    }
}

// Create scaffold for background operations
background := pipz.NewScaffold("async-operations",
    pipz.Effect("audit-log", writeToAuditLog),      // 500ms operation
    pipz.Effect("analytics", sendToAnalytics),      // 300ms operation
    pipz.Effect("cache-warm", warmSecondaryCache),  // 200ms operation
    pipz.Effect("metrics", updateMetrics),          // 100ms operation
)

// Use in a pipeline - returns immediately
pipeline := pipz.NewSequence[AuditEvent]("user-action",
    pipz.Apply("validate", validateAction),    // Must complete
    pipz.Apply("authorize", checkPermissions), // Must complete
    pipz.Apply("execute", performAction),      // Must complete
    background,                                // Returns immediately
)

// Process returns after ~10ms (validation + auth + execute)
// Background tasks continue running for up to 500ms
result, err := pipeline.Process(ctx, event)
```

## When to Use

Use `Scaffold` when:
- Operations must complete regardless of request lifecycle
- You need true fire-and-forget semantics
- Background tasks shouldn't block the main flow
- Audit logging or metrics collection
- Cache warming or cleanup tasks
- Non-critical notifications

## When NOT to Use

Don't use `Scaffold` when:
- You need results from the processors
- Errors must be handled or logged
- Operations should be cancelled with the request
- You need to wait for completion
- Critical business logic is involved

## Scaffold vs Concurrent

| Feature | Scaffold | Concurrent |
|---------|----------|------------|
| Returns | Immediately | After all complete |
| Context Cancellation | Ignored (continues) | Respected (stops) |
| Error Reporting | No | No (but waits) |
| Use Case | Background tasks | Parallel operations |
| Trace Context | Preserved | Preserved |

## Context Handling

Scaffold uses `context.WithoutCancel()` which:
- Preserves all context values (trace IDs, request IDs, etc.)
- Removes cancellation signals
- Allows processors to outlive the parent request

```go
// Example with distributed tracing
func handleRequest(ctx context.Context, req Request) {
    // ctx contains trace ID: "trace-123"
    
    scaffold := pipz.NewScaffold("background",
        pipz.Effect("log", func(ctx context.Context, _ Request) error {
            // ctx still contains trace ID: "trace-123"
            // But won't be cancelled when request ends
            traceID := ctx.Value("trace-id").(string)
            log.Printf("[%s] Background operation", traceID)
            time.Sleep(5 * time.Second) // Continues even after request done
            return nil
        }),
    )
    
    // Returns immediately
    scaffold.Process(ctx, req)
}
```

## Performance Considerations

- Creates one goroutine per processor
- Requires data cloning (allocation cost)
- No synchronization overhead (fire-and-forget)
- Goroutines are not tracked or managed
- Memory usage depends on processor lifetime

## Common Patterns

```go
// Audit logging
auditLog := pipz.NewScaffold("audit",
    pipz.Effect("primary-log", writeToDatabase),
    pipz.Effect("backup-log", writeToS3),
    pipz.Effect("compliance", sendToComplianceSystem),
)

// Metrics and monitoring
monitoring := pipz.NewScaffold("monitoring",
    pipz.Effect("prometheus", updatePrometheusMetrics),
    pipz.Effect("datadog", sendToDatadog),
    pipz.Effect("custom", updateCustomDashboard),
)

// Cache warming
cacheOps := pipz.NewScaffold("cache-ops",
    pipz.Effect("redis", warmRedisCache),
    pipz.Effect("cdn", purgeCDNCache),
    pipz.Effect("local", updateLocalCache),
)

// Complete pipeline with synchronous and async parts
pipeline := pipz.NewSequence[Order]("order-processing",
    // Synchronous - must complete
    pipz.Apply("validate", validateOrder),
    pipz.Apply("payment", processPayment),
    pipz.Apply("inventory", updateInventory),
    
    // Returns order immediately after this
    pipz.Transform("complete", markOrderComplete),
    
    // Async - fire and forget
    pipz.NewScaffold("post-order",
        pipz.Effect("email", sendConfirmationEmail),
        pipz.Effect("sms", sendSMSNotification),
        pipz.Effect("analytics", trackOrderMetrics),
        pipz.Effect("partner", notifyFulfillmentPartner),
    ),
)
```

## Implementation Requirements

Your type must implement `Clone()` correctly for thread safety:

```go
// Struct with reference types needs deep copying
type Notification struct {
    ID        string
    Channels  []string
    Data      map[string]interface{}
    Template  *Template
}

func (n Notification) Clone() Notification {
    // Deep copy slice
    channels := make([]string, len(n.Channels))
    copy(channels, n.Channels)
    
    // Deep copy map
    data := make(map[string]interface{}, len(n.Data))
    for k, v := range n.Data {
        data[k] = v
    }
    
    // Copy pointer if needed
    var template *Template
    if n.Template != nil {
        t := *n.Template
        template = &t
    }
    
    return Notification{
        ID:       n.ID,
        Channels: channels,
        Data:     data,
        Template: template,
    }
}
```

## See Also

- [Concurrent](./concurrent.md) - For parallel operations that wait for completion
- [Effect](./effect.md) - Common processor for fire-and-forget operations
- [Handle](./handle.md) - For error monitoring without blocking