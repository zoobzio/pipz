---
title: "Timeout"
description: "Enforces a time limit on processor execution to prevent indefinite operations"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - timeout
  - time-bound
  - sla
---

# Timeout

Enforces a time limit on processor execution.

## Function Signature

```go
func NewTimeout[T any](
    identity Identity,
    processor Chainable[T],
    duration time.Duration,
) *Timeout[T]
```

## Parameters

- `identity` (`Identity`) - Identifier with name and description for the connector used in debugging and observability
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
timeout := pipz.NewTimeout(
    pipz.NewIdentity("test", "Test timeout with fake clock for controlled time advancement"),
    processor,
    5*time.Second,
).WithClock(fakeClock)

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
// Define identities upfront
var (
    APITimeoutID     = pipz.NewIdentity("api-timeout", "Enforces 5 second timeout on external API calls")
    APICallID        = pipz.NewIdentity("api-call", "Calls external API")
    BoundedPipelineID = pipz.NewIdentity("bounded-pipeline", "Ensures entire data processing pipeline completes within 30 seconds")
    ProcessingID     = pipz.NewIdentity("processing", "Data processing sequence")
    FetchID          = pipz.NewIdentity("fetch", "Fetches data")
    TransformID      = pipz.NewIdentity("transform", "Transforms data")
    SaveID           = pipz.NewIdentity("save", "Saves data")
    GraduatedID      = pipz.NewIdentity("graduated", "Graduated timeout fallback")
    FastID           = pipz.NewIdentity("fast", "Primary service with strict 1 second timeout")
    SlowID           = pipz.NewIdentity("slow", "Backup service with generous 10 second timeout")
    ReliableID       = pipz.NewIdentity("reliable", "Reliable retry with timeout")
    BoundedOpID      = pipz.NewIdentity("bounded-op", "Each retry attempt limited to 5 seconds for slow operation")
)

// Basic timeout
fastAPI := pipz.NewTimeout(APITimeoutID,
    pipz.Apply(APICallID, callExternalAPI),
    5*time.Second,
)

// Timeout on complex pipeline
timeBounded := pipz.NewTimeout(BoundedPipelineID,
    pipz.NewSequence[Data](ProcessingID,
        pipz.Apply(FetchID, fetchData),
        pipz.Apply(TransformID, transformData),
        pipz.Apply(SaveID, saveData),
    ),
    30*time.Second, // Entire pipeline must complete in 30s
)

// Graduated timeouts
resilient := pipz.NewFallback(GraduatedID,
    pipz.NewTimeout(FastID,
        primaryService,
        1*time.Second,
    ),
    pipz.NewTimeout(SlowID,
        backupService,
        10*time.Second,
    ),
)

