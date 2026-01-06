---
title: "Scaffold"
description: "Fire-and-forget parallel execution with context isolation for background operations"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - processors
  - async
  - background
  - fire-and-forget
---

# Scaffold

Fire-and-forget parallel execution with context isolation.

## Function Signature

```go
func NewScaffold[T Cloner[T]](identity Identity, processors ...Chainable[T]) *Scaffold[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `identity` (`Identity`) - Identifier for the connector used in debugging
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

// Define identities upfront
var (
    AsyncOpsID   = pipz.NewIdentity("async-operations", "Background operations for event processing")
    AuditLogID   = pipz.NewIdentity("audit-log", "Writes to audit log")
    AnalyticsID  = pipz.NewIdentity("analytics", "Sends to analytics")
    CacheWarmID  = pipz.NewIdentity("cache-warm", "Warms secondary cache")
    MetricsID    = pipz.NewIdentity("metrics", "Updates metrics")
    UserActionID = pipz.NewIdentity("user-action", "User action pipeline")
    ValidateID   = pipz.NewIdentity("validate", "Validates action")
    AuthorizeID  = pipz.NewIdentity("authorize", "Checks permissions")
    ExecuteID    = pipz.NewIdentity("execute", "Performs action")
)

// Create scaffold for background operations
background := pipz.NewScaffold(AsyncOpsID,
    pipz.Effect(AuditLogID, writeToAuditLog),      // 500ms operation
    pipz.Effect(AnalyticsID, sendToAnalytics),      // 300ms operation
    pipz.Effect(CacheWarmID, warmSecondaryCache),  // 200ms operation
    pipz.Effect(MetricsID, updateMetrics),          // 100ms operation
)

// Use in a pipeline - returns immediately
pipeline := pipz.NewSequence[AuditEvent](UserActionID,
    pipz.Apply(ValidateID, validateAction),    // Must complete
    pipz.Apply(AuthorizeID, checkPermissions), // Must complete
    pipz.Apply(ExecuteID, performAction),      // Must complete
    background,                                // Returns immediately
)

// Process returns after ~10ms (validation + auth + execute)
// Background tasks continue running for up to 500ms
result, err := pipeline.Process(ctx, event)
```

## When to Use

Use `Scaffold` when:
- Operations must **complete regardless of request lifecycle**
- You need true fire-and-forget semantics
- Background tasks shouldn't block the main flow
- Audit logging or metrics collection that must always happen
- Cache warming or cleanup tasks
- Non-critical notifications (email, SMS)
- Analytics and telemetry that shouldn't affect performance

## When NOT to Use

