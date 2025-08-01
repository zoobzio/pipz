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
- Dealing with transient failures
- Network operations that may timeout
- External services with occasional failures
- Database deadlocks or conflicts
- Rate limit errors (with backoff)

Use `Backoff` specifically when:
- You need to respect rate limits
- Avoiding thundering herd problems
- External service needs recovery time
- Exponential backoff is required by API

## When NOT to Use

Don't use `Retry` when:
- Errors are permanent (validation failures)
- Operations are not idempotent
- Fast failure is preferred
- Different approach needed on failure (use `Fallback`)

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

## Monitoring Retries

```go
// Track retry metrics
monitoredRetry := pipz.NewRetry("monitored",
    pipz.Apply("operation", func(ctx context.Context, data Data) (Data, error) {
        result, err := operation(ctx, data)
        if err != nil {
            metrics.Increment("retry.needed", "operation", "myop")
        } else {
            metrics.Increment("retry.success", "operation", "myop")
        }
        return result, err
    }),
    3,
)

// Log retry attempts
loggedRetry := pipz.NewHandle("logged-retry",
    pipz.NewRetry("operation", processor, 3),
    pipz.Effect("log", func(ctx context.Context, err *pipz.Error[Data]) error {
        log.Printf("Retry failed at attempt %d: %v", 
            extractAttemptNumber(err.Err.Error()), err.Err)
        return nil
    }),
)
```

## See Also

- [Fallback](./fallback.md) - For trying different processors
- [Timeout](./timeout.md) - Often combined with retry
- [Handle](./handle.md) - For custom retry logic
- [Race](./race.md) - For parallel attempts