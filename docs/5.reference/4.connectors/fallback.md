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
func NewFallback[T any](identity Identity, primary, fallback Chainable[T]) *Fallback[T]
```

## Parameters

- `identity` (`Identity`) - Identifier with name and description for debugging
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
var (
    PaymentID = pipz.NewIdentity("payment", "Process payment with Stripe, fallback to PayPal")
    StripeID  = pipz.NewIdentity("stripe", "Process with Stripe")
    PayPalID  = pipz.NewIdentity("paypal", "Process with PayPal")
)

payment := pipz.NewFallback(
    PaymentID,
    pipz.Apply(StripeID, processWithStripe),
    pipz.Apply(PayPalID, processWithPayPal),
)

// Database with replica fallback
var (
    SaveID       = pipz.NewIdentity("save", "Save to primary database, fallback to replica")
    PrimaryDBID  = pipz.NewIdentity("primary-db", "Save to primary database")
    ReplicaDBID  = pipz.NewIdentity("replica-db", "Save to replica database")
)

saveData := pipz.NewFallback(
    SaveID,
    pipz.Apply(PrimaryDBID, saveToPrimary),
    pipz.Apply(ReplicaDBID, saveToReplica),
)

// API with mock fallback
var (
    WeatherID    = pipz.NewIdentity("weather", "Fetch from weather API, fallback to mock data")
    WeatherAPIID = pipz.NewIdentity("weather-api", "Fetch from weather API")
    MockDataID   = pipz.NewIdentity("mock-data", "Return mock weather data")
)

fetchWeather := pipz.NewFallback(
    WeatherID,
    pipz.Apply(WeatherAPIID, fetchFromWeatherAPI),
    pipz.Apply(MockDataID, returnMockWeather),
)

// Service degradation
var (
    UserLookupID    = pipz.NewIdentity("user-lookup", "Fetch full profile, fallback to basic profile")
    FullProfileID   = pipz.NewIdentity("full-profile", "Fetch full user profile")
    BasicProfileID  = pipz.NewIdentity("basic-profile", "Fetch basic user profile")
)

userService := pipz.NewFallback(
    UserLookupID,
    pipz.Apply(FullProfileID, fetchFullProfile),
    pipz.Apply(BasicProfileID, fetchBasicProfile),
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
var (
    DataFetchID = pipz.NewIdentity("data-fetch", "Fetch data from primary, fallback to backup")
    PrimaryID   = pipz.NewIdentity("primary", "Fetch from primary source")
    BackupID    = pipz.NewIdentity("backup", "Fetch from backup source")
)

fallback := pipz.NewFallback(
    DataFetchID,
    pipz.Apply(PrimaryID, func(ctx context.Context, id string) (Data, error) {
        return Data{}, errors.New("primary failed")
    }),
    pipz.Apply(BackupID, func(ctx context.Context, id string) (Data, error) {
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
var (
    UnrelatedID = pipz.NewIdentity("unrelated", "Unrelated operations")
    SaveID      = pipz.NewIdentity("save", "Save to database")
    EmailID     = pipz.NewIdentity("email", "Send email")
)

fallback := pipz.NewFallback(
    UnrelatedID,
    pipz.Apply(SaveID, saveToDatabase),
    pipz.Apply(EmailID, sendEmail), // Not a fallback!
)
```

### ✅ Use for true alternatives
```go
// RIGHT - Both achieve the same goal
var (
    AlternativesID = pipz.NewIdentity("alternatives", "Save to primary database with backup fallback")
    PrimaryDBID    = pipz.NewIdentity("primary-db", "Save to primary database")
    BackupDBID     = pipz.NewIdentity("backup-db", "Save to backup database")
)

fallback := pipz.NewFallback(
    AlternativesID,
    pipz.Apply(PrimaryDBID, saveToPrimary),
    pipz.Apply(BackupDBID, saveToBackup),
)
```

### ❌ Don't ignore primary errors completely
```go
// WRONG - No visibility into primary failures
var (
    SilentID = pipz.NewIdentity("silent", "Silent fallback without monitoring")
)

fallback := pipz.NewFallback(
    SilentID,
    primary,
    backup,
) // Primary failures are hidden
```

### ✅ Log primary failures for monitoring
```go
// RIGHT - Track primary failures
var (
    MonitoredID          = pipz.NewIdentity("monitored", "Fallback with primary failure monitoring")
    PrimaryWithLoggingID = pipz.NewIdentity("primary-with-logging", "Primary processor with error logging")
    LogID                = pipz.NewIdentity("log", "Log primary failure")
)

fallback := pipz.NewFallback(
    MonitoredID,
    pipz.NewHandle(
        PrimaryWithLoggingID,
        primary,
        pipz.Effect(LogID, func(ctx context.Context, err *pipz.Error[T]) error {
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
var (
    PrimaryID   = pipz.NewIdentity("primary", "Primary with fallback")
    SecondaryID = pipz.NewIdentity("secondary", "Secondary with fallback")
    TertiaryID  = pipz.NewIdentity("tertiary", "Tertiary with fallback")
)

primary := pipz.NewFallback(PrimaryID, processor1, secondary)
secondary := pipz.NewFallback(SecondaryID, processor2, tertiary)
tertiary := pipz.NewFallback(TertiaryID, processor3, primary) // ← Circular!

// If processor1, processor2, and processor3 all fail:
// primary → secondary → tertiary → primary → secondary → ...
// Stack overflow!
```

### ✅ Use linear fallback chains instead
```go
// RIGHT - Clear fallback hierarchy
var (
    PrimaryID   = pipz.NewIdentity("primary", "Primary processor with secondary and tertiary fallbacks")
    SecondaryID = pipz.NewIdentity("secondary", "Secondary processor with tertiary fallback")
)

primary := pipz.NewFallback(
    PrimaryID,
    processor1,
    pipz.NewFallback(
        SecondaryID,
        processor2,
        processor3, // Final fallback - no further chains
    ),
)
```

