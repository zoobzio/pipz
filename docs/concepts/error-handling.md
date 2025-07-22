# Error Handling

pipz provides comprehensive error handling patterns to build resilient data pipelines.

## Error Propagation

Errors stop pipeline execution immediately:

```go
pipeline := pipz.Sequential(
    step1, // Returns error
    step2, // Not executed
    step3, // Not executed
)
```

This fail-fast behavior ensures data integrity - partial processing is prevented.

## Built-in Error Patterns

### Retry

Simple retry for transient failures:

```go
// Retry up to 3 times
reliable := pipz.Retry(flakeyOperation, 3)

// Context cancellation is checked between attempts
result, err := reliable.Process(ctx, data)
```

### Retry with Backoff

Exponential backoff for rate limits and overloaded services:

```go
// Delays: 100ms, 200ms, 400ms, 800ms
apiCall := pipz.RetryWithBackoff(
    callExternalAPI,
    5,                    // max attempts  
    100*time.Millisecond, // base delay
)
```

### Fallback

Primary/backup pattern:

```go
// Try Stripe, fall back to PayPal
payment := pipz.Fallback(
    stripeProcessor,
    paypalProcessor,
)
```

Fallback only executes if primary fails. Can be nested:

```go
// Multiple fallbacks
payment := pipz.Fallback(
    stripeProcessor,
    pipz.Fallback(
        paypalProcessor,
        squareProcessor,
    ),
)
```

### Timeout

Prevent hung operations:

```go
// Must complete within 5 seconds
fast := pipz.Timeout(slowOperation, 5*time.Second)
```

Operations should respect context cancellation for immediate termination.

## Error Inspection

### Pipeline Errors

Pipeline errors include context about where failures occurred:

```go
type PipelineError[T any] struct {
    Stage    string        // Processor name where error occurred
    Cause    error         // Original error
    Input    T            // Input data when error occurred
    Pipeline string        // Pipeline identifier
    Timeout  bool          // Was this a timeout?
    Canceled bool          // Was this cancelled?
}

// Usage
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.PipelineError[MyType]
    if errors.As(err, &pipeErr) {
        log.Printf("Failed at stage '%s' with input: %+v", 
            pipeErr.Stage, pipeErr.Input)
    }
}
```

### Error Handler

Observe errors without changing behavior:

```go
tracked := pipz.WithErrorHandler(
    riskyOperation,
    pipz.Effect("track_error", func(ctx context.Context, err error) error {
        metrics.Increment("pipeline.errors")
        log.Printf("Operation failed: %v", err)
        
        // Categorize error
        switch {
        case errors.Is(err, context.DeadlineExceeded):
            metrics.Increment("errors.timeout")
        case errors.Is(err, context.Canceled):
            metrics.Increment("errors.canceled")
        default:
            metrics.Increment("errors.other")
        }
        
        return nil // Error handler errors are ignored
    }),
)
```

## Error Recovery Patterns

### Conditional Recovery

Route based on error type:

```go
type ErrorType string

const (
    TypeRetryable    ErrorType = "retryable"
    TypePermanent    ErrorType = "permanent"
    TypeNeedsReview  ErrorType = "needs_review"
)

func classifyError(err error) ErrorType {
    if errors.Is(err, context.DeadlineExceeded) ||
       strings.Contains(err.Error(), "connection refused") {
        return TypeRetryable
    }
    if strings.Contains(err.Error(), "invalid input") {
        return TypePermanent
    }
    return TypeNeedsReview
}

// Recovery pipeline
recovery := pipz.Switch(
    func(ctx context.Context, failure FailedOrder) ErrorType {
        return classifyError(failure.Error)
    },
    map[ErrorType]pipz.Chainable[FailedOrder]{
        TypeRetryable:   retryPipeline,
        TypePermanent:   rejectPipeline,
        TypeNeedsReview: queueForReview,
    },
)
```

### Circuit Breaker Pattern

