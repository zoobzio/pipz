# Backoff

Retry with exponential backoff for handling transient failures.

## Overview

`Backoff` provides intelligent retry logic with exponentially increasing delays between attempts. Unlike simple `Retry` which uses fixed delays, Backoff progressively increases wait times to reduce load on failing systems and improve recovery chances.

## Function Signature

```go
func NewBackoff[T any](
    name Name,
    processor Chainable[T],
    maxAttempts int,
    baseDelay time.Duration,
) *Backoff[T]
```

## Type Parameters

- `T` - The data type being processed

## Parameters

- `name` (`Name`) - Identifier for debugging and error paths
- `processor` (`Chainable[T]`) - The processor to retry on failure
- `maxAttempts` (`int`) - Maximum number of retry attempts (minimum 1)
- `baseDelay` (`time.Duration`) - Initial delay between retries

## Returns

Returns a `*Backoff[T]` that implements `Chainable[T]`.

## Behavior

- **Exponential growth**: Each retry doubles the previous delay
- **No delay after final attempt**: Fails immediately if last attempt fails
- **Context aware**: Respects cancellation and deadlines
- **Error preservation**: Maintains complete error context from failures
- **Thread-safe**: Safe for concurrent use

### Delay Progression

Given a base delay of 1 second:
- 1st retry: 1 second delay
- 2nd retry: 2 seconds delay  
- 3rd retry: 4 seconds delay
- 4th retry: 8 seconds delay
- And so on...

## Methods

### SetMaxAttempts

Updates the maximum number of retry attempts.

```go
func (b *Backoff[T]) SetMaxAttempts(n int) *Backoff[T]
```

### SetBaseDelay

Updates the base delay duration.

```go
func (b *Backoff[T]) SetBaseDelay(d time.Duration) *Backoff[T]
```

### GetMaxAttempts

Returns the current maximum attempts setting.

```go
func (b *Backoff[T]) GetMaxAttempts() int
```

### GetBaseDelay

Returns the current base delay setting.

```go
func (b *Backoff[T]) GetBaseDelay() time.Duration
```

### Name

Returns the name of this connector.

```go
func (b *Backoff[T]) Name() Name
```

## Basic Usage

```go
// Retry API calls with exponential backoff
apiCall := pipz.NewBackoff("api-retry",
    pipz.Apply("call-api", func(ctx context.Context, req Request) (Response, error) {
        return externalAPI.Call(ctx, req)
    }),
    5,                    // Max 5 attempts
    100*time.Millisecond, // Start with 100ms delay
)

// Delays will be: 100ms, 200ms, 400ms, 800ms
```

## Common Patterns

### Network Request Handling

```go
// Robust HTTP client with backoff
httpClient := pipz.NewBackoff("http-backoff",
    pipz.Apply("http-request", func(ctx context.Context, req HTTPRequest) (HTTPResponse, error) {
        resp, err := client.Do(req.ToHTTP())
        if err != nil {
            return HTTPResponse{}, err
        }
        
        // Retry on 5xx errors
        if resp.StatusCode >= 500 {
            return HTTPResponse{}, fmt.Errorf("server error: %d", resp.StatusCode)
        }
        
        return parseResponse(resp)
    }),
    4,                   // 4 attempts total
    500*time.Millisecond, // Start with 500ms
)
// Total possible delay: 500ms + 1s + 2s = 3.5s
```

### Database Operations

```go
// Database operations with backoff for lock contention
dbOperation := pipz.NewBackoff("db-retry",
    pipz.Apply("update", func(ctx context.Context, data Record) (Record, error) {
        tx, err := db.BeginTx(ctx, nil)
        if err != nil {
            return data, err
        }
        defer tx.Rollback()
        
        // Perform operations
        if err := updateRecord(tx, data); err != nil {
            return data, err
        }
        
        return data, tx.Commit()
    }),
    3,                  // 3 attempts for deadlocks
    50*time.Millisecond, // Short initial delay
)
```

### Message Queue Processing

```go
// Retry message processing with increasing delays
messageProcessor := pipz.NewBackoff("message-retry",
    pipz.NewSequence("process-message",
        parseMessage,
        validateMessage,
        pipz.Apply("send", func(ctx context.Context, msg Message) (Message, error) {
            return queue.Send(ctx, msg)
        }),
    ),
    6,              // More attempts for async operations
    1*time.Second,  // Start with 1 second
)
// Max total delay: 1s + 2s + 4s + 8s + 16s = 31s
```

### Combined with Circuit Breaker

```go
// Backoff with circuit breaker for external services
resilientService := pipz.NewCircuitBreaker("circuit",
    pipz.NewBackoff("backoff",
        externalServiceCall,
        3,                    // Limited attempts before circuit opens
        200*time.Millisecond,
    ),
    10,  // Failure threshold
    5*time.Minute, // Recovery timeout
)
```

## Configuration Patterns

### Dynamic Configuration

```go
backoff := pipz.NewBackoff("dynamic", processor, 3, 1*time.Second)

// Adjust based on load
if highLoad() {
    backoff.SetMaxAttempts(5).SetBaseDelay(2*time.Second)
} else {
    backoff.SetMaxAttempts(3).SetBaseDelay(500*time.Millisecond)
}
```

### Environment-Based Configuration

