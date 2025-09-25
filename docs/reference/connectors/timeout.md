# Timeout

Enforces a time limit on processor execution.

## Function Signature

```go
func NewTimeout[T any](
    name Name,
    processor Chainable[T],
    duration time.Duration,
) *Timeout[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `processor` - The processor to time-bound
- `duration` - Maximum allowed execution time

## Returns

Returns a `*Timeout[T]` that implements `Chainable[T]`.

## Testing Configuration

### WithClock

```go
func (t *Timeout[T]) WithClock(clock clockz.Clock) *Timeout[T]
```

Sets a custom clock implementation for testing purposes. This method enables controlled time manipulation in tests using `clockz.FakeClock`.

**Parameters:**
- `clock` (`clockz.Clock`) - Clock implementation to use

**Returns:**
Returns the same connector instance for method chaining.

**Example:**
```go
// Use fake clock in tests
fakeClock := clockz.NewFakeClock()
timeout := pipz.NewTimeout("test", processor, 5*time.Second).
    WithClock(fakeClock)

// Advance time in test
fakeClock.Advance(6 * time.Second)
```

## Behavior

- **Time enforcement** - Cancels operation after duration
- **Context timeout** - Creates a timeout context for the processor
- **Timeout errors** - Returns timeout error with timing information
- **Clean cancellation** - Processor should respect context cancellation
- **Error enrichment** - Timeout errors include duration info

## Example

```go
// Basic timeout
fastAPI := pipz.NewTimeout("api-timeout",
    pipz.Apply("api-call", callExternalAPI),
    5*time.Second,
)

// Timeout on complex pipeline
timeBounded := pipz.NewTimeout("bounded-pipeline",
    pipz.NewSequence[Data]("processing",
        pipz.Apply("fetch", fetchData),
        pipz.Apply("transform", transformData),
        pipz.Apply("save", saveData),
    ),
    30*time.Second, // Entire pipeline must complete in 30s
)

// Graduated timeouts
resilient := pipz.NewFallback("graduated",
    pipz.NewTimeout("fast", primaryService, 1*time.Second),
    pipz.NewTimeout("slow", backupService, 10*time.Second),
)

// Timeout with retry
reliableButSlow := pipz.NewRetry("reliable",
    pipz.NewTimeout("bounded-op", slowOperation, 5*time.Second),
    3, // Retry up to 3 times, each with 5s timeout
)
```

## When to Use

Use `Timeout` when:
- **Calling external services** (APIs, databases)
- Operations might hang indefinitely
- SLAs must be enforced
- Protecting against slow operations
- Resource usage needs bounds
- User-facing operations need responsiveness
- Preventing cascading delays

## When NOT to Use

Don't use `Timeout` when:
- Operations are always fast (unnecessary overhead)
- Cancellation might corrupt data (transactions)
- Exact completion is required (financial processing)
- Timeout would leave inconsistent state
- Operations can't be cancelled (use monitoring instead)

## Observability

Timeout provides comprehensive observability through metrics, tracing, and hook events.

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `timeout.processed.total` | Counter | Total timeout operations |
| `timeout.successes.total` | Counter | Operations that completed in time |
| `timeout.timeouts.total` | Counter | Operations that exceeded timeout |
| `timeout.cancellations.total` | Counter | Operations canceled by upstream |
| `timeout.duration.ms` | Gauge | Actual operation duration |

### Traces

| Span | Description |
|------|-------------|
| `timeout.process` | Span for timeout-wrapped operation |

**Span Tags:**
- `timeout.duration` - Configured timeout duration
- `timeout.elapsed` - Actual time taken
- `timeout.success` - Whether operation completed in time
- `timeout.timed_out` - Whether timeout was exceeded
- `timeout.canceled` - Whether canceled by upstream

### Hook Events

| Event | Key | Description |
|-------|-----|-------------|
| Timeout | `timeout.timeout` | Fired when operation times out |
| Near Timeout | `timeout.near_timeout` | Fired when >80% of timeout used |

### Event Handlers

```go
// Alert on timeouts
timeout.OnTimeout(func(ctx context.Context, event TimeoutEvent) error {
    log.Error("Operation timed out after %v", event.Elapsed)
    alert.Critical("Timeout in %s", event.Name)
    metrics.Inc("timeouts", event.Name)
    return nil
})

// Warn on slow operations
timeout.OnNearTimeout(func(ctx context.Context, event TimeoutEvent) error {
    log.Warn("Operation used %.0f%% of timeout (%v/%v)",
        event.PercentUsed, event.Elapsed, event.Duration)
    if event.PercentUsed > 90 {
        alert.Warning("Operation %s critically slow", event.Name)
    }
    return nil
})
```

## Error Details

Timeout errors are marked specially:

```go
timeout := pipz.NewTimeout("slow-op", slowProcessor, 2*time.Second)
_, err := timeout.Process(ctx, data)

if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) {
        if pipeErr.Timeout {
            fmt.Printf("Operation timed out after %v", pipeErr.Duration)
            // Duration will be ~2 seconds
        }
    }
}
```

## Context Cancellation

Processors must respect context cancellation:

```go
// GOOD: Checks context
goodProcessor := pipz.Apply("good", func(ctx context.Context, data Data) (Data, error) {
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            return data, ctx.Err() // Respects timeout
        default:
            // Do work
            if err := processChunk(data, i); err != nil {
                return data, err
            }
        }
    }
    return data, nil
})

// BAD: Ignores context
badProcessor := pipz.Apply("bad", func(ctx context.Context, data Data) (Data, error) {
    // This will continue even after timeout!
    time.Sleep(10 * time.Second)
    return data, nil
})
```

## Common Patterns

```go
// Tiered timeout strategy
tieredService := pipz.NewSwitch[Request, string]("tier-router",
    func(ctx context.Context, req Request) string {
        return req.Tier
    },
).
AddRoute("premium", pipz.NewTimeout("premium", processor, 10*time.Second)).
AddRoute("standard", pipz.NewTimeout("standard", processor, 5*time.Second)).
AddRoute("free", pipz.NewTimeout("free", processor, 1*time.Second))

