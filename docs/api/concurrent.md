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
- **Wait for all** - Waits for all processors or context cancellation
- **Returns original** - Always returns the original input data

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
- Operations are independent
- You want to fire-and-forget multiple actions
- Side effects can run in parallel
- Individual failures shouldn't affect others
- You need to notify multiple systems

## When NOT to Use

Don't use `Concurrent` when:
- You need the result from processors (use `Race` instead)
- Operations must run in order (use `Sequence`)
- Type doesn't implement `Cloner[T]`
- You need to stop on first error

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