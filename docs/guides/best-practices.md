# Best Practices

Guidelines for building production-ready pipelines with pipz.

## Design Principles

### 1. Single Responsibility

Each processor should do one thing well:

```go
// Good: Focused processors
validateEmail := pipz.Apply("validate_email", func(ctx context.Context, u User) (User, error) {
    if !isValidEmail(u.Email) {
        return u, errors.New("invalid email format")
    }
    return u, nil
})

normalizeEmail := pipz.Transform("normalize_email", func(ctx context.Context, u User) User {
    u.Email = strings.ToLower(strings.TrimSpace(u.Email))
    return u
})

// Bad: Doing too much
processEmail := pipz.Apply("process_email", func(ctx context.Context, u User) (User, error) {
    // Validating AND normalizing AND checking duplicates
    u.Email = strings.ToLower(strings.TrimSpace(u.Email))
    if !isValidEmail(u.Email) {
        return u, errors.New("invalid email")
    }
    if emailExists(u.Email) {
        return u, errors.New("email exists")
    }
    return u, nil
})
```

### 2. Explicit Error Handling

Make error handling visible and intentional:

```go
// Good: Clear error strategy
payment := pipz.Sequential(
    pipz.Apply("validate", validatePayment),
    
    // Critical operation with retry
    pipz.RetryWithBackoff(
        pipz.Apply("charge", chargeCard),
        3,
        time.Second,
    ),
    
    // Non-critical with error handler
    pipz.WithErrorHandler(
        pipz.Effect("notify", sendNotification),
        pipz.Effect("log_notification_error", logError),
    ),
)

// Bad: Hidden error handling
payment := pipz.Apply("process", func(ctx context.Context, p Payment) (Payment, error) {
    // Validation, charging, and notification all mixed together
    // Error handling buried in function
})
```

### 3. Type-Safe Routes

Use custom types for routing decisions:

```go
// Good: Type-safe enum
type OrderPriority int
const (
    PriorityStandard OrderPriority = iota
    PriorityExpress
    PriorityOvernight
)

router := pipz.Switch(
    func(ctx context.Context, o Order) OrderPriority {
        if o.ShippingMethod == "overnight" {
            return PriorityOvernight
        }
        // ...
    },
    map[OrderPriority]pipz.Chainable[Order]{
        PriorityStandard:  standardFulfillment,
        PriorityExpress:   expressFulfillment,
        PriorityOvernight: overnightFulfillment,
    },
)

// Bad: Magic strings
router := pipz.Switch(
    func(ctx context.Context, o Order) string {
        return o.ShippingMethod // "standard", "express", etc.
    },
    map[string]pipz.Chainable[Order]{
        "standard": standardFulfillment,
        "express":  expressFulfillment,
        // Easy to typo, no compile-time checking
    },
)
```

## Pipeline Patterns

### Pattern: Validation First

Always validate early to fail fast:

```go
pipeline := pipz.Sequential(
    // Validate first - cheap and catches errors early
    pipz.Apply("validate_structure", validateStructure),
    pipz.Apply("validate_business_rules", validateBusinessRules),
    
    // Then expensive operations
    pipz.Apply("enrich_from_api", enrichFromAPI),
    pipz.Apply("calculate_pricing", calculatePricing),
    pipz.Apply("save_to_database", saveToDatabase),
)
```

### Pattern: Graceful Degradation

Continue processing even when non-critical operations fail:

```go
pipeline := pipz.Sequential(
    // Critical path
    pipz.Apply("validate", validate),
    pipz.Apply("process", process),
    pipz.Apply("save", save),
    
    // Best-effort enrichments
    pipz.Enrich("add_recommendations", func(ctx context.Context, order Order) (Order, error) {
        recs, err := recommendationService.Get(ctx, order.UserID)
        if err != nil {
            return order, err // Enrich will ignore this
        }
        order.Recommendations = recs
        return order, nil
    }),
    
    // Non-blocking notifications
    pipz.Concurrent(
        pipz.WithErrorHandler(
            pipz.Effect("email", sendEmail),
            pipz.Effect("log_email_error", logError),
        ),
        pipz.Effect("analytics", trackAnalytics),
    ),
)
```

### Pattern: Bulkhead Isolation

Isolate failures to prevent cascade:

```go
// Separate pipelines for different concerns
orderPipeline := pipz.Sequential(
    validateOrder,
    processPayment,
    updateInventory,
)

// Isolated notification pipeline
notificationPipeline := pipz.Timeout(
    pipz.Concurrent(
        sendEmail,
        sendSMS,
        updateCRM,
    ),
    5*time.Second, // Don't let notifications block orders
)

// Compose with isolation
fullPipeline := pipz.Sequential(
    orderPipeline,
    pipz.WithErrorHandler(
        notificationPipeline,
        pipz.Effect("log_notification_failure", logError),
    ),
)
```

### Pattern: Feature Flags

Support gradual rollouts and A/B testing:

```go
func createPipeline(features FeatureFlags) pipz.Chainable[Order] {
    processors := []pipz.Chainable[Order]{
        validateOrder,
        calculatePricing,
    }
    
    // Conditionally add processors
    if features.IsEnabled("fraud-detection-v2") {
        processors = append(processors, fraudDetectionV2)
    } else {
        processors = append(processors, fraudDetectionV1)
    }
    
    if features.IsEnabled("loyalty-points") {
        processors = append(processors, calculateLoyaltyPoints)
    }
    
    processors = append(processors, 
        chargePayment,
        fulfillOrder,
    )
    
    return pipz.Sequential(processors...)
}
```

