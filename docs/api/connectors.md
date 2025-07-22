# Connectors API Reference

Complete reference for all connector functions.

## Sequential Execution

### Sequential

Runs processors in order, passing output to input.

```go
func Sequential[T any](chainables ...Chainable[T]) Chainable[T]
```

**Parameters:**
- `chainables` - Variable number of processors to execute in order

**Behavior:**
- Executes each processor with the output of the previous
- Stops immediately on first error
- Returns final output or error

**Example:**
```go
pipeline := pipz.Sequential(
    validateInput,
    transformData,
    saveResult,
)
```

## Conditional Routing

### Switch

Routes to different processors based on a condition function.

```go
func Switch[T any, K comparable](
    condition Condition[T, K],
    routes map[K]Chainable[T],
) Chainable[T]
```

**Type Parameters:**
- `T` - Data type being processed
- `K` - Route key type (must be comparable)

**Parameters:**
- `condition` - Function that returns a route key
- `routes` - Map of route keys to processors

**Behavior:**
- Evaluates condition with input data
- Routes to corresponding processor
- Returns error if no matching route

**Example:**
```go
type Status string
const (
    StatusNew    Status = "new"
    StatusActive Status = "active"
)

router := pipz.Switch(
    func(ctx context.Context, user User) Status {
        if user.ActivatedAt.IsZero() {
            return StatusNew
        }
        return StatusActive
    },
    map[Status]pipz.Chainable[User]{
        StatusNew:    newUserPipeline,
        StatusActive: activeUserPipeline,
    },
)
```

## Parallel Execution

### Concurrent

Runs multiple processors in parallel with isolated data copies.

```go
func Concurrent[T Cloner[T]](processors ...Chainable[T]) Chainable[T]
```

**Type Constraints:**
- `T` must implement `Cloner[T]` interface

**Parameters:**
- `processors` - Processors to run concurrently

**Behavior:**
- Each processor receives a clone of the input
- All processors run regardless of individual failures
- Returns original input unchanged
- Waits for all to complete or context cancellation

**Example:**
```go
notifications := pipz.Concurrent(
    sendEmailNotification,
    sendSMSNotification,
    updateAnalytics,
)
```

### Race

Runs processors in parallel and returns the first successful result.

```go
func Race[T Cloner[T]](processors ...Chainable[T]) Chainable[T]
```

**Type Constraints:**
- `T` must implement `Cloner[T]` interface

**Parameters:**
- `processors` - Processors to race

**Behavior:**
- Each processor receives a clone of the input
- First successful result cancels others
- Returns error only if all fail
- Returns last error if all fail

**Example:**
```go
fetchData := pipz.Race(
    fetchFromCache,
    fetchFromPrimary,
    fetchFromReplica,
)
```

## Error Handling

### Fallback

Tries primary processor, falls back to secondary on error.

```go
func Fallback[T any](primary, fallback Chainable[T]) Chainable[T]
```

**Parameters:**
- `primary` - Primary processor to try first
- `fallback` - Fallback processor if primary fails

**Behavior:**
- Executes primary processor
- On error, executes fallback processor
- Returns primary result if successful

**Example:**
```go
payment := pipz.Fallback(
    processWithStripe,
    processWithPayPal,
)
```

### Retry

Retries a processor up to maxAttempts times.

```go
func Retry[T any](chainable Chainable[T], maxAttempts int) Chainable[T]
```

**Parameters:**
- `chainable` - Processor to retry
- `maxAttempts` - Maximum number of attempts (min 1)

**Behavior:**
- Retries immediately on failure
- No delay between attempts
- Checks context cancellation between attempts
- Returns last error with attempt count

**Example:**
```go
reliable := pipz.Retry(flakeyOperation, 3)
```

### RetryWithBackoff

Retries with exponential backoff between attempts.

```go
func RetryWithBackoff[T any](
    chainable Chainable[T],
    maxAttempts int,
    baseDelay time.Duration,
) Chainable[T]
```

**Parameters:**
- `chainable` - Processor to retry
- `maxAttempts` - Maximum number of attempts
- `baseDelay` - Initial delay between attempts

**Behavior:**
- Delays double after each failure (exponential backoff)
- Respects context cancellation during delays
- No delay after final attempt

**Delay Pattern:** baseDelay, 2×baseDelay, 4×baseDelay, ...

**Example:**
```go
apiCall := pipz.RetryWithBackoff(
    callExternalAPI,
    5,                    // max 5 attempts
    100*time.Millisecond, // 100ms, 200ms, 400ms, 800ms
)
```

### WithErrorHandler

Adds error observation without changing behavior.

```go
func WithErrorHandler[T any](
    processor Chainable[T],
    errorHandler Chainable[error],
) Chainable[T]
```

**Parameters:**
- `processor` - Processor to wrap
- `errorHandler` - Processor to handle errors

**Behavior:**
- Runs error handler on failure
- Original error is still returned
- Error handler errors are ignored
- Success passes through unchanged

**Example:**
```go
tracked := pipz.WithErrorHandler(
    riskyOperation,
    pipz.Effect("log_error", func(ctx context.Context, err error) error {
        log.Printf("Operation failed: %v", err)
        metrics.Increment("errors")
        return nil
    }),
)
```

## Timeout Control

### Timeout

Enforces a time limit on processor execution.

```go
func Timeout[T any](chainable Chainable[T], duration time.Duration) Chainable[T]
```

**Parameters:**
- `chainable` - Processor to time-bound
- `duration` - Maximum execution time

**Behavior:**
- Creates timeout context
- Cancels operation if timeout exceeded
- Returns timeout error with duration info
- Processor should respect context cancellation

**Example:**
```go
fast := pipz.Timeout(slowOperation, 5*time.Second)
```

## Composition Rules

1. **Type Consistency**: All processors in a connector must process the same type T
2. **Context Propagation**: Context flows through all connectors
3. **Error Semantics**: First error stops Sequential, but not Concurrent
4. **Nesting**: Connectors can be arbitrarily nested

**Complex Example:**
```go
robust := pipz.Sequential(
    pipz.Apply("validate", validate),
    pipz.Timeout(
        pipz.RetryWithBackoff(
            pipz.Fallback(
                primaryProcessor,
                backupProcessor,
            ),
            3,
            time.Second,
        ),
        30*time.Second,
    ),
    pipz.Concurrent(
        notifyUsers,
        updateMetrics,
        logResult,
    ),
)
```

## Performance Considerations

| Connector | Overhead | Use When |
|-----------|----------|----------|
| Sequential | Minimal | Default choice |
| Switch | Single map lookup | Need routing |
| Concurrent | Clone + goroutines | Independent operations |
| Race | Clone + goroutines + cancellation | Need fastest result |
| Fallback | Only on error | Primary/backup pattern |
| Retry | Only on error | Transient failures |
| Timeout | Goroutine + timer | Slow operations |

## Next Steps

- [Pipeline API](./pipeline.md) - Dynamic pipeline management
- [Error Types](./errors.md) - Error handling reference
- [Examples](../examples/payment-processing.md) - Real-world usage