---
title: "Retry"
description: "Retries a processor up to a specified number of attempts for handling transient failures"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - retry
  - resilience
  - transient-failures
---

# Retry

Retries a processor up to a specified number of attempts, with optional exponential backoff.

## Function Signatures

```go
// Simple retry without delays
func NewRetry[T any](identity Identity, processor Chainable[T], maxAttempts int) *Retry[T]

// Retry with exponential backoff
func NewBackoff[T any](identity Identity, processor Chainable[T], maxAttempts int, baseDelay time.Duration) *Retry[T]
```

## Parameters

- `identity` (`Identity`) - Identity containing name and description for debugging and documentation
- `processor` - The processor to retry on failure
- `maxAttempts` - Maximum number of attempts (minimum 1)
- `baseDelay` - (Backoff only) Initial delay between attempts

## Returns

Returns a `*Retry[T]` that implements `Chainable[T]`.

## Testing Configuration

### WithClock

```go
func (b *Backoff[T]) WithClock(clock clockz.Clock) *Backoff[T]
```

Sets a custom clock implementation for testing purposes. This method enables controlled time manipulation in tests using `clockz.FakeClock`. Available only on Backoff (created with `NewBackoff`), not on simple Retry.

**Parameters:**
- `clock` (`clockz.Clock`) - Clock implementation to use

**Returns:**
Returns the same connector instance for method chaining.

**Example:**
```go
// Define identity
var TestBackoffID = pipz.NewIdentity("test", "test backoff with fake clock")

// Use fake clock in tests
fakeClock := clockz.NewFakeClock()
backoff := pipz.NewBackoff(
    TestBackoffID,
    processor, 3, 100*time.Millisecond,
).WithClock(fakeClock)

// Advance time in test to trigger delays
fakeClock.Advance(200 * time.Millisecond)
```

## Behavior

### NewRetry
- **Immediate retry** - No delay between attempts
- **Stops on success** - Returns immediately when processor succeeds
- **Context check** - Checks for cancellation between attempts
- **Error includes attempts** - Final error shows retry count

### NewBackoff
- **Exponential delays** - Delay doubles after each failure
- **Pattern** - baseDelay, 2×baseDelay, 4×baseDelay, etc.
- **No final delay** - No delay after the last attempt
- **Jittered delays** - Small randomization to prevent thundering herd

## Example

```go
// Define identities
var (
    APIRetryID      = pipz.NewIdentity("api-retry", "retry flaky API calls up to 3 times")
    APICallID       = pipz.NewIdentity("api-call", "call external API")
    ServiceRetryID  = pipz.NewIdentity("service-retry", "retry external service with exponential backoff")
    ExternalSvcID   = pipz.NewIdentity("external-service", "call external service")
    SaveRetryID     = pipz.NewIdentity("save-retry", "retry order save operation")
    SaveFlowID      = pipz.NewIdentity("save-flow", "validate, calculate, and persist order")
    ValidateID      = pipz.NewIdentity("validate", "validate order data")
    CalculateID     = pipz.NewIdentity("calculate", "calculate order totals")
    PersistID       = pipz.NewIdentity("persist", "save order to database")
    GraduatedID     = pipz.NewIdentity("graduated-retry", "graduated retry: quick attempts then slow backoff")
    QuickRetryID    = pipz.NewIdentity("quick-retry", "2 quick retry attempts without delay")
    SlowRetryID     = pipz.NewIdentity("slow-retry", "3 slower retry attempts with backoff")
)

// Simple retry
reliableAPI := pipz.NewRetry(
    APIRetryID,
    pipz.Apply(APICallID, callFlakyAPI),
    3, // Try up to 3 times
)

// Retry with backoff
resilientService := pipz.NewBackoff(
    ServiceRetryID,
    pipz.Apply(ExternalSvcID, callExternalService),
    5,                        // Max 5 attempts
    100*time.Millisecond,     // 100ms, 200ms, 400ms, 800ms delays
)

// Retry a complex operation
saveWithRetry := pipz.NewRetry(
    SaveRetryID,
    pipz.NewSequence(
        SaveFlowID,
        pipz.Apply(ValidateID, validateOrder),
        pipz.Apply(CalculateID, calculateTotals),
        pipz.Apply(PersistID, saveToDatabase),
    ),
    3,
)

// Graduated retry strategy
smartRetry := pipz.NewFallback(
    GraduatedID,
    pipz.NewRetry(QuickRetryID, processor, 2),
    pipz.NewBackoff(SlowRetryID, processor, 3, time.Second),
)
```