```go
func createBackoff(env string) *Backoff[Data] {
    switch env {
    case "production":
        return pipz.NewBackoff("prod-backoff", processor, 5, 1*time.Second)
    case "staging":
        return pipz.NewBackoff("stage-backoff", processor, 3, 500*time.Millisecond)
    default:
        return pipz.NewBackoff("dev-backoff", processor, 2, 100*time.Millisecond)
    }
}
```

## Error Handling

Backoff preserves complete error context:

```go
result, err := backoff.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) {
        // Error path includes backoff name
        fmt.Printf("Failed after %d attempts at: %v\n", 
            backoff.GetMaxAttempts(), pipeErr.Path)
        
        // Check if it was a timeout during backoff
        if pipeErr.IsTimeout() {
            log.Warn("Backoff interrupted by timeout")
        }
    }
}
```

## Comparison with Retry

### Backoff vs Retry

| Feature | Backoff | Retry |
|---------|---------|-------|
| Delay pattern | Exponential (1s, 2s, 4s...) | Fixed (1s, 1s, 1s...) |
| Use case | Transient failures, overload | Quick failures, network blips |
| Total time | Grows exponentially | Grows linearly |
| System load | Reduces pressure over time | Constant pressure |

```go
// Backoff: Good for overloaded systems
backoff := pipz.NewBackoff("exponential", processor, 4, 1*time.Second)
// Delays: 1s, 2s, 4s (total: 7s)

// Retry: Good for brief interruptions  
retry := pipz.NewRetry("fixed", processor, 4, 1*time.Second)
// Delays: 1s, 1s, 1s (total: 3s)
```

## Performance Characteristics

- **Memory**: O(1) - No additional allocations per retry
- **Goroutines**: No additional goroutines created
- **Overhead**: ~10ns + sleep time per retry
- **Context checks**: Performed before each retry

## Gotchas

### ❌ Don't use tiny base delays

```go
// WRONG - Delays too small to be meaningful
backoff := pipz.NewBackoff("too-fast", processor, 5, 1*time.Microsecond)
// Results in: 1μs, 2μs, 4μs, 8μs - essentially no delay
```

### ✅ Use meaningful base delays

```go
// RIGHT - Delays that allow recovery
backoff := pipz.NewBackoff("reasonable", processor, 5, 100*time.Millisecond)
// Results in: 100ms, 200ms, 400ms, 800ms - gives system time to recover
```

### ❌ Don't use too many attempts

```go
// WRONG - Could wait extremely long
backoff := pipz.NewBackoff("excessive", processor, 10, 1*time.Second)
// Final delay would be 512 seconds (8.5 minutes)!
```

### ✅ Balance attempts with total delay

```go
// RIGHT - Reasonable total delay
backoff := pipz.NewBackoff("balanced", processor, 5, 500*time.Millisecond)
// Max total: 500ms + 1s + 2s + 4s = 7.5s
```

### ❌ Don't ignore context cancellation

```go
// WRONG - Not checking context
for i := 0; i < maxAttempts; i++ {
    result, err := process(data)
    if err == nil {
        return result, nil
    }
    time.Sleep(delay) // Ignores context!
    delay *= 2
}
```

### ✅ Respect context cancellation

```go
// RIGHT - Backoff handles this automatically
backoff := pipz.NewBackoff("context-aware", processor, 5, 1*time.Second)
// Automatically stops on context cancellation
```

## Best Practices

1. **Start with small delays** - Begin with 50-500ms for most operations
2. **Limit max attempts** - Usually 3-5 attempts is sufficient
3. **Calculate total time** - Ensure max total delay is acceptable
4. **Match delay to failure type** - Network: 100ms+, Database: 50ms+, API: 500ms+
5. **Monitor retry metrics** - Track retry rates and success rates
6. **Consider circuit breakers** - Combine with circuit breakers for system protection
7. **Log retry attempts** - Include attempt number in logs for debugging

## Testing

```go
func TestBackoff(t *testing.T) {
    attempts := 0
    processor := pipz.Apply("flaky", func(ctx context.Context, n int) (int, error) {
        attempts++
        if attempts < 3 {
            return 0, errors.New("transient error")
        }
        return n * 2, nil
    })
    
    backoff := pipz.NewBackoff("test", processor, 5, 10*time.Millisecond)
    
    start := time.Now()
    result, err := backoff.Process(context.Background(), 5)
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Equal(t, 10, result)
    assert.Equal(t, 3, attempts)
    
    // Verify exponential delays (10ms + 20ms = 30ms minimum)
    assert.GreaterOrEqual(t, duration, 30*time.Millisecond)
}

func TestBackoffTimeout(t *testing.T) {
    processor := pipz.Apply("slow", func(ctx context.Context, n int) (int, error) {
        return 0, errors.New("always fails")
    })
    
    backoff := pipz.NewBackoff("timeout-test", processor, 10, 100*time.Millisecond)
    
    ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
    defer cancel()
    
    _, err := backoff.Process(ctx, 5)
    
    var pipeErr *pipz.Error[int]
    require.Error(t, err)
    require.True(t, errors.As(err, &pipeErr))
    assert.True(t, pipeErr.IsTimeout())
}
```

## See Also

- [Retry](./retry.md) - Fixed-delay retry pattern
- [CircuitBreaker](./circuitbreaker.md) - Prevent cascading failures
- [Timeout](./timeout.md) - Bound operation time
- [Fallback](./fallback.md) - Alternative processing on failure