Prevent cascading failures:

```go
type CircuitBreaker[T any] struct {
    processor     pipz.Chainable[T]
    failureCount  int32
    threshold     int32
    resetAfter    time.Duration
    lastFailure   time.Time
    mu            sync.Mutex
}

func (cb *CircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
    cb.mu.Lock()
    if cb.failureCount >= cb.threshold {
        if time.Since(cb.lastFailure) < cb.resetAfter {
            cb.mu.Unlock()
            return data, errors.New("circuit breaker open")
        }
        // Reset after cooldown
        atomic.StoreInt32(&cb.failureCount, 0)
    }
    cb.mu.Unlock()
    
    result, err := cb.processor.Process(ctx, data)
    if err != nil {
        atomic.AddInt32(&cb.failureCount, 1)
        cb.mu.Lock()
        cb.lastFailure = time.Now()
        cb.mu.Unlock()
        return data, err
    }
    
    // Success resets failure count
    atomic.StoreInt32(&cb.failureCount, 0)
    return result, nil
}
```

### Dead Letter Queue

Handle unprocessable items:

```go
type DLQ[T any] struct {
    primary pipz.Chainable[T]
    store   Storage[T]
}

func (d *DLQ[T]) Process(ctx context.Context, data T) (T, error) {
    result, err := d.primary.Process(ctx, data)
    if err != nil {
        // Store failed item for later processing
        dlqErr := d.store.Save(ctx, FailedItem[T]{
            Data:      data,
            Error:     err.Error(),
            Timestamp: time.Now(),
            Attempts:  1,
        })
        if dlqErr != nil {
            // Log but don't fail - DLQ is best effort
            log.Printf("DLQ storage failed: %v", dlqErr)
        }
        return data, err
    }
    return result, nil
}
```

## Best Practices

### 1. Fail Fast for Data Integrity

```go
// Good: Validate early
pipeline := pipz.Sequential(
    validateInput,    // Fail fast on bad data
    expensiveProcess,
    saveResults,
)
```

### 2. Use Appropriate Recovery

```go
// Network errors: Retry with backoff
networkCall := pipz.RetryWithBackoff(apiCall, 3, time.Second)

// Service down: Fallback
resilient := pipz.Fallback(primaryService, backupService)

// Slow operations: Timeout
bounded := pipz.Timeout(slowProcess, 30*time.Second)
```

### 3. Preserve Error Context

```go
// Wrap errors with context
func processOrder(ctx context.Context, order Order) (Order, error) {
    validated, err := validate(order)
    if err != nil {
        return order, fmt.Errorf("validation failed for order %s: %w", 
            order.ID, err)
    }
    // ...
}
```

### 4. Monitor Error Patterns

```go
// Track error metrics
errorHandler := pipz.Effect("metrics", func(ctx context.Context, err error) error {
    labels := map[string]string{
        "pipeline": "payment",
        "stage":    getPipelineStage(err),
        "type":     classifyError(err),
    }
    metrics.IncrementWithLabels("pipeline_errors", labels)
    return nil
})

robust := pipz.WithErrorHandler(pipeline, errorHandler)
```

## Testing Error Handling

```go
func TestErrorHandling(t *testing.T) {
    // Test retry behavior
    attempts := 0
    flaky := pipz.Apply("flaky", func(ctx context.Context, data string) (string, error) {
        attempts++
        if attempts < 3 {
            return "", errors.New("temporary failure")
        }
        return data, nil
    })
    
    reliable := pipz.Retry(flaky, 3)
    result, err := reliable.Process(context.Background(), "test")
    
    assert.NoError(t, err)
    assert.Equal(t, 3, attempts)
    assert.Equal(t, "test", result)
}
```

## Next Steps

- [Type Safety](./type-safety.md) - Leveraging generics for safety
- [Testing Guide](../guides/testing.md) - Testing error scenarios
- [Performance Guide](../guides/performance.md) - Error handling performance