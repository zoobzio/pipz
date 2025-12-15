---
title: "Fallback"
description: "Tries a primary processor and falls back to a secondary on error for automatic failover"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - resilience
  - failover
  - backup
---

# Fallback

Tries a primary processor, falls back to secondary on error.

## Function Signature

```go
func NewFallback[T any](name Name, primary, fallback Chainable[T]) *Fallback[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `primary` - The primary processor to try first
- `fallback` - The backup processor to use if primary fails

## Returns

Returns a `*Fallback[T]` that implements `Chainable[T]`.

## Behavior

- **Try primary first** - Always attempts the primary processor
- **Fallback on error** - Only tries fallback if primary fails
- **Success stops** - Returns immediately if primary succeeds
- **Error propagation** - Returns fallback error if both fail
- **Context awareness** - Respects cancellation throughout

## Example

```go
// Payment processing with backup
payment := pipz.NewFallback("payment",
    pipz.Apply("stripe", processWithStripe),
    pipz.Apply("paypal", processWithPayPal),
)

// Database with replica fallback
saveData := pipz.NewFallback("save",
    pipz.Apply("primary-db", saveToPrimary),
    pipz.Apply("replica-db", saveToReplica),
)

// API with mock fallback
fetchWeather := pipz.NewFallback("weather",
    pipz.Apply("weather-api", fetchFromWeatherAPI),
    pipz.Apply("mock-data", returnMockWeather),
)

// Service degradation
userService := pipz.NewFallback("user-lookup",
    pipz.Apply("full-profile", fetchFullProfile),
    pipz.Apply("basic-profile", fetchBasicProfile),
)
```

## When to Use

Use `Fallback` when:
- You have a **primary and backup service** (database replicas, payment providers)
- You want automatic failover
- The fallback provides acceptable results
- You need service resilience
- Order of preference is clear
- Graceful degradation is acceptable

## When NOT to Use

Don't use `Fallback` when:
- You need to try more than two options (chain `Fallback`s or use `Race`)
- Both processors should always run (use `Concurrent`)
- You need the fastest result (use `Race`)
- Failure reasons matter for routing (use `Switch` with error handling)
- Primary failure should stop everything (no fallback needed)

## Error Handling

Fallback returns the fallback's error if both fail:

```go
fallback := pipz.NewFallback("data-fetch",
    pipz.Apply("primary", func(ctx context.Context, id string) (Data, error) {
        return Data{}, errors.New("primary failed")
    }),
    pipz.Apply("backup", func(ctx context.Context, id string) (Data, error) {
        return Data{}, errors.New("backup failed")
    }),
)

result, err := fallback.Process(ctx, "123")
// err.Error() == "backup failed" (the fallback's error)
```

## Gotchas

### ❌ Don't use for unrelated operations
```go
// WRONG - These aren't alternatives
fallback := pipz.NewFallback("unrelated",
    pipz.Apply("save", saveToDatabase),
    pipz.Apply("email", sendEmail), // Not a fallback!
)
```

### ✅ Use for true alternatives
```go
// RIGHT - Both achieve the same goal
fallback := pipz.NewFallback("alternatives",
    pipz.Apply("primary-db", saveToPrimary),
    pipz.Apply("backup-db", saveToBackup),
)
```

### ❌ Don't ignore primary errors completely
```go
// WRONG - No visibility into primary failures
fallback := pipz.NewFallback("silent",
    primary,
    backup,
) // Primary failures are hidden
```

### ✅ Log primary failures for monitoring
```go
// RIGHT - Track primary failures
fallback := pipz.NewFallback("monitored",
    pipz.NewHandle("primary-with-logging",
        primary,
        pipz.Effect("log", func(ctx context.Context, err *pipz.Error[T]) error {
            log.Printf("Primary failed, using fallback: %v", err)
            metrics.Increment("fallback.triggered")
            return nil
        }),
    ),
    backup,
)
```

### ❌ Don't create circular fallback chains
```go
// WRONG - Creates infinite recursion risk
primary := pipz.NewFallback("primary", processor1, secondary)
secondary := pipz.NewFallback("secondary", processor2, tertiary)  
tertiary := pipz.NewFallback("tertiary", processor3, primary) // ← Circular!

