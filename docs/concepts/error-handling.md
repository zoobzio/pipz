# Error Handling

pipz provides comprehensive error handling patterns to build resilient data pipelines.

## Error Propagation

Errors stop pipeline execution immediately:

```go
pipeline := pipz.NewSequence[Data]("pipeline")
pipeline.Register(
    step1, // Returns error
    step2, // Not executed
    step3, // Not executed
)
```

This fail-fast behavior ensures data integrity - partial processing is prevented.

## Error Pipelines: A Powerful Pattern

### Errors Are Just Another Data Type

One of the most powerful patterns in pipz is treating errors as data that flows through their own pipelines. With the Handle connector accepting `Chainable[*Error[T]]`, you can build sophisticated error recovery flows using the same tools you use for regular data processing.

```go
// Create an error recovery pipeline - it's just another pipeline!
errorRecovery := pipz.NewSequence[*pipz.Error[Order]]("error-recovery",
    // Categorize the error
    pipz.Transform("categorize", func(_ context.Context, err *pipz.Error[Order]) *pipz.Error[Order] {
        if err.Timeout {
            err.InputData.Status = "retry_timeout"
        } else if strings.Contains(err.Err.Error(), "payment") {
            err.InputData.Status = "payment_failed"
        }
        return err
    }),
    
    // Route based on error type
    pipz.NewSwitch("error-router", 
        func(_ context.Context, err *pipz.Error[Order]) string {
            if err.InputData.Amount > 1000 {
                return "high-value"
            }
            return "standard"
        },
    ).AddRoute("high-value", alertOpsTeam).AddRoute("standard", logError),
    
    // Enrich error context
    pipz.Enrich("add-customer-info", func(ctx context.Context, err *pipz.Error[Order]) (*pipz.Error[Order], error) {
        customer, _ := getCustomer(err.InputData.CustomerID)
        err.InputData.CustomerEmail = customer.Email
        return err, nil
    }),
    
    // Attempt recovery
    pipz.Apply("recover", func(ctx context.Context, err *pipz.Error[Order]) (*pipz.Error[Order], error) {
        if err.InputData.Status == "retry_timeout" {
            // Queue for retry
            retryQueue.Add(err.InputData)
        }
        return err, nil
    }),
)

// Wrap your main pipeline with error handling
mainPipeline := pipz.NewHandle("order-processor",
    orderProcessingPipeline,
    errorRecovery, // Errors flow through this pipeline!
)
```

### Why This Pattern Is Powerful

1. **Composability**: Use all pipz features (Switch, Concurrent, Sequence, etc.) for error handling
2. **Type Safety**: Full access to the original input data with type safety
3. **Rich Context**: Error path, timing, and flags available for decision making
4. **Reusability**: Error pipelines can be shared across different main pipelines
5. **Testability**: Error handling logic can be tested independently

### Real-World Example: Payment Processing

```go
// Define a sophisticated error recovery pipeline
paymentErrorPipeline := pipz.NewSequence[*pipz.Error[Payment]]("payment-error-recovery",
    // Log with full context
    pipz.Effect("log", func(_ context.Context, err *pipz.Error[Payment]) error {
        log.Printf("Payment %s failed at %v: %v", 
            err.InputData.ID, err.Path, err.Err)
        return nil
    }),
    
    // Parallel notification and recovery
    pipz.NewConcurrent("parallel-recovery",
        // Notify customer
        pipz.Effect("notify", func(_ context.Context, err *pipz.Error[Payment]) error {
            email.Send(err.InputData.CustomerEmail, 
                "Payment failed", formatError(err))
            return nil
        }),
        
        // Try alternative payment method
        pipz.Apply("fallback-payment", func(ctx context.Context, err *pipz.Error[Payment]) (*pipz.Error[Payment], error) {
            if err.InputData.Method == "card" && hasAlternative(err.InputData.CustomerID) {
                err.InputData.Method = "bank"
                err.InputData.Status = "retry_with_bank"
            }
            return err, nil
        }),
        
        // Update metrics
        pipz.Effect("metrics", func(_ context.Context, err *pipz.Error[Payment]) error {
            metrics.Inc("payment.failures", 
                "provider", err.InputData.Provider,
                "amount_range", getAmountRange(err.InputData.Amount))
            return nil
        }),
    ),
)

// Use it with Handle
paymentProcessor := pipz.NewHandle("payment", 
    actualPaymentPipeline,
    paymentErrorPipeline,
)
```

## Built-in Error Patterns

### Retry

Simple retry for transient failures:

```go
// Retry up to 3 times
reliable := pipz.NewRetry("reliable-op", flakeyOperation, 3)

// Context cancellation is checked between attempts
result, err := reliable.Process(ctx, data)
```

### Backoff

Exponential backoff for rate limits and overloaded services:

