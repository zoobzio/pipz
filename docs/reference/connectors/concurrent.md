# Concurrent

Runs multiple processors in parallel with isolated data copies.

## Function Signature

```go
func NewConcurrent[T Cloner[T]](name Name, processors ...Chainable[T]) *Concurrent[T]
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
- `processors` - Variable number of processors to run concurrently

## Returns

Returns a `*Concurrent[T]` that implements `Chainable[T]`.

## Behavior

- **Parallel execution** - All processors run simultaneously
- **Data isolation** - Each processor receives a clone of the input
- **Non-failing** - Individual failures don't stop other processors
- **Wait for all** - Waits for all processors to complete
- **Returns original** - Always returns the original input data
- **Context preservation** - Passes original context to all processors, preserving distributed tracing and context values
- **Cancellation support** - Parent context cancellation affects all child processors

## Example

```go
// Define a type that implements Cloner
type User struct {
    ID      string
    Name    string
    Email   string
    Tags    []string
}

func (u User) Clone() User {
    tags := make([]string, len(u.Tags))
    copy(tags, u.Tags)
    return User{
        ID:    u.ID,
        Name:  u.Name,
        Email: u.Email,
        Tags:  tags,
    }
}

// Create concurrent processor
notifications := pipz.NewConcurrent("notify-all",
    pipz.Effect("email", sendEmailNotification),
    pipz.Effect("sms", sendSMSNotification),
    pipz.Effect("push", sendPushNotification),
    pipz.Effect("audit", logToAuditTrail),
)

// Use in a pipeline
pipeline := pipz.NewSequence[User]("user-update",
    pipz.Apply("validate", validateUser),
    pipz.Apply("update", updateDatabase),
    notifications, // All notifications sent in parallel
)
```

## When to Use

Use `Concurrent` when:
- Operations are **independent and can run in parallel**
- You want to fire multiple actions simultaneously
- Side effects can run in parallel (notifications, logging)
- Individual failures shouldn't affect others
- You need to notify multiple systems
- Performance benefit from parallelization

## When NOT to Use

Don't use `Concurrent` when:
- You need the result from processors (always returns original)
- Operations must run in order (use `Sequence`)
- Type doesn't implement `Cloner[T]` (compilation error)
- You need to stop on first error (all run to completion)
- Operations share state or resources (race conditions)
- You need fastest result (use `Race`)

## Error Handling

Concurrent continues even if some processors fail:

```go
concurrent := pipz.NewConcurrent("multi-save",
    pipz.Apply("primary", saveToPrimary),    // Might fail
    pipz.Apply("backup", saveToBackup),      // Still runs
    pipz.Effect("cache", updateCache),       // Still runs
)

// The original data is returned regardless of individual failures
result, err := concurrent.Process(ctx, data)
// err is nil even if some processors failed
// result is the original data
```

## Performance Considerations

- Creates one goroutine per processor
- Requires data cloning (allocation cost)
- All processors run even if some finish early
- Context cancellation stops waiting processors

## Observability

Concurrent provides comprehensive observability through metrics, tracing, and hook events.

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `concurrent.processed.total` | Counter | Total concurrent operations |
| `concurrent.successes.total` | Counter | Operations where all processors completed |
| `concurrent.tasks.total` | Counter | Total tasks launched across all operations |
| `concurrent.duration.ms` | Gauge | Total operation duration in milliseconds |

### Traces

| Span | Description |
|------|-------------|
| `concurrent.process` | Parent span for entire concurrent operation |
| `concurrent.task` | Child span for each individual processor |

**Span Tags:**
- `concurrent.processor_count` - Number of processors running
- `concurrent.processor_name` - Name of individual processor (on task spans)
- `concurrent.success` - Whether all processors completed
- `concurrent.error` - Error if any processor failed

### Hook Events

| Event | Key | Description |
|-------|-----|-------------|
| Task Started | `concurrent.task_started` | Fired when a processor starts |
| Task Complete | `concurrent.task_complete` | Fired when a processor finishes |
| All Complete | `concurrent.all_complete` | Fired when all processors finish |

### Event Handlers

```go
// Monitor task execution
concurrent.OnTaskStarted(func(ctx context.Context, event ConcurrentEvent) error {
    log.Debug("Starting processor %s", event.ProcessorName)
    return nil
})