// If processor1, processor2, and processor3 all fail:
// primary → secondary → tertiary → primary → secondary → ...
// Stack overflow!
```

### ✅ Use linear fallback chains instead
```go
// RIGHT - Clear fallback hierarchy
primary := pipz.NewFallback("primary",
    processor1,
    pipz.NewFallback("secondary", 
        processor2,
        processor3, // Final fallback - no further chains
    ),
)
```

## Common Patterns

```go
// Chained fallbacks for multiple backups
multiBackup := pipz.NewFallback("multi-backup",
    pipz.Apply("primary", usePrimary),
    pipz.NewFallback("backups",
        pipz.Apply("secondary", useSecondary),
        pipz.Apply("tertiary", useTertiary),
    ),
)

// Fallback with retry
resilientSave := pipz.NewFallback("resilient-save",
    pipz.NewRetry("primary-retry", saveToPrimary, 3),
    pipz.NewRetry("backup-retry", saveToBackup, 2),
)

// Degraded service
fullService := pipz.NewFallback("service",
    pipz.NewTimeout("full-service",
        pipz.Apply("complete", provideFullService),
        5*time.Second,
    ),
    pipz.Apply("degraded", provideDegradedService),
)

// Development fallback
apiCall := pipz.NewFallback("api",
    pipz.Apply("real", callRealAPI),
    pipz.Apply("mock", func(ctx context.Context, req Request) (Response, error) {
        if os.Getenv("ENV") == "development" {
            return mockResponse(req), nil
        }
        return Response{}, errors.New("production only")
    }),
)
```

## Monitoring Fallbacks

```go
// Track fallback usage
monitoredFallback := pipz.NewFallback("monitored",
    pipz.Apply("primary", func(ctx context.Context, data Data) (Data, error) {
        result, err := primaryService(ctx, data)
        if err != nil {
            metrics.Increment("fallback.triggered", "service", "primary")
        }
        return result, err
    }),
    pipz.Apply("fallback", func(ctx context.Context, data Data) (Data, error) {
        metrics.Increment("fallback.used", "service", "backup")
        return backupService(ctx, data)
    }),
)

// Log fallback activation
loggedFallback := pipz.NewHandle("logged-fallback",
    pipz.NewFallback("service", primary, backup),
    pipz.Effect("log-failure", func(ctx context.Context, err *pipz.Error[Data]) error {
        if strings.Contains(err.Path[len(err.Path)-1], "primary") {
            log.Printf("Primary failed, using fallback: %v", err.Err)
        }
        return nil
    }),
)
```

## Best Practices

```go
// Ensure fallback is truly independent
// BAD: Fallback might fail for same reason
badFallback := pipz.NewFallback("bad",
    pipz.Apply("db-write-1", writeToDatabase),
    pipz.Apply("db-write-2", writeToSameDatabase), // Same failure mode!
)

// GOOD: Independent failure modes
goodFallback := pipz.NewFallback("good",
    pipz.Apply("database", writeToDatabase),
    pipz.Apply("file", writeToFile), // Different failure mode
)

// Consider data consistency
transactional := pipz.NewFallback("transaction",
    pipz.Apply("primary", func(ctx context.Context, tx Transaction) (Transaction, error) {
        // Full ACID transaction
        return processPrimary(ctx, tx)
    }),
    pipz.Apply("fallback", func(ctx context.Context, tx Transaction) (Transaction, error) {
        // Ensure fallback maintains consistency
        log.Printf("WARNING: Using eventual consistency fallback for tx %s", tx.ID)
        return processEventually(ctx, tx)
    }),
)
```

## See Also

- [Race](./race.md) - For trying multiple options in parallel
- [Retry](./retry.md) - For retrying the same processor
- [Handle](../3.processors/handle.md) - For custom error handling
- [Switch](./switch.md) - For conditional routing