```go
// Delays: 100ms, 200ms, 400ms, 800ms
apiCall := pipz.NewBackoff("api-call",
    callExternalAPI,
    5,                    // max attempts  
    100*time.Millisecond, // base delay
)
```

### Fallback

Primary/backup pattern:

```go
// Try Stripe, fall back to PayPal
payment := pipz.NewFallback("payment",
    stripeProcessor,
    paypalProcessor,
)
```

Fallback only executes if primary fails. Can be nested:

```go
// Multiple fallbacks
payment := pipz.NewFallback("payment",
    stripeProcessor,
    pipz.NewFallback("secondary-payment",
        paypalProcessor,
        squareProcessor,
    ),
)
```

### Timeout

Prevent hung operations:

```go
// Must complete within 5 seconds
fast := pipz.NewTimeout("fast-op", slowOperation, 5*time.Second)
```

Operations should respect context cancellation for immediate termination.

## Error Inspection

### Pipeline Errors

Pipeline errors include context about where failures occurred:

```go
type Error[T any] struct {
    Path      []Name        // Full path through processors
    Err       error         // Original error
    InputData T             // Input data when error occurred
    Timeout   bool          // Was this a timeout?
    Canceled  bool          // Was this cancelled?
    Timestamp time.Time     // When the error occurred
    Duration  time.Duration // How long processing took before error
}

// Usage
result, err := pipeline.Process(ctx, data)
if err != nil {
    // Process returns *Error[T] directly
    log.Printf("Failed at path %v with input: %+v", 
        err.Path, err.InputData)
    log.Printf("Duration before failure: %v", err.Duration)
    
    if err.Timeout {
        log.Println("Failed due to timeout")
    }
}
```

### Handle Connector

The Handle connector wraps any processor with an error handling pipeline:

```go
// Handle accepts a Chainable[*Error[T]] - a full pipeline for errors!
tracked := pipz.NewHandle("tracked-op",
    riskyOperation,
    pipz.NewSequence[*pipz.Error[Data]]("error-handler",
        // Access full error context
        pipz.Effect("track_error", func(ctx context.Context, err *pipz.Error[Data]) error {
            metrics.Increment("pipeline.errors", 
                "path", strings.Join(err.Path, "."),
                "duration_ms", err.Duration.Milliseconds())
            log.Printf("Operation failed: %v", err)
            
            // Categorize using error flags
            switch {
            case err.Timeout:
                metrics.Increment("errors.timeout")
            case err.Canceled:
                metrics.Increment("errors.canceled")
            default:
                metrics.Increment("errors.other")
            }
            
            return nil
        }),
        
        // Can even modify the input data for recovery
        pipz.Transform("mark_for_retry", func(_ context.Context, err *pipz.Error[Data]) *pipz.Error[Data] {
            if err.Timeout && err.InputData.RetryCount < 3 {
                err.InputData.Status = "pending_retry"
                err.InputData.RetryCount++
            }
            return err
        }),
    ),
)
```

## Error Recovery Patterns

### Building Recovery Pipelines

The key insight is that error recovery is just another pipeline. You can compose recovery logic from simpler pieces:

```go
// Reusable error categorization
categorizeError := pipz.Transform[*pipz.Error[Order]]("categorize",
    func(_ context.Context, err *pipz.Error[Order]) *pipz.Error[Order] {
        switch {
        case err.Timeout:
            err.InputData.ErrorType = "timeout"
        case strings.Contains(err.Err.Error(), "payment"):
            err.InputData.ErrorType = "payment"
        case strings.Contains(err.Err.Error(), "inventory"):
            err.InputData.ErrorType = "inventory"
        default:
            err.InputData.ErrorType = "unknown"
        }
        return err
    },
)

// Reusable notification
notifyOps := pipz.Effect[*pipz.Error[Order]]("notify_ops",
    func(_ context.Context, err *pipz.Error[Order]) error {
        if err.InputData.Amount > 10000 {
            alertOps(fmt.Sprintf("High-value order %s failed: %v", 
                err.InputData.ID, err.Err))
        }
        return nil
    },
)

// Compose into recovery pipeline
orderRecovery := pipz.NewSequence[*pipz.Error[Order]]("order-recovery",
    categorizeError,
    notifyOps,
    pipz.NewSwitch[*pipz.Error[Order]]("recovery-router",
        func(_ context.Context, err *pipz.Error[Order]) string {
            return err.InputData.ErrorType
        },
    ).AddRoute("timeout", retryLater).
      AddRoute("payment", tryAlternativePayment).
      AddRoute("inventory", backorder),
)
```

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
recovery := pipz.NewSwitch("recovery",
    func(ctx context.Context, failure FailedOrder) ErrorType {
        return classifyError(failure.Error)
    },
)

