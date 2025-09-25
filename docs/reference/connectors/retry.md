# Retry

Retries a processor up to a specified number of attempts, with optional exponential backoff.

## Function Signatures

```go
// Simple retry without delays
func NewRetry[T any](name Name, processor Chainable[T], maxAttempts int) *Retry[T]

// Retry with exponential backoff
func NewBackoff[T any](name Name, processor Chainable[T], maxAttempts int, baseDelay time.Duration) *Retry[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
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
// Use fake clock in tests
fakeClock := clockz.NewFakeClock()
backoff := pipz.NewBackoff("test", processor, 3, 100*time.Millisecond).
    WithClock(fakeClock)

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
// Simple retry
reliableAPI := pipz.NewRetry("api-retry",
    pipz.Apply("api-call", callFlakyAPI),
    3, // Try up to 3 times
)

// Retry with backoff
resilientService := pipz.NewBackoff("service-retry",
    pipz.Apply("external-service", callExternalService),
    5,                        // Max 5 attempts
    100*time.Millisecond,     // 100ms, 200ms, 400ms, 800ms delays
)

// Retry a complex operation
saveWithRetry := pipz.NewRetry("save-retry",
    pipz.NewSequence[Order]("save-flow",
        pipz.Apply("validate", validateOrder),
        pipz.Apply("calculate", calculateTotals),
        pipz.Apply("persist", saveToDatabase),
    ),
    3,
)

// Graduated retry strategy
smartRetry := pipz.NewFallback("graduated-retry",
    pipz.NewRetry("quick-retry", processor, 2),              // 2 quick attempts
    pipz.NewBackoff("slow-retry", processor, 3, time.Second), // Then slower
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

## Observability

Retry provides comprehensive observability through metrics, tracing, and hook events.

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `retry.attempts.total` | Counter | Total retry attempts across all operations |
| `retry.successes.total` | Counter | Operations that eventually succeeded |
| `retry.exhausted.total` | Counter | Operations that failed after all attempts |
| `retry.attempt.current` | Gauge | Current attempt number during execution |
| `retry.duration.ms` | Gauge | Total operation duration including retries |

### Traces

| Span | Description |
|------|-------------|
| `retry.process` | Parent span for entire retry operation |
| `retry.attempt` | Child span for each individual attempt |

**Span Tags:**
- `retry.max_attempts` - Maximum attempts configured
- `retry.attempt_num` - Current attempt number
- `retry.success` - Whether operation succeeded
- `retry.exhausted` - Whether all attempts were exhausted
- `retry.attempts_used` - Total attempts used

### Hook Events

| Event | Key | Description |
|-------|-----|-------------|
| Attempt | `retry.attempt` | Fired before each retry attempt |
| Success | `retry.success` | Fired when operation succeeds |
| Exhausted | `retry.exhausted` | Fired when all attempts fail |

### Event Handlers

```go
// Monitor retry attempts
retry.OnAttempt(func(ctx context.Context, event RetryEvent) error {
    log.Debug("Retry attempt %d/%d for %s",
        event.AttemptNum, event.MaxAttempts, event.Name)
    return nil
})

// Track success patterns
retry.OnSuccess(func(ctx context.Context, event RetryEvent) error {
    log.Info("Succeeded after %d attempts", event.AttemptNum)
    metrics.Record("retry.attempts_to_success", event.AttemptNum)
    return nil
})