// Track individual completions
concurrent.OnTaskComplete(func(ctx context.Context, event ConcurrentEvent) error {
    if event.Error != nil {
        log.Error("Processor %s failed: %v", event.ProcessorName, event.Error)
    } else {
        log.Info("Processor %s completed in %v", event.ProcessorName, event.Duration)
    }
    return nil
})

// Monitor overall completion
concurrent.OnAllComplete(func(ctx context.Context, event ConcurrentEvent) error {
    log.Info("All %d processors completed in %v (failures: %d)",
        event.TotalTasks, event.TotalDuration, event.FailedTasks)
    if event.FailedTasks > 0 {
        alert.Warn("%d concurrent tasks failed", event.FailedTasks)
    }
    return nil
})
```

## Common Patterns

```go
// Parallel notifications
userNotifications := pipz.NewConcurrent("notifications",
    pipz.Effect("email", sendWelcomeEmail),
    pipz.Effect("sms", sendWelcomeSMS),
    pipz.Effect("crm", updateCRM),
    pipz.Effect("analytics", trackSignup),
)

// Parallel data distribution
distribute := pipz.NewConcurrent("distribute",
    pipz.Apply("elasticsearch", indexInElastic),
    pipz.Apply("redis", cacheInRedis),
    pipz.Apply("s3", uploadToS3),
    pipz.Effect("metrics", recordMetrics),
)

// Multi-channel processing
processOrder := pipz.NewSequence[Order]("order-flow",
    pipz.Apply("validate", validateOrder),
    pipz.Apply("payment", processPayment),
    pipz.NewConcurrent("post-payment",
        pipz.Effect("inventory", updateInventory),
        pipz.Effect("shipping", createShippingLabel),
        pipz.Effect("email", sendConfirmation),
        pipz.Effect("analytics", trackRevenue),
    ),
)
```

## Gotchas

### ❌ Don't forget Concurrent doesn't return results
```go
// WRONG - Expecting modified data
concurrent := pipz.NewConcurrent("modify",
    pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is still 5, not 10!
```

### ✅ Use for side effects only
```go
// RIGHT - Side effects, not transformations
concurrent := pipz.NewConcurrent("effects",
    pipz.Effect("log", logData),
    pipz.Effect("metrics", updateMetrics),
)
```

### ❌ Don't share state between processors
```go
// WRONG - Race condition!
var counter int
concurrent := pipz.NewConcurrent("racy",
    pipz.Effect("inc1", func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
    pipz.Effect("inc2", func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
)
```

### ✅ Use proper synchronization or avoid shared state
```go
// RIGHT - No shared state
concurrent := pipz.NewConcurrent("safe",
    pipz.Effect("db1", saveToDatabase1),
    pipz.Effect("db2", saveToDatabase2),
)
```

## Implementation Requirements

Your type must implement `Clone()` correctly:

```go
// Simple struct
type Event struct {
    ID        string
    Type      string
    Timestamp time.Time
}

func (e Event) Clone() Event {
    return e // Struct with only value types can be copied directly
}

// Struct with slices/maps
type Document struct {
    ID       string
    Sections []Section
    Metadata map[string]string
}

func (d Document) Clone() Document {
    sections := make([]Section, len(d.Sections))
    copy(sections, d.Sections)
    
    metadata := make(map[string]string, len(d.Metadata))
    for k, v := range d.Metadata {
        metadata[k] = v
    }
    
    return Document{
        ID:       d.ID,
        Sections: sections,
        Metadata: metadata,
    }
}
```

## See Also

- [Race](./race.md) - For getting the first successful result
- [Sequence](./sequence.md) - For sequential execution
- [Effect](./effect.md) - Common processor for concurrent operations