## Error Handling Strategies

### Strategy: Categorize and Route

```go
type ErrorCategory string

const (
    CategoryValidation ErrorCategory = "validation"
    CategoryTransient  ErrorCategory = "transient"
    CategoryBusiness   ErrorCategory = "business"
    CategorySystem     ErrorCategory = "system"
)

func handleError(ctx context.Context, failure FailedOrder) (FailedOrder, error) {
    category := categorizeError(failure.Error)
    
    switch category {
    case CategoryValidation:
        // Don't retry bad input
        return sendToDeadLetter(ctx, failure)
        
    case CategoryTransient:
        // Retry with backoff
        return pipz.RetryWithBackoff(
            reprocessOrder,
            5,
            time.Second,
        ).Process(ctx, failure)
        
    case CategoryBusiness:
        // Needs human intervention
        return sendToManualReview(ctx, failure)
        
    case CategorySystem:
        // Alert ops team
        alertOps(failure.Error)
        return sendToDeadLetter(ctx, failure)
    }
    
    return failure, failure.Error
}
```

### Strategy: Circuit Breaker per Dependency

```go
var (
    stripeBreaker = NewCircuitBreaker("stripe", 5, 30*time.Second)
    dbBreaker     = NewCircuitBreaker("database", 10, 1*time.Minute)
    cacheBreaker  = NewCircuitBreaker("cache", 20, 10*time.Second)
)

pipeline := pipz.Sequential(
    // Wrap external calls with circuit breakers
    pipz.Apply("load_from_cache", func(ctx context.Context, id string) (Order, error) {
        return cacheBreaker.Do(ctx, func() (Order, error) {
            return cache.Get(ctx, id)
        })
    }),
    
    pipz.Apply("charge_payment", func(ctx context.Context, order Order) (Order, error) {
        return stripeBreaker.Do(ctx, func() (Order, error) {
            return stripe.Charge(ctx, order)
        })
    }),
    
    pipz.Apply("save_order", func(ctx context.Context, order Order) (Order, error) {
        return dbBreaker.Do(ctx, func() (Order, error) {
            return db.Save(ctx, order)
        })
    }),
)
```

## Monitoring and Observability

### Add Metrics

```go
func withMetrics[T any](name string, processor pipz.Chainable[T]) pipz.Chainable[T] {
    return pipz.ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
        start := time.Now()
        
        result, err := processor.Process(ctx, data)
        
        duration := time.Since(start)
        labels := map[string]string{
            "processor": name,
            "success":   fmt.Sprintf("%t", err == nil),
        }
        
        metrics.ObserveHistogram("pipeline_duration_seconds", duration.Seconds(), labels)
        metrics.IncrementCounter("pipeline_processed_total", labels)
        
        if err != nil {
            metrics.IncrementCounter("pipeline_errors_total", labels)
        }
        
        return result, err
    })
}

// Use throughout pipeline
pipeline := pipz.Sequential(
    withMetrics("validate", validateOrder),
    withMetrics("enrich", enrichOrder),
    withMetrics("process", processOrder),
)
```

### Add Tracing

```go
func withTracing[T any](name string, processor pipz.Chainable[T]) pipz.Chainable[T] {
    return pipz.ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
        span, ctx := opentracing.StartSpanFromContext(ctx, name)
        defer span.Finish()
        
        result, err := processor.Process(ctx, data)
        
        if err != nil {
            span.SetTag("error", true)
            span.LogFields(
                log.String("error.message", err.Error()),
            )
        }
        
        return result, err
    })
}
```

## Production Checklist

### Design
- [ ] Each processor has single responsibility
- [ ] Error handling is explicit and appropriate
- [ ] Uses type-safe routing (no magic strings)
- [ ] Validates input early
- [ ] Non-critical operations don't block critical path

### Resilience
- [ ] Timeouts on all external calls
- [ ] Retry logic for transient failures
- [ ] Circuit breakers for dependencies
- [ ] Graceful degradation for features
- [ ] Bulkhead isolation between components

### Observability
- [ ] Metrics on all processors
- [ ] Distributed tracing enabled
- [ ] Structured logging throughout
- [ ] Error categorization and alerting
- [ ] Performance benchmarks

### Operations
- [ ] Feature flags for gradual rollout
- [ ] Dead letter queue for failed items
- [ ] Manual intervention workflow
- [ ] Runbooks for common issues
- [ ] Load testing completed

## Anti-Patterns to Avoid

1. **God Processor**: One processor doing everything
2. **Silent Failures**: Swallowing errors without logging
3. **Unbounded Retries**: Retrying forever without backoff
4. **Missing Timeouts**: No time bounds on operations
5. **Shared Mutable State**: Processors modifying shared data
6. **Magic Strings**: Using strings instead of typed constants
7. **Hidden Dependencies**: Processors with side effects on external state

## Next Steps

- [Examples](../examples/payment-processing.md) - See patterns in action
- [Performance Guide](./performance.md) - Optimize for production
- [Error Recovery](./error-recovery.md) - Advanced error patterns