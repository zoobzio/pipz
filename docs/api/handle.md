# Handle

Processes errors through their own pipeline, enabling sophisticated error recovery flows.

## Function Signature

```go
func NewHandle[T any](
    name Name,
    processor Chainable[T],
    errorHandler Chainable[*Error[T]],
) *Handle[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `processor` - Main processor that might fail
- `errorHandler` - Pipeline that processes errors (receives `*Error[T]`)

## Returns

Returns a `*Handle[T]` that implements `Chainable[T]`.

## Behavior

- **Error interception** - Catches errors from the main processor
- **Error as data** - Errors flow through the error handler pipeline
- **Original error returned** - Still returns the original error
- **Success pass-through** - Successful results bypass error handler
- **Handler errors ignored** - Error handler failures don't affect outcome

## Key Insight

Handle treats errors as data that can flow through pipelines. This enables using all pipz features (Switch, Concurrent, Retry, etc.) for error handling.

## Example

```go
// Simple error logging
logged := pipz.NewHandle("logged-operation",
    riskyOperation,
    pipz.Effect("log-error", func(ctx context.Context, err *pipz.Error[Order]) error {
        log.Printf("Order %s failed at %v: %v", 
            err.InputData.ID, err.Path, err.Err)
        return nil
    }),
)

// Complex error recovery
recovery := pipz.NewHandle("with-recovery",
    mainPipeline,
    pipz.NewSequence[*pipz.Error[User]]("error-pipeline",
        pipz.Effect("log", logError),
        pipz.Apply("categorize", categorizeError),
        pipz.NewSwitch("error-router", routeByErrorType).
            AddRoute("timeout", handleTimeout).
            AddRoute("validation", handleValidation).
            AddRoute("system", handleSystemError),
        pipz.NewConcurrent("notify",
            pipz.Effect("alert-ops", alertOperations),
            pipz.Effect("update-metrics", updateErrorMetrics),
        ),
    ),
)

// Retry based on error type
smartRetry := pipz.NewHandle("smart-retry",
    processor,
    pipz.Apply("check-retry", func(ctx context.Context, err *pipz.Error[Data]) (*pipz.Error[Data], error) {
        if err.Timeout || isTransient(err.Err) {
            // Retry transient errors
            result, retryErr := processor.Process(ctx, err.InputData)
            if retryErr == nil {
                // Success on retry!
                metrics.Increment("retry.success")
                return err, nil // Original error still returned
            }
        }
        return err, nil
    }),
)
```

## When to Use

Use `Handle` when:
- You need sophisticated error handling
- Errors require different handling based on type
- You want to log/monitor all errors
- Error recovery is complex
- You need error-driven workflows

## When NOT to Use

Don't use `Handle` when:
- Simple retry is sufficient (use `Retry`)
- You just need a fallback (use `Fallback`)
- Errors should stop processing immediately
- No error handling is needed

## Error Handler Access

The error handler receives `*Error[T]` with full context:

```go
type Error[T any] struct {
    Path      []Name        // Full path through processors
    Err       error         // Original error
    InputData T             // Input when error occurred
    Timeout   bool          // Was this a timeout?
    Canceled  bool          // Was this cancelled?
    Timestamp time.Time     // When the error occurred
    Duration  time.Duration // Processing time before error
}
```

## Common Patterns

```go
// Error monitoring and alerting
monitoring := pipz.NewHandle("monitored",
    businessLogic,
    pipz.NewSequence[*pipz.Error[Order]]("monitor",
        pipz.Effect("metrics", func(ctx context.Context, err *pipz.Error[Order]) error {
            metrics.Increment("errors",
                "path", strings.Join(err.Path, "."),
                "timeout", fmt.Sprint(err.Timeout),
            )
            return nil
        }),
        pipz.Mutate("alert-critical",
            func(ctx context.Context, err *pipz.Error[Order]) bool {
                return err.InputData.Priority == "critical"
            },
            func(ctx context.Context, err *pipz.Error[Order]) *pipz.Error[Order] {
                alerting.SendPage("Critical order failed", err)
                return err
            },
        ),
    ),
)

