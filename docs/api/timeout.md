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
- Calling external services
- Operations might hang
- SLAs must be enforced
- Protecting against slow operations
- Resource usage needs bounds

## When NOT to Use

Don't use `Timeout` when:
- Operations are always fast
- Cancellation might corrupt data
- Exact completion is required
- Timeout would leave inconsistent state

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
- [Handle](./handle.md) - For timeout monitoring