// Timeout with monitoring
monitoredTimeout := pipz.NewHandle("monitored",
    pipz.NewTimeout("operation", processor, 3*time.Second),
    pipz.Effect("track", func(ctx context.Context, err *pipz.Error[Data]) error {
        if err.Timeout {
            metrics.Increment("timeouts", "operation", "myop")
            if err.Duration > 2*time.Second {
                log.Printf("Near timeout: %v", err.Duration)
            }
        }
        return nil
    }),
)

// Dynamic timeout based on context
dynamicTimeout := pipz.Apply("dynamic", func(ctx context.Context, req Request) (Request, error) {
    timeout := 5 * time.Second // default
    if req.Priority == "high" {
        timeout = 30 * time.Second
    }
    
    return pipz.NewTimeout("bounded", processor, timeout).Process(ctx, req)
})
```

## Advanced Patterns

```go
// Percentage-based timeout (P95)
adaptiveTimeout := pipz.NewTimeout("adaptive",
    processor,
    calculateP95Timeout(), // Based on historical data
)

// Timeout with graceful shutdown
gracefulTimeout := pipz.Apply("graceful", func(ctx context.Context, batch Batch) (Batch, error) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    results := make([]Result, 0, len(batch.Items))
    for _, item := range batch.Items {
        select {
        case <-ctx.Done():
            // Save partial results
            batch.Partial = true
            batch.Results = results
            return batch, fmt.Errorf("timeout after %d items", len(results))
        default:
            result, err := processItem(ctx, item)
            if err != nil {
                return batch, err
            }
            results = append(results, result)
        }
    }
    
    batch.Results = results
    return batch, nil
})

// Cascading timeouts
cascading := pipz.NewTimeout("outer",
    pipz.NewSequence[Data]("inner-sequence",
        pipz.NewTimeout("step1", step1, 5*time.Second),
        pipz.NewTimeout("step2", step2, 5*time.Second),
        pipz.NewTimeout("step3", step3, 5*time.Second),
    ),
    12*time.Second, // Total timeout less than sum of parts
)
```

## Gotchas

### ❌ Don't use timeouts that are too short
```go
// WRONG - Timeout shorter than average response time
timeout := pipz.NewTimeout("too-short",
    apiCall, // Takes 2-3 seconds normally
    1*time.Second, // Will almost always timeout!
)
```

### ✅ Base timeouts on actual performance
```go
// RIGHT - Realistic timeout
timeout := pipz.NewTimeout("realistic",
    apiCall,
    5*time.Second, // P99 response time
)
```

### ❌ Don't ignore context in processors
```go
// WRONG - Ignores timeout!
processor := pipz.Apply("bad", func(ctx context.Context, data Data) (Data, error) {
    time.Sleep(10 * time.Second) // Ignores context!
    return data, nil
})
```

### ✅ Check context during long operations
```go
// RIGHT - Respects timeout
processor := pipz.Apply("good", func(ctx context.Context, data Data) (Data, error) {
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            return data, ctx.Err()
        default:
            processChunk(data, i)
        }
    }
    return data, nil
})
```

### ❌ Don't nest timeouts incorrectly
```go
// WRONG - Inner timeout longer than outer!
timeout := pipz.NewTimeout("outer",
    pipz.NewTimeout("inner", processor, 10*time.Second), // Longer!
    5*time.Second, // Shorter - inner never gets full time
)
```

### ✅ Make outer timeouts longer than sum of inner
```go
// RIGHT - Outer accommodates inner timeouts
timeout := pipz.NewTimeout("outer",
    pipz.NewSequence("steps",
        pipz.NewTimeout("step1", step1, 5*time.Second),
        pipz.NewTimeout("step2", step2, 5*time.Second),
    ),
    12*time.Second, // Allows both to complete with buffer
)
```

## Best Practices

```go
// Set realistic timeouts
// GOOD: Based on actual performance
goodTimeout := pipz.NewTimeout("realistic",
    apiCall,
    2*averageResponseTime + standardDeviation,
)

// BAD: Arbitrary timeout
badTimeout := pipz.NewTimeout("arbitrary",
    apiCall,
    1*time.Minute, // Why 1 minute?
)

// Handle partial results
batchWithTimeout := pipz.NewTimeout("batch",
    pipz.Apply("process", func(ctx context.Context, batch Batch) (Batch, error) {
        var processed []Item
        for _, item := range batch.Items {
            if ctx.Err() != nil {
                // Return partial results
                return Batch{
                    Items:     batch.Items,
                    Processed: processed,
                    Partial:   true,
                }, ctx.Err()
            }
            processed = append(processed, process(item))
        }
        return Batch{Items: batch.Items, Processed: processed}, nil
    }),
    30*time.Second,
)

// Log slow operations
slowLogger := pipz.NewHandle("slow-log",
    pipz.NewTimeout("maybe-slow", processor, 5*time.Second),
    pipz.Effect("log-slow", func(ctx context.Context, err *pipz.Error[Data]) error {
        // Log operations that take more than 3s
        if err.Duration > 3*time.Second {
            log.Printf("Slow operation: %v (timeout: %v)", err.Duration, err.Timeout)
        }
        return nil
    }),
)
```

## See Also

- [Retry](./retry.md) - Often combined with timeout
- [Fallback](./fallback.md) - For timeout recovery
- [Race](./race.md) - Alternative approach to timeouts
- [Handle](../processors/handle.md) - For timeout monitoring