## When to Use

Use `Retry` when:
- Dealing with **transient failures** (network blips, temporary unavailability)
- Network operations that may timeout
- External services with occasional failures
- Database deadlocks or conflicts
- Rate limit errors (with backoff)
- Operations are idempotent (safe to repeat)

Use `Backoff` specifically when:
- You need to respect rate limits
- Avoiding thundering herd problems
- External service needs recovery time
- Exponential backoff is required by API
- Load shedding is important

## When NOT to Use

Don't use `Retry` when:
- Errors are permanent (validation failures, business logic errors)
- Operations are not idempotent (payments, incrementing counters)
- Fast failure is preferred (user-facing APIs)
- Different approach needed on failure (use `Fallback`)
- Error indicates a bug (null pointer, index out of bounds)

## Gotchas

### ❌ Don't retry non-idempotent operations
```go
// Define identities
var (
    ChargeRetryID = pipz.NewIdentity("charge", "retry payment charge")
    PaymentID     = pipz.NewIdentity("payment", "charge card")
)

// WRONG - Each retry charges the card again!
retry := pipz.NewRetry(
    ChargeRetryID,
    pipz.Apply(PaymentID, chargeCard),
    3,
)
```

### ✅ Make operations idempotent first
```go
// Define identities
var (
    IdempotentChargeID = pipz.NewIdentity("charge", "retry idempotent payment charge")
    IdempotentPayID    = pipz.NewIdentity("payment", "charge card with idempotency key")
)

// RIGHT - Use idempotency key
retry := pipz.NewRetry(
    IdempotentChargeID,
    pipz.Apply(IdempotentPayID, func(ctx context.Context, payment Payment) (Payment, error) {
        payment.IdempotencyKey = generateIdempotencyKey(payment)
        return chargeCardIdempotent(ctx, payment)
    }),
    3,
)
```

### ❌ Don't retry validation errors
```go
// Define identities
var (
    ValidateRetryID = pipz.NewIdentity("validate", "retry email validation")
    CheckEmailID    = pipz.NewIdentity("check", "check email format")
)

// WRONG - Will never succeed
retry := pipz.NewRetry(
    ValidateRetryID,
    pipz.Apply(CheckEmailID, func(ctx context.Context, email string) (string, error) {
        if !strings.Contains(email, "@") {
            return "", errors.New("invalid email") // Permanent error!
        }
        return email, nil
    }),
    5, // Wastes 5 attempts
)
```

### ✅ Only retry transient errors
```go
// Define identities
var (
    SmartRetryID = pipz.NewIdentity("smart", "smart retry that distinguishes transient from permanent errors")
    APICheckID   = pipz.NewIdentity("api", "call API with error type checking")
)

// RIGHT - Check error type
retry := pipz.NewRetry(
    SmartRetryID,
    pipz.Apply(APICheckID, func(ctx context.Context, req Request) (Response, error) {
        resp, err := callAPI(ctx, req)
        if err != nil {
            if isPermanentError(err) {
                return resp, fmt.Errorf("permanent: %w", err) // Mark as permanent
            }
            return resp, err // Transient, will retry
        }
        return resp, nil
    }),
    3,
)
```

## Error Messages

Retry enriches errors with attempt information:

```go
// Define identity
var APIRetryID = pipz.NewIdentity("api", "retry flaky API processor")

retry := pipz.NewRetry(APIRetryID, flakyProcessor, 3)
_, err := retry.Process(ctx, input)
if err != nil {
    // Error message includes retry information
    // Example: "api failed after 3 attempts: connection timeout"
}
```

## Common Patterns

