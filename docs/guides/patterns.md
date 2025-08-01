# Common Patterns

This guide demonstrates powerful patterns you can build with pipz by leveraging the `Chainable[T]` interface and composing processors.

## Infrastructure Patterns

### Rate Limiter

Control the rate of processing to protect downstream services:

```go
import "golang.org/x/time/rate"

type RateLimiter[T any] struct {
    name    string
    limiter *rate.Limiter
}

func (r *RateLimiter[T]) Process(ctx context.Context, data T) (T, error) {
    if err := r.limiter.Wait(ctx); err != nil {
        return data, fmt.Errorf("rate limit exceeded: %w", err)
    }
    return data, nil
}

func (r *RateLimiter[T]) Name() string { return r.name }

// Usage: 100 requests per second with burst of 10
rateLimiter := &RateLimiter[Order]{
    name:    "api-rate-limit",
    limiter: rate.NewLimiter(rate.Every(time.Second/100), 10),
}

pipeline := pipz.NewSequence("throttled-api",
    rateLimiter,
    pipz.Apply("call-api", callExternalAPI),
)
```

### Circuit Breaker

Prevent cascading failures by stopping requests to failing services:

```go
type CircuitBreaker[T any] struct {
    name          string
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
        return data, fmt.Errorf("circuit breaker is open")
    }
    
    cb.mu.Unlock()
    
    // Try the operation
    result, err := cb.executeOperation(ctx, data)
    
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.failures >= cb.failureThreshold {
            cb.state = "open"
        }
        return data, err
    }
    
    // Success - reset on half-open
    if cb.state == "half-open" {
        cb.state = "closed"
        cb.failures = 0
    }
    
    return result, nil
}

func (cb *CircuitBreaker[T]) Name() string { return cb.name }

// Combine with retry for resilient API calls
resilientAPI := pipz.NewSequence("resilient-api",
    &CircuitBreaker[Request]{
        name:             "api-breaker",
        failureThreshold: 5,
        resetTimeout:     30 * time.Second,
    },
    pipz.NewRetry("retry-api", 
        pipz.Apply("call-api", callAPI),
        3,
    ),
)
```

### Bulkhead Pattern

Isolate resources to prevent total system failure:

```go
type Bulkhead[T any] struct {
    name      string
    semaphore chan struct{}
}

func NewBulkhead[T any](name string, maxConcurrent int) *Bulkhead[T] {
    return &Bulkhead[T]{
        name:      name,
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (b *Bulkhead[T]) Process(ctx context.Context, data T) (T, error) {
    select {
    case b.semaphore <- struct{}{}:
        defer func() { <-b.semaphore }()
        return data, nil
    case <-ctx.Done():
        return data, fmt.Errorf("bulkhead timeout: %w", ctx.Err())
    }
}

func (b *Bulkhead[T]) Name() string { return b.name }

// Limit concurrent database connections
dbPipeline := pipz.NewSequence("db-ops",
    NewBulkhead[Query]("db-bulkhead", 10), // Max 10 concurrent
    pipz.Apply("execute-query", executeQuery),
)
```

### Cache-Aside Pattern

Speed up processing with caching:

```go
type CacheAside[T any, K comparable] struct {
    name     string
    cache    sync.Map
    keyFunc  func(T) K
    ttl      time.Duration
    fallback pipz.Chainable[T]
}

type cacheEntry[T any] struct {
    value     T
    expiresAt time.Time
}

func (c *CacheAside[T, K]) Process(ctx context.Context, data T) (T, error) {
    key := c.keyFunc(data)
    
    // Check cache
    if cached, ok := c.cache.Load(key); ok {
        entry := cached.(cacheEntry[T])
        if time.Now().Before(entry.expiresAt) {
            return entry.value, nil
        }
        c.cache.Delete(key)
    }
    
    // Cache miss - use fallback
    result, err := c.fallback.Process(ctx, data)
    if err != nil {
        return data, err
    }
    
    // Cache the result
    c.cache.Store(key, cacheEntry[T]{
        value:     result,
        expiresAt: time.Now().Add(c.ttl),
    })
    
    return result, nil
}

func (c *CacheAside[T, K]) Name() string { return c.name }

// Cache user lookups
userPipeline := pipz.NewSequence("user-lookup",
    &CacheAside[UserRequest, string]{
        name:    "user-cache",
        keyFunc: func(r UserRequest) string { return r.UserID },
        ttl:     5 * time.Minute,
        fallback: pipz.Apply("fetch-user", fetchUserFromDB),
    },
)
```

## Processing Patterns

### Validation Chain

Build complex validation from simple rules:

```go
// Each validator is a simple Effect processor
requiredFields := pipz.Effect("required-fields", func(ctx context.Context, order Order) error {
    if order.CustomerID == "" {
        return errors.New("customer ID required")
    }
    if len(order.Items) == 0 {
        return errors.New("order must have items")
    }
    return nil
})

businessRules := pipz.Effect("business-rules", func(ctx context.Context, order Order) error {
    if order.Total > 10000 && !order.IsApproved {
        return errors.New("high-value orders require approval")
    }
    return nil
})

inventoryCheck := pipz.Apply("inventory", func(ctx context.Context, order Order) (Order, error) {
    for i, item := range order.Items {
        available, err := checkInventory(ctx, item.SKU)
        if err != nil {
            return order, err
        }
        if available < item.Quantity {
            order.Items[i].Status = "backorder"
        }
    }
    return order, nil
})

// Compose into validation pipeline
validation := pipz.NewSequence("order-validation",
    requiredFields,
    businessRules,
    inventoryCheck,
)
```

### Enrichment Pipeline

Add data from multiple sources without blocking on failures:

```go
// Core enrichment that must succeed
mustEnrich := pipz.NewSequence("required-enrichment",
    pipz.Apply("customer-data", enrichCustomerData),
    pipz.Apply("pricing", calculatePricing),
)

// Optional enrichments that won't fail the pipeline
optionalEnrich := pipz.NewConcurrent("optional-enrichment",
    pipz.Enrich("recommendations", addRecommendations),
    pipz.Enrich("loyalty-points", calculateLoyaltyPoints),
    pipz.Enrich("shipping-options", getShippingOptions),
)

// Combine required and optional
enrichmentPipeline := pipz.NewSequence("full-enrichment",
    mustEnrich,
    optionalEnrich,
)
```

### Fan-out/Fan-in

Process data through multiple paths and aggregate results:

```go
// Fan-out to multiple processors
analysisResults := pipz.NewConcurrent("analyze-all",
    pipz.Transform("sentiment", analyzeSentiment),
    pipz.Transform("keywords", extractKeywords),
    pipz.Transform("language", detectLanguage),
)

// Fan-in with aggregation
aggregate := pipz.Transform("aggregate", func(ctx context.Context, results AnalysisResult) AnalysisResult {
    // Results from all three analyses are in the struct
    results.Score = (results.SentimentScore + results.KeywordRelevance) / 2
    return results
})

// Complete fan-out/fan-in pattern
analytics := pipz.NewSequence("text-analytics",
    analysisResults,
    aggregate,
)
```

### Conditional Processing Flows

Route processing based on data characteristics:

```go
// Route by data type
typeRouter := pipz.NewSwitch("type-router",
    func(ctx context.Context, msg Message) string {
        return msg.Type
    },
).
    AddRoute("email", processEmail).
    AddRoute("sms", processSMS).
    AddRoute("push", processPushNotification)

// Route by business rules
priorityRouter := pipz.NewSwitch("priority-router",
    func(ctx context.Context, order Order) string {
        if order.Total > 1000 || order.IsPriority {
            return "express"
        }
        return "standard"
    },
).
    AddRoute("express", expressProcessing).
    AddRoute("standard", standardProcessing)

// Conditional execution
premiumFeatures := pipz.NewFilter("premium-only",
    func(ctx context.Context, user User) bool {
        return user.Subscription == "premium"
    },
    pipz.NewSequence("premium-features",
        pipz.Apply("advanced-analytics", runAdvancedAnalytics),
        pipz.Apply("priority-support", notifyPrioritySupport),
    ),
)
```

## Integration Patterns

### API Gateway Pattern

Build a complete API gateway with pipz:

```go
// API Gateway pipeline
apiGateway := pipz.NewSequence("api-gateway",
    // Rate limiting per client
    &RateLimiter[Request]{
        name:    "client-rate-limit",
        limiter: getClientLimiter(request.ClientID),
    },
    
    // Authentication
    pipz.Apply("authenticate", authenticateRequest),
    
    // Authorization
    pipz.Apply("authorize", checkPermissions),
    
    // Request validation
    pipz.Apply("validate", validateRequest),
    
    // Circuit breaker for backend
    &CircuitBreaker[Request]{
        name:             "backend-breaker",
        failureThreshold: 10,
        resetTimeout:     time.Minute,
    },
    
    // Route to backend with timeout
    pipz.NewTimeout("backend-call",
        pipz.NewSwitch("route", routeToBackend).
            AddRoute("users", userService).
            AddRoute("orders", orderService).
            AddRoute("products", productService),
        30*time.Second,
    ),
    
    // Response transformation
    pipz.Transform("format-response", formatResponse),
)

// With error handling
gatewayWithRecovery := pipz.NewHandle("gateway",
    apiGateway,
    pipz.NewSequence("error-handler",
        pipz.Transform("format-error", formatErrorResponse),
        pipz.Effect("log-error", logError),
        pipz.Effect("metrics", recordErrorMetrics),
    ),
)
```