Don't use `Scaffold` when:
- You need results from the processors (use `Concurrent`)
- Errors must be handled or logged (no error reporting)
- Operations should be cancelled with the request (use `Concurrent`)
- You need to wait for completion (use `Concurrent`)
- Critical business logic is involved (use synchronous processors)
- You need confirmation of success (no feedback mechanism)

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

    scaffold := pipz.NewScaffold(
        pipz.NewIdentity("background", "Background operation with trace context"),
        pipz.Effect(
            pipz.NewIdentity("log", "Logs with trace ID"),
            func(ctx context.Context, _ Request) error {
                // ctx still contains trace ID: "trace-123"
                // But won't be cancelled when request ends
                traceID := ctx.Value("trace-id").(string)
                log.Printf("[%s] Background operation", traceID)
                time.Sleep(5 * time.Second) // Continues even after request done
                return nil
            },
        ),
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
// Define identities upfront
var (
    AuditID          = pipz.NewIdentity("audit", "Audit logging to multiple destinations")
    PrimaryLogID     = pipz.NewIdentity("primary-log", "Writes to database")
    BackupLogID      = pipz.NewIdentity("backup-log", "Writes to S3")
    ComplianceID     = pipz.NewIdentity("compliance", "Sends to compliance system")
    MonitoringID     = pipz.NewIdentity("monitoring", "Updates monitoring systems")
    PrometheusID     = pipz.NewIdentity("prometheus", "Updates Prometheus metrics")
    DatadogID        = pipz.NewIdentity("datadog", "Sends to Datadog")
    CustomID         = pipz.NewIdentity("custom", "Updates custom dashboard")
    CacheOpsID       = pipz.NewIdentity("cache-ops", "Cache operations")
    RedisID          = pipz.NewIdentity("redis", "Warms Redis cache")
    CDNID            = pipz.NewIdentity("cdn", "Purges CDN cache")
    LocalID          = pipz.NewIdentity("local", "Updates local cache")
    OrderProcessingID = pipz.NewIdentity("order-processing", "Order processing pipeline")
    ValidateOrderID  = pipz.NewIdentity("validate", "Validates order")
    PaymentID        = pipz.NewIdentity("payment", "Processes payment")
    InventoryID      = pipz.NewIdentity("inventory", "Updates inventory")
    CompleteID       = pipz.NewIdentity("complete", "Marks order complete")
    PostOrderID      = pipz.NewIdentity("post-order", "Post-order notifications and analytics")
    EmailID          = pipz.NewIdentity("email", "Sends confirmation email")
    SMSID            = pipz.NewIdentity("sms", "Sends SMS notification")
    OrderAnalyticsID = pipz.NewIdentity("analytics", "Tracks order metrics")
    PartnerID        = pipz.NewIdentity("partner", "Notifies fulfillment partner")
)

// Audit logging
auditLog := pipz.NewScaffold(AuditID,
    pipz.Effect(PrimaryLogID, writeToDatabase),
    pipz.Effect(BackupLogID, writeToS3),
    pipz.Effect(ComplianceID, sendToComplianceSystem),
)

// Metrics and monitoring
monitoring := pipz.NewScaffold(MonitoringID,
    pipz.Effect(PrometheusID, updatePrometheusMetrics),
    pipz.Effect(DatadogID, sendToDatadog),
    pipz.Effect(CustomID, updateCustomDashboard),
)

// Cache warming
cacheOps := pipz.NewScaffold(CacheOpsID,
    pipz.Effect(RedisID, warmRedisCache),
    pipz.Effect(CDNID, purgeCDNCache),
    pipz.Effect(LocalID, updateLocalCache),
)

// Complete pipeline with synchronous and async parts
pipeline := pipz.NewSequence[Order](OrderProcessingID,
    // Synchronous - must complete
    pipz.Apply(ValidateOrderID, validateOrder),
    pipz.Apply(PaymentID, processPayment),
    pipz.Apply(InventoryID, updateInventory),

    // Returns order immediately after this
    pipz.Transform(CompleteID, markOrderComplete),

    // Async - fire and forget
    pipz.NewScaffold(PostOrderID,
        pipz.Effect(EmailID, sendConfirmationEmail),
        pipz.Effect(SMSID, sendSMSNotification),
        pipz.Effect(OrderAnalyticsID, trackOrderMetrics),
        pipz.Effect(PartnerID, notifyFulfillmentPartner),
    ),
)
```

## Gotchas

### ❌ Don't use for critical operations
```go
// Define identities upfront
var (
    CriticalID = pipz.NewIdentity("critical", "Critical payment processing")
    PaymentID  = pipz.NewIdentity("payment", "Processes payment")
)

// WRONG - Payment must be confirmed!
scaffold := pipz.NewScaffold(CriticalID,
    pipz.Apply(PaymentID, processPayment), // No error feedback!
)
```

### ✅ Use synchronous processing for critical ops
```go
// Define identities upfront
var (
    CriticalID = pipz.NewIdentity("critical", "Critical payment processing")
    PaymentID  = pipz.NewIdentity("payment", "Processes payment")
)

// RIGHT - Wait for payment confirmation
sequence := pipz.NewSequence(CriticalID,
    pipz.Apply(PaymentID, processPayment),
)
```

### ❌ Don't forget to implement Clone properly
```go
// WRONG - Shallow copy of slice
func (d Data) Clone() Data {
    return Data{Items: d.Items} // Shares slice memory!
}
```

### ✅ Deep copy all reference types
```go
// RIGHT - Deep copy
func (d Data) Clone() Data {
    items := make([]Item, len(d.Items))
    copy(items, d.Items)
    return Data{Items: items}
}
```

### ❌ Don't use when you need error handling
```go
// Define identities upfront
var (
    NoErrorsID = pipz.NewIdentity("no-errors", "No error handling")
    RiskyID    = pipz.NewIdentity("risky", "Risky operation")
)

// WRONG - Can't handle or log errors
scaffold := pipz.NewScaffold(NoErrorsID,
    pipz.Apply(RiskyID, riskyOperation), // Errors vanish!
)
```

### ✅ Use Concurrent if you need to know about failures
```go
// Define identities upfront
var (
    WithErrorsID = pipz.NewIdentity("with-errors", "With error handling")
    RiskyID      = pipz.NewIdentity("risky", "Risky operation")
)

// RIGHT - Errors are reported (though not individually)
concurrent := pipz.NewConcurrent(WithErrorsID,
    pipz.Apply(RiskyID, riskyOperation),
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