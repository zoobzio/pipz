# Common Patterns

This guide demonstrates powerful patterns you can build with pipz by leveraging the `Chainable[T]` interface and composing processors.

## Complex Pipeline Architecture

Here's how a production-ready pipeline architecture looks when combining multiple patterns:

```
┌──────────────────────────────────────────────────────────────────┐
│              Production Order Processing Pipeline                 │
└──────────────────────────────────────────────────────────────────┘

                ┌─────────────────┐
                │   User Request  │
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │  Rate Limiter   │──→ [Drop if exceeded]
                │  100 req/sec    │
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │    Validate     │──→ [Fail fast on bad input]
                │  • Required     │
                │  • Business     │
                └────────┬────────┘
                         │
                         ▼
              ┌──────────┴──────────┐
              │   Switch Router     │
              │  (by order type)    │
              └─┬────────────────┬──┘
                │                │
        [premium user]     [regular user]
                │                │
         ┌──────▼──────┐  ┌──────▼──────┐
         │  Fast Path  │  │ Normal Path │
         │ • Priority  │  │ • Standard  │
         │ • Express   │  │ • Queue     │
         └──────┬──────┘  └──────┬──────┘
                │                │
                └────────┬───────┘
                         │
                         ▼
         ┌───────────────────────────────┐
         │     Circuit Breaker           │
         │   (Payment Processing)        │
         │  • Closed: Process normally   │
         │  • Open: Fail fast            │
         │  • Half-Open: Test recovery   │
         └───────────────┬───────────────┘
                         │
                         ▼
         ┌───────────────────────────────┐
         │    Retry with Backoff         │
         │  • Attempt 1: 100ms delay     │
         │  • Attempt 2: 200ms delay     │
         │  • Attempt 3: 400ms delay     │
         └───────────────┬───────────────┘
                         │
                         ▼
         ┌───────────────────────────────┐
         │      Timeout Wrapper          │
         │     (30 second limit)         │
         └───────────────┬───────────────┘
                         │
                    [Success]
                         │
                         ▼
         ┌───────────────────────────────┐
         │    Concurrent Notifications   │
         ├───────────────────────────────┤
         │ ┌─────────┐ ┌─────────┐      │
         │ │  Email  │ │   SMS   │      │
         │ └─────────┘ └─────────┘      │
         │ ┌─────────┐ ┌─────────┐      │
         │ │Analytics│ │   CRM   │      │
         │ └─────────┘ └─────────┘      │
         └───────────────────────────────┘
                         │
                    All complete
                         │
                         ▼
                ┌─────────────────┐
                │     Response     │
                └─────────────────┘

Error Flow:
═══════════
Any Stage [✗] ──→ Error Handler ──→ Categorize ──→ Recovery Strategy
                         │                              │
                         ▼                              ▼
                  [Log & Metrics]            [Retry / Fallback / Alert]
```

### Implementation of Complex Pipeline

```go
// Build the complete pipeline shown in the diagram
func BuildOrderPipeline() pipz.Chainable[Order] {
    // Rate limiting layer
    rateLimiter := pipz.NewRateLimiter("rate-limit", 100, 10)
    
    // Validation layer
    validation := pipz.NewSequence("validation",
        pipz.Apply("validate-required", validateRequired),
        pipz.Apply("validate-business", validateBusinessRules),
    )
    
    // Routing layer - different paths for different users
    routing := pipz.NewSwitch("order-router",
        func(ctx context.Context, order Order) string {
            if order.Customer.IsPremium {
                return "premium"
            }
            return "regular"
        },
    ).
    AddRoute("premium", pipz.NewSequence("fast-path",
        pipz.Apply("priority-queue", addToPriorityQueue),
        pipz.Apply("express-processing", expressProcess),
    )).
    AddRoute("regular", pipz.NewSequence("normal-path", 
        pipz.Apply("standard-queue", addToStandardQueue),
        pipz.Apply("standard-processing", standardProcess),
    ))
    
    // Payment processing with full resilience
    payment := pipz.NewCircuitBreaker("payment-breaker",
        pipz.NewBackoff("payment-retry",
            pipz.NewTimeout("payment-timeout",
                pipz.Apply("charge-payment", chargePayment),
                30*time.Second,
            ),
            3,
            100*time.Millisecond,
        ),
        5,
        time.Minute,
    )
    
    // Concurrent notifications
    notifications := pipz.NewConcurrent("notifications",
        pipz.Effect("send-email", sendEmailNotification),
        pipz.Effect("send-sms", sendSMSNotification),
        pipz.Effect("update-analytics", updateAnalytics),
        pipz.Effect("update-crm", updateCRM),
    )
    
    // Compose everything
    return pipz.NewSequence("order-pipeline",
        rateLimiter,
        validation,
        routing,
        payment,
        notifications,
    )
}
```

