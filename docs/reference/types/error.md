# Error[T]

Rich error context for pipeline failures with complete debugging information.

## Overview

`Error[T]` provides comprehensive error information when pipeline processing fails. It captures not just what went wrong, but where, when, and with what data - giving you everything needed to debug production issues.

## Type Definition

```go
type Error[T any] struct {
    Timestamp time.Time  // When the error occurred
    InputData T          // The data that caused the failure  
    Err       error      // The underlying error
    Path      []Name     // Complete processing path to failure point
    Duration  time.Duration // How long before failure
    Timeout   bool       // Whether it was a timeout
    Canceled  bool       // Whether it was canceled
}
```

## Type Parameters

- `T` - The type of data being processed when the error occurred

## Fields

### Timestamp
- **Type**: `time.Time`
- **Purpose**: Records the exact time when the error occurred
- **Usage**: Correlate with logs, metrics, and monitoring systems

### InputData
- **Type**: `T` (generic type parameter)
- **Purpose**: Preserves the exact input data that caused the failure
- **Usage**: Reproduce issues, debug data-specific problems, audit failures

### Err
- **Type**: `error`
- **Purpose**: The underlying error that caused the failure
- **Usage**: Access original error details, use with `errors.Is` and `errors.As`

### Path
- **Type**: `[]Name`
- **Purpose**: Complete trace of processors/connectors leading to the failure
- **Usage**: Pinpoint exactly where in the pipeline the failure occurred

### Duration
- **Type**: `time.Duration`
- **Purpose**: How long the operation ran before failing
- **Usage**: Identify performance issues, detect timeout patterns

### Timeout
- **Type**: `bool`
- **Purpose**: Indicates if the error was caused by a timeout
- **Usage**: Implement timeout-specific retry logic or alerting

### Canceled
- **Type**: `bool`
- **Purpose**: Indicates if the error was caused by cancellation
- **Usage**: Distinguish intentional shutdowns from actual failures

## Methods

### Error() string

Returns a formatted error message with path and timing information.

```go
func (e *Error[T]) Error() string
```

**Format patterns**:
- Timeout: `"path -> component timed out after 5s: context deadline exceeded"`
- Canceled: `"path -> component canceled after 2s: context canceled"`
- Normal: `"path -> component failed after 100ms: validation error"`

**Example**:
```go
err := &Error[Order]{
    Path: []Name{"order-pipeline", "validate", "check-inventory"},
    Duration: 250 * time.Millisecond,
    Err: errors.New("item out of stock"),
}
fmt.Println(err.Error())
// Output: "order-pipeline -> validate -> check-inventory failed after 250ms: item out of stock"
```

### Unwrap() error

Returns the underlying error for compatibility with Go's error wrapping.

```go
func (e *Error[T]) Unwrap() error
```

**Usage**:
- Enables `errors.Is(err, targetErr)` checking
- Enables `errors.As(err, &targetType)` conversion
- Maintains compatibility with standard error handling

**Example**:
```go
var pipeErr *pipz.Error[Order]
if errors.As(err, &pipeErr) {
    // Access Error[T] fields
    fmt.Printf("Failed at: %v\n", pipeErr.Path)
    fmt.Printf("Input data: %+v\n", pipeErr.InputData)
    
    // Check underlying error
    if errors.Is(pipeErr, sql.ErrNoRows) {
        // Handle specific database error
    }
}
```

### IsTimeout() bool

Checks if the error was caused by a timeout.

```go
func (e *Error[T]) IsTimeout() bool
```

**Returns true when**:
- The `Timeout` field is explicitly set to true
- The underlying error is `context.DeadlineExceeded`

**Example**:
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) && pipeErr.IsTimeout() {
        // Implement timeout-specific handling
        metrics.IncrementTimeouts()
        return retryWithBackoff(data)
    }
}
```

### IsCanceled() bool

Checks if the error was caused by cancellation.

```go
func (e *Error[T]) IsCanceled() bool
```

**Returns true when**:
- The `Canceled` field is explicitly set to true
- The underlying error is `context.Canceled`

**Example**:
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) && pipeErr.IsCanceled() {
        // Don't treat as failure - graceful shutdown
        log.Info("Pipeline canceled during shutdown")
        return nil
    }
}
```

## Usage Examples

### Basic Error Handling

```go
pipeline := pipz.NewSequence("order-processing",
    validateOrder,
    checkInventory,
    processPayment,
    shipOrder,
)

result, err := pipeline.Process(ctx, order)
if err != nil {
    var pipeErr *pipz.Error[Order]
    if errors.As(err, &pipeErr) {
        log.WithFields(log.Fields{
            "path":      strings.Join(pipeErr.Path, " -> "),
            "duration":  pipeErr.Duration,
            "timestamp": pipeErr.Timestamp,
            "order_id":  pipeErr.InputData.ID,
        }).Error("Order processing failed", pipeErr.Err)
    }
}
```

### Retry Logic Based on Error Type

```go
func processWithRetry(ctx context.Context, pipeline Chainable[Data], data Data) (Data, error) {
    for attempt := 0; attempt < 3; attempt++ {
        result, err := pipeline.Process(ctx, data)
        if err == nil {
            return result, nil
        }
        
        var pipeErr *pipz.Error[Data]
        if !errors.As(err, &pipeErr) {
            return data, err // Not a pipeline error
        }
        
        // Don't retry timeouts or cancellations
        if pipeErr.IsTimeout() || pipeErr.IsCanceled() {
            return data, err
        }
        
        // Retry with exponential backoff
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    return data, fmt.Errorf("max retries exceeded")
}
```