// Alert on exhaustion
retry.OnExhausted(func(ctx context.Context, event RetryEvent) error {
    log.Error("All %d attempts failed: %v",
        event.MaxAttempts, event.LastError)
    alert.Error("Retry exhausted for %s", event.Name)
    return nil
})
```

## Gotchas

### ❌ Don't retry non-idempotent operations
```go
// WRONG - Each retry charges the card again!
retry := pipz.NewRetry("charge",
    pipz.Apply("payment", chargeCard),
    3,
)
```

### ✅ Make operations idempotent first
```go
// RIGHT - Use idempotency key
retry := pipz.NewRetry("charge",
    pipz.Apply("payment", func(ctx context.Context, payment Payment) (Payment, error) {
        payment.IdempotencyKey = generateIdempotencyKey(payment)
        return chargeCardIdempotent(ctx, payment)
    }),
    3,
)
```

### ❌ Don't retry validation errors
```go
// WRONG - Will never succeed
retry := pipz.NewRetry("validate",
    pipz.Apply("check", func(ctx context.Context, email string) (string, error) {
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
// RIGHT - Check error type
retry := pipz.NewRetry("smart",
    pipz.Apply("api", func(ctx context.Context, req Request) (Response, error) {
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
retry := pipz.NewRetry("api", flakyProcessor, 3)
_, err := retry.Process(ctx, input)
if err != nil {
    // Error message includes retry information
    // Example: "api failed after 3 attempts: connection timeout"
}
```

## Common Patterns

```go
// Network operations with backoff
httpClient := pipz.NewBackoff("http-client",
    pipz.Apply("request", makeHTTPRequest),
    5,
    500*time.Millisecond, // 0.5s, 1s, 2s, 4s
)

// Database operations with quick retry
dbOperation := pipz.NewRetry("db-op",
    pipz.Apply("query", runDatabaseQuery),
    3, // Handle transient deadlocks
)

// Cascading retry strategy
cascadingRetry := pipz.NewSequence[Data]("cascading",
    pipz.Apply("validate", validate),
    pipz.NewRetry("quick", quickOperation, 2),
    pipz.NewBackoff("slow", slowOperation, 5, time.Second),
)

// Retry with circuit breaker pattern
type CircuitBreaker struct {
    failures int
    mu       sync.Mutex
}

circuitBreaker := pipz.NewSequence[Request]("circuit",
    pipz.Apply("check-circuit", func(ctx context.Context, req Request) (Request, error) {
        cb.mu.Lock()
        defer cb.mu.Unlock()
        if cb.failures > 10 {
            return req, errors.New("circuit open")
        }
        return req, nil
    }),
    pipz.NewRetry("protected-call", protectedOperation, 3),
)
```

## Advanced Patterns

```go
// Custom backoff strategy
customBackoff := pipz.NewBackoff("custom",
    pipz.Apply("operation", func(ctx context.Context, data Data) (Data, error) {
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
intelligentRetry := pipz.NewHandle("intelligent",
    processor,
    pipz.NewSwitch("error-router",
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
    AddRoute("timeout", pipz.NewRetry("timeout-retry", processor, 5)).
    AddRoute("rate-limit", pipz.NewBackoff("rate-retry", processor, 3, 30*time.Second)).
    AddRoute("other", pipz.NewRetry("general-retry", processor, 2)),
)
```

## Observability

### Metrics

The Retry connector tracks the following metrics using `metricz`:

- `retry.attempts.total` - Counter of total retry attempts
- `retry.successes.total` - Counter of successful retries
- `retry.failures.total` - Counter of exhausted retries (all attempts failed)
- `retry.attempt.current` - Gauge showing current attempt number during processing

### Tracing

The Retry connector creates spans using `tracez`:

- `retry.process` - Parent span for the entire retry operation
- `retry.attempt` - Child span for each individual attempt

Span tags:
- `retry.connector` - Name of the retry connector
- `retry.max_attempts` - Maximum attempts configured
- `retry.attempt` - Current attempt number (on attempt spans)
- `retry.attempts_used` - Total attempts used (on process span)
- `retry.success` - Whether the operation succeeded
- `retry.exhausted` - Whether all attempts were exhausted
- `retry.error` - Error message (on failed attempts)
- `retry.canceled` - Whether operation was canceled

### Example Usage

```go
// Create retry with observability
retry := pipz.NewRetry("api-retry", processor, 3)
defer retry.Close() // Clean up observability resources

// Access metrics
metrics := retry.Metrics()
attempts := metrics.Counter(pipz.RetryAttemptsTotal).Value()
successes := metrics.Counter(pipz.RetrySuccessesTotal).Value()
failures := metrics.Counter(pipz.RetryFailuresTotal).Value()

// Process with automatic observability
result, err := retry.Process(ctx, data)
```

## Monitoring Retries

```go
// Access built-in metrics for monitoring
monitoredRetry := pipz.NewRetry("monitored", processor, 3)

// Export metrics to external systems
stats := map[string]float64{
    "attempts": monitoredRetry.Metrics().Counter(pipz.RetryAttemptsTotal).Value(),
    "successes": monitoredRetry.Metrics().Counter(pipz.RetrySuccessesTotal).Value(),
    "failures": monitoredRetry.Metrics().Counter(pipz.RetryFailuresTotal).Value(),
    "current_attempt": monitoredRetry.Metrics().Gauge(pipz.RetryAttemptCurrent).Value(),
}

// Send to Prometheus, DataDog, etc.
for name, value := range stats {
    prometheus.Gauge(name).Set(value)
}
```

## See Also

- [Fallback](./fallback.md) - For trying different processors
- [Timeout](./timeout.md) - Often combined with retry
- [Handle](../processors/handle.md) - For custom retry logic
- [Race](./race.md) - For parallel attempts