// Add routes
recovery.AddRoute(TypeRetryable, retryPipeline)
recovery.AddRoute(TypePermanent, rejectPipeline)
recovery.AddRoute(TypeNeedsReview, queueForReview)
```

### Combining Error Patterns

You can combine multiple error handling patterns using pipelines:

```go
// Create a comprehensive error handling strategy
comprehensiveErrorHandling := pipz.NewSequence[*pipz.Error[Request]]("comprehensive-error",
    // First, categorize and log
    pipz.Apply("categorize", categorizeAndLog),
    
    // Then, attempt immediate recovery
    pipz.NewFallback("immediate-recovery",
        pipz.Apply("quick-retry", func(ctx context.Context, err *pipz.Error[Request]) (*pipz.Error[Request], error) {
            if err.Timeout && err.InputData.RetryCount == 0 {
                // One immediate retry for timeouts
                result, retryErr := processRequest(ctx, err.InputData)
                if retryErr == nil {
                    err.InputData = result
                    err.Err = nil // Mark as recovered
                }
            }
            return err, nil
        }),
        pipz.Transform("mark-failed", func(_ context.Context, err *pipz.Error[Request]) *pipz.Error[Request] {
            err.InputData.Status = "failed"
            return err
        }),
    ),
    
    // Finally, handle based on recovery result  
    pipz.NewSwitch[*pipz.Error[Request]]("post-recovery",
        func(_ context.Context, err *pipz.Error[Request]) string {
            if err.Err == nil {
                return "recovered"
            }
            return "still-failed"
        },
    ).AddRoute("recovered", logRecovery).
      AddRoute("still-failed", queueForManualReview),
)
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
pipeline := pipz.NewSequence[Data]("pipeline")
pipeline.Register(
    validateInput,    // Fail fast on bad data
    expensiveProcess,
    saveResults,
)

// Better: Add error recovery
withRecovery := pipz.NewHandle("pipeline-with-recovery",
    pipeline,
    pipz.NewSequence[*pipz.Error[Data]]("recovery",
        logError,
        notifyIfCritical,
        queueForRetry,
    ),
)
```

### 2. Use Appropriate Recovery

```go
// Network errors: Retry with backoff
networkCall := pipz.NewBackoff("network-call", apiCall, 3, time.Second)

// Service down: Fallback
resilient := pipz.NewFallback("resilient", primaryService, backupService)

// Slow operations: Timeout
bounded := pipz.NewTimeout("bounded", slowProcess, 30*time.Second)
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
// Build a monitoring pipeline for errors
errorMonitoring := pipz.NewSequence[*pipz.Error[Payment]]("error-monitoring",
    // Extract metrics
    pipz.Effect("metrics", func(ctx context.Context, err *pipz.Error[Payment]) error {
        labels := map[string]string{
            "pipeline": "payment",
            "stage":    err.Path[len(err.Path)-1], // Last stage that failed
            "type":     classifyError(err.Err),
            "timeout":  strconv.FormatBool(err.Timeout),
        }
        metrics.IncrementWithLabels("pipeline_errors", labels)
        metrics.Histogram("pipeline_error_duration_ms", 
            err.Duration.Milliseconds(), labels)
        return nil
    }),
    
    // Alert on patterns
    pipz.Apply("detect-patterns", func(ctx context.Context, err *pipz.Error[Payment]) (*pipz.Error[Payment], error) {
        if recentErrors := getRecentErrors(err.Path); recentErrors > 10 {
            alert("High error rate in pipeline: " + strings.Join(err.Path, " -> "))
        }
        return err, nil
    }),
)

robust := pipz.NewHandle("robust", pipeline, errorMonitoring)
```

### 5. Compose Error Handling

```go
// Create reusable error handling components
var (
    // Basic logging
    logErrors = pipz.Effect[*pipz.Error[any]]("log", func(_ context.Context, err *pipz.Error[any]) error {
        log.Printf("Error: %v", err)
        return nil
    })
    
    // Timeout recovery
    timeoutRecovery = pipz.NewSequence[*pipz.Error[Order]]("timeout-recovery",
        pipz.Apply("check-timeout", func(ctx context.Context, err *pipz.Error[Order]) (*pipz.Error[Order], error) {
            if err.Timeout {
                retryQueue.Add(err.InputData)
            }
            return err, nil
        }),
    )
    
    // Payment-specific recovery
    paymentRecovery = pipz.NewSwitch[*pipz.Error[Payment]]("payment-recovery",
        func(_ context.Context, err *pipz.Error[Payment]) string {
            if strings.Contains(err.Err.Error(), "insufficient_funds") {
                return "insufficient_funds"
            }
            return "other"
        },
    ).AddRoute("insufficient_funds", notifyCustomer).
      AddRoute("other", logAndAlert)
)

// Compose for specific use cases
orderPipeline := pipz.NewHandle("orders", processOrder, timeoutRecovery)
paymentPipeline := pipz.NewHandle("payments", processPayment, paymentRecovery)
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
    
    reliable := pipz.NewRetry("reliable", flaky, 3)
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