## Infrastructure Patterns

### Rate Limiter (Built-in Connector)

Control the rate of processing to protect downstream services. pipz provides a built-in RateLimiter connector:

```go
// 100 requests per second with burst of 10
rateLimiter := pipz.NewRateLimiter("api-rate-limit", 100, 10)

// Configure mode: "wait" (default) or "drop"
rateLimiter.SetMode("drop") // Return error immediately if rate exceeded

// Use in a pipeline
pipeline := pipz.NewSequence("throttled-api",
    rateLimiter,
    pipz.Apply("call-api", callExternalAPI),
)

// Runtime configuration
rateLimiter.SetRate(200)    // Update to 200/sec
rateLimiter.SetBurst(20)    // Update burst capacity
```

The RateLimiter uses a token bucket algorithm and provides:
- Two modes: "wait" (blocks until allowed) or "drop" (fails immediately)
- Runtime reconfiguration of rate and burst
- Context-aware cancellation during waits
- Thread-safe operation

### Circuit Breaker (Built-in Connector)

Prevent cascading failures by stopping requests to failing services. pipz provides a built-in CircuitBreaker connector:

```go
// Open circuit after 5 failures, try recovery after 30 seconds
breaker := pipz.NewCircuitBreaker("api-breaker", apiProcessor, 5, 30*time.Second)

// Configure thresholds
breaker.SetFailureThreshold(10)     // Failures to open
breaker.SetSuccessThreshold(3)      // Successes to close from half-open
breaker.SetResetTimeout(time.Minute) // Recovery timeout

// Check state
state := breaker.GetState() // "closed", "open", or "half-open"

// Manual reset if needed
breaker.Reset()

// Combine with retry for resilient API calls
resilientAPI := pipz.NewSequence("resilient-api",
    breaker,
    pipz.NewRetry("retry-api", 
        pipz.Apply("call-api", callAPI),
        3,
    ),
)
```

The CircuitBreaker implements the standard circuit breaker pattern with:
- Three states: closed (normal), open (blocking), half-open (testing)
- Automatic recovery attempts after timeout
- Configurable failure and success thresholds
- Thread-safe state management
- Manual reset capability

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
// API Gateway pipeline using built-in connectors
apiGateway := pipz.NewSequence("api-gateway",
    // Rate limiting per client
    pipz.NewRateLimiter("client-rate-limit", 100, 20), // 100/sec, burst 20
    
    // Authentication
    pipz.Apply("authenticate", authenticateRequest),
    
    // Authorization
    pipz.Apply("authorize", checkPermissions),
    
    // Request validation
    pipz.Apply("validate", validateRequest),
    
    // Circuit breaker for backend
    pipz.NewCircuitBreaker("backend-breaker",
        pipz.NewSwitch("route", routeToBackend).
            AddRoute("users", userService).
            AddRoute("orders", orderService).
            AddRoute("products", productService),
        10,              // Open after 10 failures
        time.Minute,     // Try recovery after 1 minute
    ),
    
    // Timeout wrapper
    pipz.NewTimeout("deadline", 
        pipz.Transform("format-response", formatResponse),
        30*time.Second,
    ),
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