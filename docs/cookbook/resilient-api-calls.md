# Recipe: Resilient API Calls

Build production-ready API integrations with automatic retry, circuit breaking, and timeouts.

## Problem

External APIs can fail due to network issues, rate limits, or service outages. You need resilient API calls that:
- Retry transient failures
- Prevent cascading failures
- Respect rate limits
- Timeout gracefully
- Provide fallback responses

## Solution

Layer multiple resilience patterns:

```go
// Complete resilient API setup
func createResilientAPI(endpoint string) pipz.Chainable[Request] {
    // 1. Basic API call
    apiCall := pipz.Apply("api-call", func(ctx context.Context, req Request) (Response, error) {
        return callAPI(ctx, endpoint, req)
    })
    
    // 2. Add retry for transient failures
    withRetry := pipz.NewBackoff("retry", apiCall, 
        3,                      // 3 attempts
        100*time.Millisecond,   // Exponential backoff: 100ms, 200ms, 400ms
    )
    
    // 3. Add circuit breaker
    withBreaker := pipz.NewCircuitBreaker("breaker", withRetry,
        5,                      // Open after 5 failures
        30*time.Second,         // Try recovery after 30s
    )
    
    // 4. Add timeout protection
    withTimeout := pipz.NewTimeout("timeout", withBreaker,
        5*time.Second,          // 5 second timeout per request
    )
    
    // 5. Add rate limiting (singleton!)
    withRateLimit := pipz.NewRateLimiter("rate", withTimeout,
        pipz.WithRateLimiterRate(100),        // 100 requests
        pipz.WithRateLimiterPeriod(time.Second), // per second
    )
    
    // 6. Add fallback for complete failures
    withFallback := pipz.NewFallback("fallback", 
        withRateLimit,
        pipz.Apply("cache", getCachedResponse),
    )
    
    return withFallback
}

// Use the resilient API
var productAPI = createResilientAPI("https://api.example.com/products")

func getProduct(ctx context.Context, id string) (Product, error) {
    req := Request{ProductID: id}
    resp, err := productAPI.Process(ctx, req)
    if err != nil {
        // Check error type for specific handling
        var pipeErr *pipz.Error[Request]
        if errors.As(err, &pipeErr) {
            if pipeErr.Timeout {
                log.Printf("Product API timeout for %s", id)
            }
            if strings.Contains(err.Error(), "circuit breaker is open") {
                log.Printf("Product API circuit open, using cached data")
            }
        }
        return Product{}, err
    }
    return resp.Product, nil
}
```

## Configuration Guidelines

### Retry Settings
- **Quick retry (2-3 attempts)**: For fast, idempotent operations
- **Backoff retry (3-5 attempts)**: For rate-limited APIs
- **No retry**: For non-idempotent operations (payments)

### Circuit Breaker Thresholds
- **Low (3-5)**: Critical services, fast failure preferred
- **Medium (5-10)**: Standard services with occasional issues
- **High (10-20)**: Services with known intermittent problems

### Timeout Values
- **User-facing**: 1-5 seconds
- **Background jobs**: 30-60 seconds
- **Batch processing**: 5-10 minutes

### Rate Limits
- **Match API limits**: Set slightly below provider limits
- **Add buffer**: Use 80-90% of actual limit
- **Monitor and adjust**: Track rate limit errors

## Advanced Patterns

### Dynamic Configuration

```go
type AdaptiveAPI struct {
    breaker *pipz.CircuitBreaker[Request]
    limiter *pipz.RateLimiter[Request]
}

func (a *AdaptiveAPI) AdjustForPeakHours() {
    hour := time.Now().Hour()
    if hour >= 9 && hour <= 17 { // Business hours
        a.breaker.SetFailureThreshold(10) // More tolerant
        a.limiter.SetRate(50)              // Slower rate
    } else {
        a.breaker.SetFailureThreshold(5)   // Less tolerant
        a.limiter.SetRate(200)              // Faster rate
    }
}
```

### Multi-Region Failover

```go
multiRegion := pipz.NewRace("multi-region",
    createResilientAPI("https://us-east.api.example.com"),
    createResilientAPI("https://eu-west.api.example.com"),
    createResilientAPI("https://ap-south.api.example.com"),
)
```

### Request Prioritization

```go
priorityRouter := pipz.NewSwitch[Request]("priority",
    func(ctx context.Context, req Request) string {
        if req.Priority == "high" {
            return "express"
        }
        return "standard"
    },
).
AddRoute("express", expressAPI).      // No rate limit
AddRoute("standard", standardAPI)      // Rate limited
```

## Testing

```go
func TestResilientAPI(t *testing.T) {
    // Mock API that fails first 2 times
    attempts := 0
    mockAPI := pipz.Apply("mock", func(ctx context.Context, req Request) (Response, error) {
        attempts++
        if attempts < 3 {
            return Response{}, errors.New("temporary failure")
        }
        return Response{Data: "success"}, nil
    })
    
    // Wrap with retry
    resilient := pipz.NewRetry("test", mockAPI, 3)
    
    resp, err := resilient.Process(context.Background(), Request{})
    assert.NoError(t, err)
    assert.Equal(t, "success", resp.Data)
    assert.Equal(t, 3, attempts) // Verify it retried
}
```

## Monitoring

```go
// Wrap with metrics collection
monitored := pipz.NewHandle("monitored-api",
    resilientAPI,
    pipz.Effect("metrics", func(ctx context.Context, err *pipz.Error[Request]) error {
        labels := map[string]string{
            "api":      "product",
            "endpoint": err.InputData.Endpoint,
        }
        
        metrics.RecordLatency(err.Duration, labels)
        
        if err.Timeout {
            metrics.Increment("api.timeouts", labels)
        } else if strings.Contains(err.Error(), "circuit breaker") {
            metrics.Increment("api.circuit_open", labels)
        } else {
            metrics.Increment("api.errors", labels)
        }
        
        return nil
    }),
)
```

## Common Pitfalls

1. **Creating rate limiters per request** - Use singletons!
2. **Retrying non-idempotent operations** - Add idempotency keys
3. **No timeout protection** - Always set timeouts
4. **Ignoring circuit breaker state** - Monitor and alert
5. **Same timeout for all operations** - Adjust per use case

## See Also

- [Retry Reference](../reference/connectors/retry.md)
- [CircuitBreaker Reference](../reference/connectors/circuitbreaker.md)
- [RateLimiter Reference](../reference/connectors/ratelimiter.md)
- [Fallback Reference](../reference/connectors/fallback.md)