### Message Queue Integration

Process messages from queues reliably:

```go
type QueueProcessor[T any] struct {
    name     string
    queue    MessageQueue
    pipeline pipz.Chainable[T]
    decoder  func([]byte) (T, error)
}

func (qp *QueueProcessor[T]) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            msg, err := qp.queue.Receive(ctx)
            if err != nil {
                continue
            }
            
            data, err := qp.decoder(msg.Body)
            if err != nil {
                msg.Nack()
                continue
            }
            
            _, err = qp.pipeline.Process(ctx, data)
            if err != nil {
                // Check if error is retryable
                var pipeErr *pipz.Error[T]
                if errors.As(err, &pipeErr) && pipeErr.Timeout {
                    msg.Requeue()
                } else {
                    msg.Nack()
                }
                continue
            }
            
            msg.Ack()
        }
    }
}

// Create a resilient message processor
messageProcessor := &QueueProcessor[Order]{
    name:    "order-queue",
    queue:   orderQueue,
    decoder: decodeOrderMessage,
    pipeline: pipz.NewSequence("process-order",
        pipz.NewRetry("process-with-retry",
            pipz.NewTimeout("timeout-wrapper",
                orderProcessingPipeline,
                30*time.Second,
            ),
            3,
        ),
    ),
}
```

## Performance Patterns

### Batch Processing

Process items in batches for efficiency:

```go
type Batcher[T any] struct {
    name      string
    batchSize int
    timeout   time.Duration
    processor pipz.Chainable[[]T]
    
    mu    sync.Mutex
    batch []T
    timer *time.Timer
}

func (b *Batcher[T]) Process(ctx context.Context, item T) (T, error) {
    b.mu.Lock()
    b.batch = append(b.batch, item)
    
    if len(b.batch) >= b.batchSize {
        batch := b.batch
        b.batch = nil
        b.mu.Unlock()
        
        _, err := b.processor.Process(ctx, batch)
        return item, err
    }
    
    if b.timer == nil {
        b.timer = time.AfterFunc(b.timeout, func() {
            b.flush(context.Background())
        })
    }
    
    b.mu.Unlock()
    return item, nil
}

func (b *Batcher[T]) Name() string { return b.name }

// Batch database inserts
batchInsert := &Batcher[LogEntry]{
    name:      "batch-logs",
    batchSize: 100,
    timeout:   time.Second,
    processor: pipz.Apply("bulk-insert", bulkInsertLogs),
}
```

### Async Fire-and-Forget

Process without waiting for completion:

```go
type AsyncProcessor[T any] struct {
    name      string
    processor pipz.Chainable[T]
    buffer    chan T
}

func NewAsyncProcessor[T any](name string, proc pipz.Chainable[T], bufferSize int) *AsyncProcessor[T] {
    ap := &AsyncProcessor[T]{
        name:      name,
        processor: proc,
        buffer:    make(chan T, bufferSize),
    }
    
    // Start background processor
    go func() {
        for item := range ap.buffer {
            ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
            ap.processor.Process(ctx, item)
            cancel()
        }
    }()
    
    return ap
}

func (ap *AsyncProcessor[T]) Process(ctx context.Context, data T) (T, error) {
    select {
    case ap.buffer <- data:
        return data, nil
    case <-ctx.Done():
        return data, fmt.Errorf("async buffer full")
    }
}

func (ap *AsyncProcessor[T]) Name() string { return ap.name }

// Fire-and-forget analytics
asyncAnalytics := NewAsyncProcessor("async-analytics",
    pipz.NewSequence("analytics",
        pipz.Apply("enrich", enrichAnalyticsData),
        pipz.Apply("send", sendToAnalytics),
    ),
    1000, // Buffer up to 1000 events
)
```

## Best Practices

1. **Keep it Simple**: Start with basic processors and compose them
2. **Name Everything**: Use descriptive names for debugging
3. **Test in Isolation**: Test each custom processor independently
4. **Monitor Performance**: Use benchmarks for critical paths
5. **Handle Context**: Always respect context cancellation
6. **Document Behavior**: Especially for complex custom processors

## Next Steps

- Explore the [examples](../../examples/) directory for complete implementations
- Review [performance guide](./performance.md) for optimization tips
- Check [error handling](../concepts/error-handling.md) for resilience patterns