// Timeout with retry
reliableButSlow := pipz.NewRetry(ReliableID,
    pipz.NewTimeout(BoundedOpID,
        slowOperation,
        5*time.Second,
    ),
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

## Error Details

Timeout errors are marked specially:

```go
timeout := pipz.NewTimeout(
    pipz.NewIdentity("slow-op", "Timeout for potentially slow processor with 2 second limit"),
    slowProcessor,
    2*time.Second,
)
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
// Define identities upfront
var (
    GoodProcessorID = pipz.NewIdentity("good", "Context-aware processor")
    BadProcessorID  = pipz.NewIdentity("bad", "Context-ignoring processor")
)

// GOOD: Checks context
goodProcessor := pipz.Apply(GoodProcessorID, func(ctx context.Context, data Data) (Data, error) {
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
badProcessor := pipz.Apply(BadProcessorID, func(ctx context.Context, data Data) (Data, error) {
    // This will continue even after timeout!
    time.Sleep(10 * time.Second)
    return data, nil
})
```

## Common Patterns

```go
// Tiered timeout strategy
tieredService := pipz.NewSwitch[Request](
    pipz.NewIdentity("tier-router", "Routes requests by tier with appropriate timeout limits"),
    func(ctx context.Context, req Request) string {
        return req.Tier
    },
).
AddRoute("premium", pipz.NewTimeout(
    pipz.NewIdentity("premium", "Premium tier processing with 10 second timeout"),
    processor,
    10*time.Second,
)).
AddRoute("standard", pipz.NewTimeout(
    pipz.NewIdentity("standard", "Standard tier processing with 5 second timeout"),
    processor,
    5*time.Second,
)).
AddRoute("free", pipz.NewTimeout(
    pipz.NewIdentity("free", "Free tier processing with 1 second timeout"),
    processor,
    1*time.Second,
))

// Define identities for monitoring and dynamic patterns
var (
    MonitoredID  = pipz.NewIdentity("monitored", "Monitored timeout handler")
    OperationID  = pipz.NewIdentity("operation", "Monitored operation with 3 second timeout")
    TrackID      = pipz.NewIdentity("track", "Tracks timeout metrics")
    DynamicID    = pipz.NewIdentity("dynamic", "Dynamic timeout based on priority")
    BoundedID    = pipz.NewIdentity("bounded", "Dynamically timed operation")
)

// Timeout with monitoring
monitoredTimeout := pipz.NewHandle(MonitoredID,
    pipz.NewTimeout(OperationID,
        processor,
        3*time.Second,
    ),
    pipz.Effect(TrackID, func(ctx context.Context, err *pipz.Error[Data]) error {
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
dynamicTimeout := pipz.Apply(DynamicID, func(ctx context.Context, req Request) (Request, error) {
    timeout := 5 * time.Second // default
    if req.Priority == "high" {
        timeout = 30 * time.Second
    }

    return pipz.NewTimeout(BoundedID,
        processor,
        timeout,
    ).Process(ctx, req)
})
```

## Advanced Patterns

```go
// Percentage-based timeout (P95)
adaptiveTimeout := pipz.NewTimeout(
    pipz.NewIdentity("adaptive", "Adaptive timeout based on P95 historical performance metrics"),
    processor,
    calculateP95Timeout(), // Based on historical data
)

// Define identities for advanced patterns
var (
    GracefulID     = pipz.NewIdentity("graceful", "Graceful shutdown with partial results")
    OuterID        = pipz.NewIdentity("outer", "Outer timeout for multi-step sequence")
    InnerSequenceID = pipz.NewIdentity("inner-sequence", "Inner sequence with step timeouts")
    Step1ID        = pipz.NewIdentity("step1", "First step with 5 second timeout")
    Step2ID        = pipz.NewIdentity("step2", "Second step with 5 second timeout")
    Step3ID        = pipz.NewIdentity("step3", "Third step with 5 second timeout")
)

// Timeout with graceful shutdown
gracefulTimeout := pipz.Apply(GracefulID, func(ctx context.Context, batch Batch) (Batch, error) {
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
cascading := pipz.NewTimeout(OuterID,
    pipz.NewSequence[Data](InnerSequenceID,
        pipz.NewTimeout(Step1ID,
            step1,
            5*time.Second,
        ),
        pipz.NewTimeout(Step2ID,
            step2,
            5*time.Second,
        ),
        pipz.NewTimeout(Step3ID,
            step3,
            5*time.Second,
        ),
    ),
    12*time.Second, // Total timeout less than sum of parts
)
```

## Gotchas

### ❌ Don't use timeouts that are too short
```go
// WRONG - Timeout shorter than average response time
timeout := pipz.NewTimeout(
    pipz.NewIdentity("too-short", "Unrealistic timeout that will fail most normal requests"),
    apiCall, // Takes 2-3 seconds normally
    1*time.Second, // Will almost always timeout!
)
```

### ✅ Base timeouts on actual performance
```go
// RIGHT - Realistic timeout
timeout := pipz.NewTimeout(
    pipz.NewIdentity("realistic", "Realistic timeout based on P99 response time measurements"),
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
timeout := pipz.NewTimeout(
    pipz.NewIdentity("outer", "Outer timeout shorter than inner timeout - misconfigured"),
    pipz.NewTimeout(
        pipz.NewIdentity("inner", "Inner timeout that will never complete fully"),
        processor,
        10*time.Second,
    ), // Longer!
    5*time.Second, // Shorter - inner never gets full time
)
```

### ✅ Make outer timeouts longer than sum of inner
```go
// Define identities upfront
var (
    OuterTimeoutID = pipz.NewIdentity("outer", "Outer timeout with buffer")
    StepsID        = pipz.NewIdentity("steps", "Sequential steps")
    StepOneID      = pipz.NewIdentity("step1", "First step with 5 second timeout")
    StepTwoID      = pipz.NewIdentity("step2", "Second step with 5 second timeout")
)

// RIGHT - Outer accommodates inner timeouts
timeout := pipz.NewTimeout(OuterTimeoutID,
    pipz.NewSequence(StepsID,
        pipz.NewTimeout(StepOneID,
            step1,
            5*time.Second,
        ),
        pipz.NewTimeout(StepTwoID,
            step2,
            5*time.Second,
        ),
    ),
    12*time.Second, // Allows both to complete with buffer
)
```

## Best Practices

```go
// Set realistic timeouts
// GOOD: Based on actual performance
goodTimeout := pipz.NewTimeout(
    pipz.NewIdentity("realistic", "Statistically-based timeout using mean plus standard deviation"),
    apiCall,
    2*averageResponseTime + standardDeviation,
)

// BAD: Arbitrary timeout
badTimeout := pipz.NewTimeout(
    pipz.NewIdentity("arbitrary", "Arbitrary timeout without performance justification"),
    apiCall,
    1*time.Minute, // Why 1 minute?
)

// Define identities for best practices examples
var (
    BatchID     = pipz.NewIdentity("batch", "Batch processor with timeout")
    ProcessID   = pipz.NewIdentity("process", "Processes batch items")
    SlowLogID   = pipz.NewIdentity("slow-log", "Logs slow operations")
    MaybeSlowID = pipz.NewIdentity("maybe-slow", "Potentially slow operation")
    LogSlowID   = pipz.NewIdentity("log-slow", "Logs slow operations")
)

// Handle partial results
batchWithTimeout := pipz.NewTimeout(BatchID,
    pipz.Apply(ProcessID, func(ctx context.Context, batch Batch) (Batch, error) {
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
slowLogger := pipz.NewHandle(SlowLogID,
    pipz.NewTimeout(MaybeSlowID,
        processor,
        5*time.Second,
    ),
    pipz.Effect(LogSlowID, func(ctx context.Context, err *pipz.Error[Data]) error {
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
- [Handle](../3.processors/handle.md) - For timeout monitoring