```go
// Define identities
var (
    HTTPClientID    = pipz.NewIdentity("http-client", "HTTP client with exponential backoff retry")
    RequestID       = pipz.NewIdentity("request", "make HTTP request")
    DBOpID          = pipz.NewIdentity("db-op", "database operation with quick retry for deadlocks")
    QueryID         = pipz.NewIdentity("query", "run database query")
    CascadingID     = pipz.NewIdentity("cascading", "cascading retry strategy with validation and progressive delays")
    ValidateDataID  = pipz.NewIdentity("validate", "validate data")
    QuickOpID       = pipz.NewIdentity("quick", "quick operation with immediate retry")
    SlowOpID        = pipz.NewIdentity("slow", "slow operation with exponential backoff")
    CircuitID       = pipz.NewIdentity("circuit", "circuit breaker with retry protection")
    CheckCircuitID  = pipz.NewIdentity("check-circuit", "check if circuit is open")
    ProtectedCallID = pipz.NewIdentity("protected-call", "retry protected operation")
)

// Network operations with backoff
httpClient := pipz.NewBackoff(
    HTTPClientID,
    pipz.Apply(RequestID, makeHTTPRequest),
    5,
    500*time.Millisecond, // 0.5s, 1s, 2s, 4s
)

// Database operations with quick retry
dbOperation := pipz.NewRetry(
    DBOpID,
    pipz.Apply(QueryID, runDatabaseQuery),
    3, // Handle transient deadlocks
)

// Cascading retry strategy
cascadingRetry := pipz.NewSequence(
    CascadingID,
    pipz.Apply(ValidateDataID, validate),
    pipz.NewRetry(QuickOpID, quickOperation, 2),
    pipz.NewBackoff(SlowOpID, slowOperation, 5, time.Second),
)

// Retry with circuit breaker pattern
type CircuitBreaker struct {
    failures int
    mu       sync.Mutex
}

circuitBreaker := pipz.NewSequence(
    CircuitID,
    pipz.Apply(CheckCircuitID, func(ctx context.Context, req Request) (Request, error) {
        cb.mu.Lock()
        defer cb.mu.Unlock()
        if cb.failures > 10 {
            return req, errors.New("circuit open")
        }
        return req, nil
    }),
    pipz.NewRetry(ProtectedCallID, protectedOperation, 3),
)
```

## Advanced Patterns

```go
// Define identities
var (
    CustomBackoffID   = pipz.NewIdentity("custom", "custom backoff with rate limit awareness")
    OperationID       = pipz.NewIdentity("operation", "operation with rate limit handling")
    IntelligentID     = pipz.NewIdentity("intelligent", "intelligent retry with error-specific strategies")
    ErrorRouterID     = pipz.NewIdentity("error-router", "route to retry strategy based on error type")
    TimeoutRetryID    = pipz.NewIdentity("timeout-retry", "retry timeout errors aggressively")
    RateLimitRetryID  = pipz.NewIdentity("rate-retry", "retry rate limit errors with long backoff")
    GeneralRetryID    = pipz.NewIdentity("general-retry", "retry other errors conservatively")
)

// Custom backoff strategy
customBackoff := pipz.NewBackoff(
    CustomBackoffID,
    pipz.Apply(OperationID, func(ctx context.Context, data Data) (Data, error) {
        // Check for specific error types
        result, err := operation(ctx, data)
        if err != nil {
            var rateLimitErr *RateLimitError
            if errors.As(err, &rateLimitErr) {
                // Wait for rate limit reset
                select {
                case <-time.After(rateLimitErr.ResetAfter):
                case <-ctx.Done():
                    return data, ctx.Err()
                }
            }
        }
        return result, err
    }),
    3,
    time.Second,
)

// Retry with different strategies per error
intelligentRetry := pipz.NewHandle(
    IntelligentID,
    processor,
    pipz.NewSwitch(
        ErrorRouterID,
        func(ctx context.Context, err *pipz.Error[Data]) string {
            if err.Timeout {
                return "timeout"
            }
            if strings.Contains(err.Err.Error(), "rate limit") {
                return "rate-limit"
            }
            return "other"
        },
    ).
    AddRoute("timeout", pipz.NewRetry(TimeoutRetryID, processor, 5)).
    AddRoute("rate-limit", pipz.NewBackoff(RateLimitRetryID, processor, 3, 30*time.Second)).
    AddRoute("other", pipz.NewRetry(GeneralRetryID, processor, 2)),
)
```

## See Also

- [Fallback](./fallback.md) - For trying different processors
- [Timeout](./timeout.md) - Often combined with retry
- [Handle](../3.processors/handle.md) - For custom retry logic
- [Race](./race.md) - For parallel attempts