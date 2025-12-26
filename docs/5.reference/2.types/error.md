---
title: "Error[T]"
description: "Rich error context type providing complete debugging information for pipeline failures"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - types
  - error-handling
  - debugging
---

# Error[T]

Rich error context for pipeline failures with complete debugging information.

## Overview

`Error[T]` provides comprehensive error information when pipeline processing fails. It captures not just what went wrong, but where, when, and with what data - giving you everything needed to debug production issues.

## Type Definition

```go
type Error[T any] struct {
    Timestamp time.Time    // When the error occurred
    InputData T            // The data that caused the failure
    Err       error        // The underlying error
    Path      []Identity   // Complete processing path to failure point
    Duration  time.Duration // How long before failure
    Timeout   bool         // Whether it was a timeout
    Canceled  bool         // Whether it was canceled
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
- **Panic Recovery**: When processors panic, this contains a `panicError` type with sanitized panic message and processor name for security

### Path
- **Type**: `[]Identity`
- **Purpose**: Complete trace of processors/connectors leading to the failure
- **Usage**: Pinpoint exactly where in the pipeline the failure occurred
- **Note**: Each Identity contains both a name and description, providing rich context about each stage in the pipeline

### Duration
- **Type**: `time.Duration`
- **Purpose**: How long the operation ran before failing
- **Usage**: Identify performance issues, detect timeout patterns
- **Panic Behavior**: Always `0` for panic recovery - timing is not tracked when processors panic

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
var (
    OrderPipelineID  = pipz.NewIdentity("order-pipeline", "Process customer orders")
    ValidateID       = pipz.NewIdentity("validate", "Validate order fields")
    CheckInventoryID = pipz.NewIdentity("check-inventory", "Check product availability")
)

err := &Error[Order]{
    Path: []Identity{
        OrderPipelineID,
        ValidateID,
        CheckInventoryID,
    },
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
var OrderProcessingID = pipz.NewIdentity("order-processing", "Process customer orders")

pipeline := pipz.NewSequence(OrderProcessingID,
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
var MonitorID = pipz.NewIdentity("monitor", "Monitor pipeline errors")

func monitorPipeline(pipeline Chainable[Request]) Chainable[Request] {
    return pipz.Handle(MonitorID,
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

    // Check if it was a panic that was automatically recovered
    if strings.Contains(pipeErr.Error(), "panic in processor") {
        fmt.Println("This was a recovered panic (automatically handled by pipz)")
    }
}
```

## Common Patterns

### Creating Custom Errors

```go
var ExternalProcessorID = pipz.NewIdentity("external-processor", "Call external API")

// Wrap external errors with context
func processExternal(ctx context.Context, data Data) (Data, error) {
    result, err := externalAPI.Call(data)
    if err != nil {
        return data, &pipz.Error[Data]{
            Timestamp: time.Now(),
            InputData: data,
            Err:       fmt.Errorf("external API failed: %w", err),
            Path:      []Identity{ExternalProcessorID},
            Duration:  time.Since(start),
            Timeout:   errors.Is(err, context.DeadlineExceeded),
        }
    }
    return result, nil
}
```

### Error Recovery

```go
var (
    WithRecoveryID = pipz.NewIdentity("with-recovery", "Recover from failures")
    RecoverDataID  = pipz.NewIdentity("recover", "Extract partial data")
)

// Recover and continue with partial data
pipeline := pipz.NewFallback(WithRecoveryID,
    primaryPipeline,
    pipz.Apply(RecoverDataID, func(ctx context.Context, data Data) (Data, error) {
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

## Panic Recovery Errors

pipz automatically recovers from all panics in processor and connector functions, converting them to `Error[T]` instances. When you see errors containing "panic in processor", these represent panics that were automatically caught and sanitized.

### Identifying Panic Errors

```go
result, err := processor.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) {
        // Check if this was a recovered panic
        if strings.Contains(pipeErr.Error(), "panic in processor") {
            log.Warn("Processor panicked but was safely recovered")

            // The panic message is sanitized for security:
            // - Memory addresses redacted (0x*** instead of 0x1234...)
            // - File paths removed to prevent info leakage
            // - Stack traces stripped
            // - Long messages truncated
        }
    }
}
```

### Security Sanitization

Panic messages undergo security sanitization to prevent information leakage:

- **Memory addresses**: `0x1234567890abcdef` → `0x***`
- **File paths**: `panic in /sensitive/path/file.go:123` → `"panic occurred (file path sanitized)"`
- **Stack traces**: `goroutine 1 [running]:...` → `"panic occurred (stack trace sanitized)"`
- **Long messages**: Truncated to prevent log spam

### What This Means for You

1. **No crashes**: Your application will never crash due to panics in pipelines
2. **Error handling**: Panics become regular errors in the error handling flow
3. **Security**: Sensitive information is automatically stripped from panic messages
4. **Monitoring**: You can detect and alert on panic occurrences in production
5. **Debugging**: Use development environments to get more detailed panic information

## See Also

- [Handle](../3.processors/handle.md) - Observe and react to errors
- [Fallback](../4.connectors/fallback.md) - Automatic error recovery
- [Retry](../4.connectors/retry.md) - Retry failed operations
- [CircuitBreaker](../4.connectors/circuitbreaker.md) - Prevent cascading failures
- [Safety and Reliability](../../3.guides/8.safety-reliability.md) - Complete panic recovery documentation