## Common Patterns

```go
// Chained fallbacks for multiple backups
var (
    MultiBackupID = pipz.NewIdentity("multi-backup", "Multi-tier backup with primary, secondary, and tertiary")
    PrimaryID     = pipz.NewIdentity("primary", "Use primary service")
    BackupsID     = pipz.NewIdentity("backups", "Secondary and tertiary backup chain")
    SecondaryID   = pipz.NewIdentity("secondary", "Use secondary service")
    TertiaryID    = pipz.NewIdentity("tertiary", "Use tertiary service")
)

multiBackup := pipz.NewFallback(
    MultiBackupID,
    pipz.Apply(PrimaryID, usePrimary),
    pipz.NewFallback(
        BackupsID,
        pipz.Apply(SecondaryID, useSecondary),
        pipz.Apply(TertiaryID, useTertiary),
    ),
)

// Fallback with retry
var (
    ResilientSaveID  = pipz.NewIdentity("resilient-save", "Save with retries on primary and backup")
    PrimaryRetryID   = pipz.NewIdentity("primary-retry", "Retry primary save up to 3 times")
    BackupRetryID    = pipz.NewIdentity("backup-retry", "Retry backup save up to 2 times")
)

resilientSave := pipz.NewFallback(
    ResilientSaveID,
    pipz.NewRetry(PrimaryRetryID, saveToPrimary, 3),
    pipz.NewRetry(BackupRetryID, saveToBackup, 2),
)

// Degraded service
var (
    ServiceID     = pipz.NewIdentity("service", "Full service with timeout, fallback to degraded")
    FullServiceID = pipz.NewIdentity("full-service", "Full service with 5 second timeout")
    CompleteID    = pipz.NewIdentity("complete", "Provide full service")
    DegradedID    = pipz.NewIdentity("degraded", "Provide degraded service")
)

fullService := pipz.NewFallback(
    ServiceID,
    pipz.NewTimeout(
        FullServiceID,
        pipz.Apply(CompleteID, provideFullService),
        5*time.Second,
    ),
    pipz.Apply(DegradedID, provideDegradedService),
)

// Development fallback
var (
    APIID  = pipz.NewIdentity("api", "Call real API, fallback to mock in development")
    RealID = pipz.NewIdentity("real", "Call real API")
    MockID = pipz.NewIdentity("mock", "Return mock response")
)

apiCall := pipz.NewFallback(
    APIID,
    pipz.Apply(RealID, callRealAPI),
    pipz.Apply(MockID, func(ctx context.Context, req Request) (Response, error) {
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
var (
    MonitoredID = pipz.NewIdentity("monitored", "Fallback with metrics tracking")
    PrimaryID   = pipz.NewIdentity("primary", "Primary service with metrics")
    FallbackID  = pipz.NewIdentity("fallback", "Fallback service with metrics")
)

monitoredFallback := pipz.NewFallback(
    MonitoredID,
    pipz.Apply(PrimaryID, func(ctx context.Context, data Data) (Data, error) {
        result, err := primaryService(ctx, data)
        if err != nil {
            metrics.Increment("fallback.triggered", "service", "primary")
        }
        return result, err
    }),
    pipz.Apply(FallbackID, func(ctx context.Context, data Data) (Data, error) {
        metrics.Increment("fallback.used", "service", "backup")
        return backupService(ctx, data)
    }),
)

// Log fallback activation
var (
    LoggedFallbackID = pipz.NewIdentity("logged-fallback", "Fallback with error logging")
    ServiceID        = pipz.NewIdentity("service", "Primary and backup service")
    LogFailureID     = pipz.NewIdentity("log-failure", "Log primary failure")
)

loggedFallback := pipz.NewHandle(
    LoggedFallbackID,
    pipz.NewFallback(
        ServiceID,
        primary,
        backup,
    ),
    pipz.Effect(LogFailureID, func(ctx context.Context, err *pipz.Error[Data]) error {
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
var (
    BadID     = pipz.NewIdentity("bad", "Fallback to same database (bad practice)")
    DBWrite1ID = pipz.NewIdentity("db-write-1", "Write to database")
    DBWrite2ID = pipz.NewIdentity("db-write-2", "Write to same database")
)

badFallback := pipz.NewFallback(
    BadID,
    pipz.Apply(DBWrite1ID, writeToDatabase),
    pipz.Apply(DBWrite2ID, writeToSameDatabase), // Same failure mode!
)

// GOOD: Independent failure modes
var (
    GoodID     = pipz.NewIdentity("good", "Fallback from database to file system")
    DatabaseID = pipz.NewIdentity("database", "Write to database")
    FileID     = pipz.NewIdentity("file", "Write to file")
)

goodFallback := pipz.NewFallback(
    GoodID,
    pipz.Apply(DatabaseID, writeToDatabase),
    pipz.Apply(FileID, writeToFile), // Different failure mode
)

// Consider data consistency
var (
    TransactionID = pipz.NewIdentity("transaction", "ACID transaction with eventual consistency fallback")
    PrimaryID     = pipz.NewIdentity("primary", "Process with ACID transaction")
    FallbackID    = pipz.NewIdentity("fallback", "Process with eventual consistency")
)

transactional := pipz.NewFallback(
    TransactionID,
    pipz.Apply(PrimaryID, func(ctx context.Context, tx Transaction) (Transaction, error) {
        // Full ACID transaction
        return processPrimary(ctx, tx)
    }),
    pipz.Apply(FallbackID, func(ctx context.Context, tx Transaction) (Transaction, error) {
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