### Error Monitoring and Alerting

```go
func monitorPipeline(pipeline Chainable[Request]) Chainable[Request] {
    return pipz.Handle("monitor", 
        pipeline,
        func(ctx context.Context, req Request, err error) {
            var pipeErr *pipz.Error[Request]
            if !errors.As(err, &pipeErr) {
                return // Not a pipeline error
            }
            
            // Send to monitoring system
            metrics.RecordError(metrics.ErrorRecord{
                Path:      pipeErr.Path,
                Duration:  pipeErr.Duration,
                Timeout:   pipeErr.IsTimeout(),
                Canceled:  pipeErr.IsCanceled(),
                RequestID: req.ID,
            })
            
            // Alert on critical paths
            if containsPath(pipeErr.Path, "payment") && !pipeErr.IsCanceled() {
                alerting.SendAlert(alerting.Critical, 
                    fmt.Sprintf("Payment processing failed: %v", pipeErr))
            }
        },
    )
}
```

### Debugging with Full Context

```go
func debugFailure(err error) {
    var pipeErr *pipz.Error[Order]
    if !errors.As(err, &pipeErr) {
        return
    }
    
    fmt.Println("=== Pipeline Failure Debug ===")
    fmt.Printf("Time: %v\n", pipeErr.Timestamp)
    fmt.Printf("Duration: %v\n", pipeErr.Duration)
    fmt.Printf("Path: %s\n", strings.Join(pipeErr.Path, " -> "))
    fmt.Printf("Timeout: %v\n", pipeErr.IsTimeout())
    fmt.Printf("Canceled: %v\n", pipeErr.IsCanceled())
    fmt.Printf("Error: %v\n", pipeErr.Err)
    fmt.Printf("Input Data:\n%+v\n", pipeErr.InputData)
    
    // Check for specific error types
    var validationErr *ValidationError
    if errors.As(pipeErr.Err, &validationErr) {
        fmt.Printf("Validation failures: %v\n", validationErr.Fields)
    }
}
```

## Common Patterns

### Creating Custom Errors

```go
// Wrap external errors with context
func processExternal(ctx context.Context, data Data) (Data, error) {
    result, err := externalAPI.Call(data)
    if err != nil {
        return data, &pipz.Error[Data]{
            Timestamp: time.Now(),
            InputData: data,
            Err:       fmt.Errorf("external API failed: %w", err),
            Path:      []Name{"external-processor"},
            Duration:  time.Since(start),
            Timeout:   errors.Is(err, context.DeadlineExceeded),
        }
    }
    return result, nil
}
```

### Error Recovery

```go
// Recover and continue with partial data
pipeline := pipz.NewFallback("with-recovery",
    primaryPipeline,
    pipz.Apply("recover", func(ctx context.Context, data Data) (Data, error) {
        // Access the error that caused the fallback
        if err := ctx.Value("error"); err != nil {
            var pipeErr *pipz.Error[Data]
            if errors.As(err.(error), &pipeErr) {
                // Log the failure path
                log.Warnf("Primary failed at %v, using fallback", pipeErr.Path)
                
                // Return partial data from the error
                return pipeErr.InputData, nil
            }
        }
        return data, nil
    }),
)
```

## Best Practices

1. **Always check error type** - Use `errors.As` to safely access Error[T] fields
2. **Preserve error chains** - Use `fmt.Errorf` with `%w` verb when wrapping
3. **Log complete context** - Include Path, Duration, and InputData in logs
4. **Handle timeouts differently** - Don't retry timeouts with same timeout duration
5. **Distinguish cancellations** - Treat as graceful shutdown, not failure
6. **Use Path for debugging** - Shows exact failure point in complex pipelines
7. **Monitor Duration** - Detect performance degradation before timeouts

## Gotchas

### ❌ Don't ignore the InputData field
```go
// WRONG - Losing valuable debug information
var pipeErr *pipz.Error[Order]
if errors.As(err, &pipeErr) {
    log.Error(pipeErr.Err) // Only logging the error message
}
```

### ✅ Use all available context
```go
// RIGHT - Complete error context
var pipeErr *pipz.Error[Order]
if errors.As(err, &pipeErr) {
    log.WithFields(log.Fields{
        "order_id": pipeErr.InputData.ID,
        "customer": pipeErr.InputData.CustomerID,
        "amount":   pipeErr.InputData.Total,
        "path":     pipeErr.Path,
        "duration": pipeErr.Duration,
    }).Error("Order failed", pipeErr.Err)
}
```

### ❌ Don't retry canceled operations
```go
// WRONG - Retrying during shutdown
if err != nil {
    return retryOperation(ctx, data)
}
```

### ✅ Check cancellation first
```go
// RIGHT - Respect cancellation
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) && pipeErr.IsCanceled() {
        return data, err // Don't retry - system is shutting down
    }
    return retryOperation(ctx, data)
}
```

## See Also

- [Handle](../processors/handle.md) - Observe and react to errors
- [Fallback](../connectors/fallback.md) - Automatic error recovery
- [Retry](../connectors/retry.md) - Retry failed operations
- [CircuitBreaker](../connectors/circuitbreaker.md) - Prevent cascading failures