// Error recovery with fallback data
gracefulDegradation := pipz.NewHandle("degraded",
    pipz.Apply("fetch-live", fetchLiveData),
    pipz.Apply("use-cache", func(ctx context.Context, err *pipz.Error[Request]) (*pipz.Error[Request], error) {
        // Try to serve from cache on error
        cached, cacheErr := cache.Get(ctx, err.InputData.Key)
        if cacheErr == nil {
            log.Printf("Serving stale data due to error: %v", err.Err)
            metrics.Increment("cache.fallback")
            // Note: We can't change the pipeline result from here
            // This is just for side effects
        }
        return err, nil
    }),
)

// Circuit breaker pattern
circuitBreaker := pipz.NewHandle("circuit",
    service,
    pipz.Effect("track-failure", func(ctx context.Context, err *pipz.Error[Request]) error {
        breaker.RecordFailure()
        if breaker.ShouldOpen() {
            log.Printf("Circuit breaker opened after error: %v", err.Err)
        }
        return nil
    }),
)
```

## Advanced Error Flows

```go
// Parallel error handling
parallelRecovery := pipz.NewHandle("parallel-recovery",
    mainProcess,
    pipz.NewConcurrent[*pipz.Error[Data]](
        pipz.Effect("log", logToCentralSystem),
        pipz.Effect("metrics", updateDashboard),
        pipz.Effect("backup", saveFailedRequest),
        pipz.Apply("notify", notifyOnCallTeam),
    ),
)

// Nested error handling
nestedHandling := pipz.NewHandle("outer",
    pipz.NewHandle("inner",
        riskyOperation,
        pipz.Effect("inner-log", logInnerError),
    ),
    pipz.Effect("outer-log", func(ctx context.Context, err *pipz.Error[Data]) error {
        // This catches errors from both riskyOperation and inner-log
        log.Printf("Outer handler: %v", err)
        return nil
    }),
)

// Error aggregation for batch processing
batchErrors := pipz.NewHandle("batch",
    batchProcessor,
    pipz.Apply("collect", func(ctx context.Context, err *pipz.Error[Batch]) (*pipz.Error[Batch], error) {
        errorCollector.Add(err)
        if errorCollector.Count() > errorThreshold {
            // Trigger batch error recovery
            triggerBatchRecovery(errorCollector.GetAll())
        }
        return err, nil
    }),
)
```

## Best Practices

```go
// Keep error handlers focused
// GOOD: Specific error handling
goodHandle := pipz.NewHandle("specific",
    processor,
    pipz.Effect("log-timeout", func(ctx context.Context, err *pipz.Error[Data]) error {
        if err.Timeout {
            timeoutLogger.Error("Operation timed out", err)
        }
        return nil
    }),
)

// BAD: Trying to modify pipeline flow from error handler
badHandle := pipz.NewHandle("bad",
    processor,
    pipz.Apply("modify", func(ctx context.Context, err *pipz.Error[Data]) (*pipz.Error[Data], error) {
        // This CANNOT change the main pipeline's result!
        // Handle is for side effects only
        return err, nil
    }),
)

// Use Handle for observability
observable := pipz.NewHandle("observable",
    complexPipeline,
    pipz.NewSequence[*pipz.Error[Event]]("observe",
        pipz.Effect("trace", addToDistributedTrace),
        pipz.Effect("log", structuredLogging),
        pipz.Effect("metric", recordErrorMetrics),
        pipz.Effect("alert", checkAlertingRules),
    ),
)
```

## See Also

- [Fallback](./fallback.md) - For simple primary/backup patterns
- [Retry](./retry.md) - For retry logic
- [Switch](./switch.md) - Often used within error handlers
- [Concurrent](./concurrent.md) - For parallel error handling