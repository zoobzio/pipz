# Error Recovery Patterns

Learn advanced patterns for handling and recovering from errors in your pipelines.

## Error Classification

First, understand what type of error you're dealing with:

```go
type ErrorClass int

const (
    ClassTransient ErrorClass = iota  // Temporary, can retry
    ClassInvalid                      // Bad input, don't retry
    ClassSystemic                     // System failure, need fallback
    ClassQuota                        // Rate limit, backoff needed
    ClassUnknown                      // Investigate manually
)

func classifyError(err error) ErrorClass {
    errStr := err.Error()
    
    // Transient errors
    if errors.Is(err, context.DeadlineExceeded) ||
       strings.Contains(errStr, "connection refused") ||
       strings.Contains(errStr, "temporary failure") {
        return ClassTransient
    }
    
    // Invalid input
    if strings.Contains(errStr, "invalid") ||
       strings.Contains(errStr, "validation failed") ||
       strings.Contains(errStr, "bad request") {
        return ClassInvalid
    }
    
    // Rate limits
    if strings.Contains(errStr, "rate limit") ||
       strings.Contains(errStr, "too many requests") {
        return ClassQuota
    }
    
    // System failures
    if strings.Contains(errStr, "service unavailable") ||
       strings.Contains(errStr, "internal server error") {
        return ClassSystemic
    }
    
    return ClassUnknown
}
```

## Recovery Strategies

### Strategy 1: Selective Retry

Only retry errors that might succeed:

```go
func createSelectiveRetryPipeline[T any](
    processor pipz.Chainable[T],
) pipz.Chainable[T] {
    return pipz.ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
        var lastErr error
        
        for attempt := 0; attempt < 3; attempt++ {
            result, err := processor.Process(ctx, data)
            if err == nil {
                return result, nil
            }
            
            lastErr = err
            
            // Only retry transient errors
            if classifyError(err) != ClassTransient {
                return data, err
            }
            
            // Exponential backoff
            if attempt < 2 {
                delay := time.Duration(attempt+1) * 100 * time.Millisecond
                select {
                case <-time.After(delay):
                case <-ctx.Done():
                    return data, ctx.Err()
                }
            }
        }
        
        return data, fmt.Errorf("failed after 3 attempts: %w", lastErr)
    })
}
```

### Strategy 2: Fallback Chain

Try multiple alternatives:

```go
type PaymentProvider struct {
    Name      string
    Processor pipz.Chainable[Payment]
    Health    float64 // Success rate
}

func createFallbackChain(providers []PaymentProvider) pipz.Chainable[Payment] {
    // Sort by health score
    sort.Slice(providers, func(i, j int) bool {
        return providers[i].Health > providers[j].Health
    })
    
    return pipz.ProcessorFunc[Payment](func(ctx context.Context, payment Payment) (Payment, error) {
        var lastErr error
        
        for _, provider := range providers {
            // Skip unhealthy providers
            if provider.Health < 0.5 {
                continue
            }
            
            payment.Provider = provider.Name
            result, err := provider.Processor.Process(ctx, payment)
            if err == nil {
                return result, nil
            }
            
            lastErr = err
            log.Printf("Provider %s failed: %v", provider.Name, err)
            
            // Don't try other providers for invalid input
            if classifyError(err) == ClassInvalid {
                return payment, err
            }
        }
        
        return payment, fmt.Errorf("all providers failed: %w", lastErr)
    })
}
```

### Strategy 3: Circuit Breaker

Prevent cascading failures:

```go
type CircuitBreaker[T any] struct {
    processor    pipz.Chainable[T]
    failureThreshold int
    resetTimeout     time.Duration
    
    mu           sync.Mutex
    failures     int
    lastFailTime time.Time
    state        string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
    cb.mu.Lock()
    
    // Check if we should reset
    if cb.state == "open" && time.Since(cb.lastFailTime) > cb.resetTimeout {
        cb.state = "half-open"
        cb.failures = 0
    }
    
    if cb.state == "open" {
        cb.mu.Unlock()
        return data, errors.New("circuit breaker is open")
    }
    
    cb.mu.Unlock()
    
    // Try the operation
    result, err := cb.processor.Process(ctx, data)
    
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.failures >= cb.failureThreshold {
            cb.state = "open"
            log.Printf("Circuit breaker opened after %d failures", cb.failures)
        }
        
        return data, err
    }
    
    // Success - reset state
    if cb.state == "half-open" {
        cb.state = "closed"
        log.Println("Circuit breaker closed after successful operation")
    }
    cb.failures = 0
    
    return result, nil
}
```

### Strategy 4: Compensation

Undo previous operations on failure:

