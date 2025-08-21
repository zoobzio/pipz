# Effect

Creates a processor that performs side effects without modifying the input data.

> **Note**: Effect is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Effect[T any](name Name, fn func(context.Context, T) error) Chainable[T]
```

## Parameters

- `name` (`Name`) - Identifier for the processor used in error messages and debugging
- `fn` - Side effect function that takes context and input, returns error on failure

## Returns

Returns a `Chainable[T]` that passes through the original input unchanged (unless error occurs).

## Behavior

- **Pass-through** - Original input is returned unchanged
- **Side effects only** - Used for logging, metrics, notifications, etc.
- **Can fail** - Errors stop pipeline execution
- **Context aware** - Respects cancellation and timeouts

## Example

```go
// Logging
logger := pipz.Effect("log", func(ctx context.Context, order Order) error {
    log.Printf("Processing order: %s, amount: %.2f", order.ID, order.Total)
    return nil
})

// Metrics
metrics := pipz.Effect("metrics", func(ctx context.Context, event Event) error {
    if err := metricsClient.Increment("events.processed", 
        "type", event.Type,
        "source", event.Source,
    ); err != nil {
        return fmt.Errorf("metrics failed: %w", err)
    }
    return nil
})

// Audit trail
audit := pipz.Effect("audit", func(ctx context.Context, user User) error {
    entry := AuditEntry{
        UserID:    user.ID,
        Action:    "profile_update",
        Timestamp: time.Now(),
        IP:        getIPFromContext(ctx),
    }
    return auditLog.Write(ctx, entry)
})

// Validation (no data modification)
checkPermissions := pipz.Effect("auth", func(ctx context.Context, req Request) error {
    if !req.User.HasPermission(req.Action) {
        return errors.New("permission denied")
    }
    return nil
})
```

## When to Use

Use `Effect` when:
- You need **side effects without data changes** (logging, metrics, notifications)
- You're writing audit trails that must succeed
- You need validation without modification
- You want to maintain data immutability
- Recording events or telemetry
- Triggering external systems

## When NOT to Use

Don't use `Effect` when:
- You need to modify the data (use `Transform` or `Apply`)
- The side effect is optional (consider `Enrich`)
- You want to ignore errors (wrap with error handling)
- Computing values to add to data (use `Transform`)
- The operation returns useful data (use `Apply`)

## Performance

Effect has similar performance to Apply:
- ~46ns per operation (success case)
- Zero allocations on success
- Original data is passed through without copying

## Common Patterns

```go
// Observability pipeline
observability := pipz.NewSequence[Order]("observe",
    pipz.Effect("log", logOrder),
    pipz.Effect("metrics", recordMetrics),
    pipz.Effect("trace", addTraceSpan),
)

// Notification effects running in parallel
notifications := pipz.NewConcurrent[User](
    pipz.Effect("email", sendEmail),
    pipz.Effect("sms", sendSMS),
    pipz.Effect("push", sendPushNotification),
)

// Conditional logging
debugLog := pipz.Mutate("debug",
    func(ctx context.Context, data Data) bool {
        return os.Getenv("DEBUG") == "true"
    },
    func(ctx context.Context, data Data) Data {
        log.Printf("DEBUG: %+v", data)
        return data
    },
)

// Critical validation
authorize := pipz.Effect("authorize", func(ctx context.Context, req Request) error {
    if !isAuthorized(ctx, req.UserID, req.Resource) {
        return fmt.Errorf("unauthorized access to %s", req.Resource)
    }
    return nil
})
```

## Gotchas

### ❌ Don't modify the input
```go
// WRONG - Effect shouldn't modify data
effect := pipz.Effect("bad", func(ctx context.Context, user *User) error {
    user.LastSeen = time.Now() // Modifying!
    return nil
})
```

### ✅ Use Transform for modifications
```go
// RIGHT - Use Transform to modify
transform := pipz.Transform("update", func(ctx context.Context, user User) User {
    user.LastSeen = time.Now()
    return user
})
```

### ❌ Don't return data through side channels
```go
// WRONG - Using closure to smuggle data out
var result string
effect := pipz.Effect("fetch", func(ctx context.Context, id string) error {
    data, err := fetchData(id)
    result = data // Side channel!
    return err
})
```

### ✅ Use Apply for operations that return data
```go
// RIGHT - Proper data flow
apply := pipz.Apply("fetch", func(ctx context.Context, id string) (Data, error) {
    return fetchData(id)
})
```

## Error Handling

Effect errors include the same rich context as other processors:

```go
audit := pipz.Effect("audit", func(ctx context.Context, tx Transaction) error {
    if err := auditDB.Log(ctx, tx); err != nil {
        // This error will stop the pipeline
        return fmt.Errorf("audit log failed: %w", err)
    }
    return nil
})

// To make effects optional, wrap with error handling
optionalAudit := pipz.NewEnrich("optional-audit", 
    func(ctx context.Context, tx Transaction) (Transaction, error) {
        if err := auditDB.Log(ctx, tx); err != nil {
            // Log but don't fail
            log.Printf("Audit failed (continuing): %v", err)
            return tx, err // Will be ignored by Enrich
        }
        return tx, nil
    },
)
```

## See Also

- [Transform](./transform.md) - For data transformations
- [Apply](./apply.md) - For operations that modify data
- [Enrich](./enrich.md) - For optional side effects