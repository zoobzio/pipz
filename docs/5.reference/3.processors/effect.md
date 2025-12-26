---
title: "Effect"
description: "Creates a processor that performs side effects without modifying the input data"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - processors
  - side-effects
  - logging
---

# Effect

Creates a processor that performs side effects without modifying the input data.

> **Note**: Effect is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Effect[T any](identity Identity, fn func(context.Context, T) error) Chainable[T]
```

## Parameters

- `identity` (`Identity`) - Identifier for the processor used in error messages and debugging
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
logger := pipz.Effect(
    pipz.NewIdentity("log-order", "Logs order processing details"),
    func(ctx context.Context, order Order) error {
        log.Printf("Processing order: %s, amount: %.2f", order.ID, order.Total)
        return nil
    },
)

// Metrics
metrics := pipz.Effect(
    pipz.NewIdentity("record-metrics", "Records event metrics"),
    func(ctx context.Context, event Event) error {
        if err := metricsClient.Increment("events.processed",
            "type", event.Type,
            "source", event.Source,
        ); err != nil {
            return fmt.Errorf("metrics failed: %w", err)
        }
        return nil
    },
)

// Audit trail
audit := pipz.Effect(
    pipz.NewIdentity("audit-trail", "Writes audit log entry"),
    func(ctx context.Context, user User) error {
        entry := AuditEntry{
            UserID:    user.ID,
            Action:    "profile_update",
            Timestamp: time.Now(),
            IP:        getIPFromContext(ctx),
        }
        return auditLog.Write(ctx, entry)
    },
)

// Validation (no data modification)
checkPermissions := pipz.Effect(
    pipz.NewIdentity("check-permissions", "Validates user permissions"),
    func(ctx context.Context, req Request) error {
        if !req.User.HasPermission(req.Action) {
            return errors.New("permission denied")
        }
        return nil
    },
)
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
// Define identities upfront
var (
    ObserveID       = pipz.NewIdentity("observe", "Order observability pipeline")
    LogOrderID      = pipz.NewIdentity("log-order", "Logs order details")
    RecordMetricsID = pipz.NewIdentity("record-metrics", "Records order metrics")
    AuditOrderID    = pipz.NewIdentity("audit-order", "Audits order processing")
    NotificationsID = pipz.NewIdentity("notifications", "Notification effects")
    SendEmailID     = pipz.NewIdentity("send-email", "Sends email notification")
    SendSMSID       = pipz.NewIdentity("send-sms", "Sends SMS notification")
    SendPushID      = pipz.NewIdentity("send-push", "Sends push notification")
)

// Custom observability pipeline
observe := pipz.NewSequence[Order](ObserveID,
    pipz.Effect(LogOrderID, logOrder),
    pipz.Effect(RecordMetricsID, recordMetrics),
    pipz.Effect(AuditOrderID, auditOrder),
)

// Notification effects running in parallel
notifications := pipz.NewConcurrent[User](NotificationsID,
    pipz.Effect(SendEmailID, sendEmail),
    pipz.Effect(SendSMSID, sendSMS),
    pipz.Effect(SendPushID, sendPushNotification),
)

// Conditional logging
debugLog := pipz.Mutate(
    pipz.NewIdentity("debug-log", "Logs debug information when enabled"),
    func(ctx context.Context, data Data) Data {
        log.Printf("DEBUG: %+v", data)
        return data
    },
    func(ctx context.Context, data Data) bool {
        return os.Getenv("DEBUG") == "true"
    },
)

// Critical validation
authorize := pipz.Effect(
    pipz.NewIdentity("authorize-request", "Authorizes user access to resource"),
    func(ctx context.Context, req Request) error {
        if !isAuthorized(ctx, req.UserID, req.Resource) {
            return fmt.Errorf("unauthorized access to %s", req.Resource)
        }
        return nil
    },
)
```

## Gotchas

### ❌ Don't modify the input
```go
// WRONG - Effect shouldn't modify data
effect := pipz.Effect(
    pipz.NewIdentity("bad-effect", "Incorrectly modifies data"),
    func(ctx context.Context, user *User) error {
        user.LastSeen = time.Now() // Modifying!
        return nil
    },
)
```

### ✅ Use Transform for modifications
```go
// RIGHT - Use Transform to modify
transform := pipz.Transform(
    pipz.NewIdentity("update-last-seen", "Updates last seen timestamp"),
    func(ctx context.Context, user User) User {
        user.LastSeen = time.Now()
        return user
    },
)
```

### ❌ Don't return data through side channels
```go
// WRONG - Using closure to smuggle data out
var result string
effect := pipz.Effect(
    pipz.NewIdentity("fetch-data", "Fetches data via side channel"),
    func(ctx context.Context, id string) error {
        data, err := fetchData(id)
        result = data // Side channel!
        return err
    },
)
```

### ✅ Use Apply for operations that return data
```go
// RIGHT - Proper data flow
apply := pipz.Apply(
    pipz.NewIdentity("fetch-data", "Fetches data from external source"),
    func(ctx context.Context, id string) (Data, error) {
        return fetchData(id)
    },
)
```

## Error Handling

Effect errors include the same rich context as other processors:

```go
audit := pipz.Effect(
    pipz.NewIdentity("audit-transaction", "Logs transaction to audit database"),
    func(ctx context.Context, tx Transaction) error {
        if err := auditDB.Log(ctx, tx); err != nil {
            // This error will stop the pipeline
            return fmt.Errorf("audit log failed: %w", err)
        }
        return nil
    },
)

// To make effects optional, wrap with error handling
optionalAudit := pipz.NewEnrich(
    pipz.NewIdentity("optional-audit", "Attempts to log transaction audit"),
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