```go
type CompensatingPipeline[T any] struct {
    steps []struct {
        forward  pipz.Chainable[T]
        rollback func(context.Context, T) error
    }
}

func (cp *CompensatingPipeline[T]) Process(ctx context.Context, data T) (T, error) {
    completed := 0
    current := data
    
    // Execute forward operations
    for i, step := range cp.steps {
        result, err := step.forward.Process(ctx, current)
        if err != nil {
            // Rollback completed steps in reverse order
            for j := completed - 1; j >= 0; j-- {
                if cp.steps[j].rollback != nil {
                    rollbackErr := cp.steps[j].rollback(ctx, current)
                    if rollbackErr != nil {
                        log.Printf("Rollback failed at step %d: %v", j, rollbackErr)
                    }
                }
            }
            return data, fmt.Errorf("step %d failed: %w", i, err)
        }
        current = result
        completed++
    }
    
    return current, nil
}

// Usage
compensating := &CompensatingPipeline[Order]{
    steps: []struct {
        forward  pipz.Chainable[Order]
        rollback func(context.Context, Order) error
    }{
        {
            forward: chargePayment,
            rollback: func(ctx context.Context, order Order) error {
                return refundPayment(ctx, order)
            },
        },
        {
            forward: reserveInventory,
            rollback: func(ctx context.Context, order Order) error {
                return releaseInventory(ctx, order)
            },
        },
        {
            forward: shipOrder,
            rollback: nil, // Can't rollback shipping
        },
    },
}
```

### Strategy 5: Bulkhead Pattern

Isolate failures to prevent system-wide impact:

```go
func createBulkhead[T any](
    processor pipz.Chainable[T],
    maxConcurrent int,
) pipz.Chainable[T] {
    semaphore := make(chan struct{}, maxConcurrent)
    
    return pipz.ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
        select {
        case semaphore <- struct{}{}:
            defer func() { <-semaphore }()
            return processor.Process(ctx, data)
        case <-ctx.Done():
            return data, ctx.Err()
        default:
            return data, errors.New("bulkhead at capacity")
        }
    })
}
```

## Complex Recovery Pipeline

Combining multiple strategies:

```go
func createResilientPipeline() pipz.Chainable[Order] {
    // Create circuit breakers for each provider
    stripeBreaker := &CircuitBreaker[Order]{
        processor:        createStripeProcessor(),
        failureThreshold: 5,
        resetTimeout:     30 * time.Second,
    }
    
    paypalBreaker := &CircuitBreaker[Order]{
        processor:        createPayPalProcessor(),
        failureThreshold: 5,
        resetTimeout:     30 * time.Second,
    }
    
    return pipz.Sequential(
        // Validate with no retry (invalid input won't get better)
        pipz.Apply("validate", validateOrder),
        
        // Payment with sophisticated error handling
        pipz.ProcessorFunc[Order](func(ctx context.Context, order Order) (Order, error) {
            // Try primary provider with circuit breaker
            result, err := stripeBreaker.Process(ctx, order)
            if err == nil {
                return result, nil
            }
            
            // Classify error
            errClass := classifyError(err)
            
            switch errClass {
            case ClassTransient:
                // Retry with backoff
                return pipz.RetryWithBackoff(
                    stripeBreaker,
                    3,
                    time.Second,
                ).Process(ctx, order)
                
            case ClassSystemic:
                // Try fallback provider
                return paypalBreaker.Process(ctx, order)
                
            case ClassQuota:
                // Queue for later processing
                return queueForLater(ctx, order)
                
            default:
                // No recovery possible
                return order, err
            }
        }),
        
        // Always attempt these (non-critical)
        pipz.Concurrent(
            bulkheadNotification,
            asyncAnalytics,
        ),
    )
}
```

## Monitoring Recovery

Track recovery effectiveness:

```go
type RecoveryMetrics struct {
    Attempts  map[string]int64
    Successes map[string]int64
    mu        sync.RWMutex
}

func (rm *RecoveryMetrics) Record(strategy string, success bool) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rm.Attempts[strategy]++
    if success {
        rm.Successes[strategy]++
    }
}

func (rm *RecoveryMetrics) SuccessRate(strategy string) float64 {
    rm.mu.RLock()
    defer rm.mu.RUnlock()
    
    attempts := rm.Attempts[strategy]
    if attempts == 0 {
        return 0
    }
    
    return float64(rm.Successes[strategy]) / float64(attempts)
}
```

## Best Practices

1. **Classify Errors**: Different errors need different strategies
2. **Fail Fast on Invalid Input**: Don't retry validation errors
3. **Use Circuit Breakers**: Prevent cascading failures
4. **Monitor Recovery**: Track what works and what doesn't
5. **Set Timeouts**: Don't let recovery attempts hang forever
6. **Log Everything**: You'll need it for debugging
7. **Test Failure Scenarios**: Use chaos engineering principles

## Testing Recovery

```go
func TestRecoveryPipeline(t *testing.T) {
    // Create a processor that fails then succeeds
    attempts := 0
    flaky := pipz.Apply("flaky", func(ctx context.Context, data string) (string, error) {
        attempts++
        if attempts < 3 {
            return "", errors.New("connection refused")
        }
        return data + "_processed", nil
    })
    
    // Wrap with recovery
    recovered := createSelectiveRetryPipeline(flaky)
    
    result, err := recovered.Process(context.Background(), "test")
    assert.NoError(t, err)
    assert.Equal(t, "test_processed", result)
    assert.Equal(t, 3, attempts)
}
```

## Next Steps

- [Performance Guide](./performance.md) - Recovery performance impact
- [Testing Guide](./testing.md) - Testing failure scenarios
- [Best Practices](./best-practices.md) - Production patterns