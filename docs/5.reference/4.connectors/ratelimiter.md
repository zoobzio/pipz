---
title: "RateLimiter"
description: "Controls the rate of processing to protect downstream services and rate-limited resources"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - rate-limiting
  - throttling
  - resource-control
---

# RateLimiter

Controls the rate of processing to protect downstream services and rate-limited resources.

## Function Signatures

```go
// Create rate limiter with specified rate and burst capacity
func NewRateLimiter[T any](name Name, ratePerSecond float64, burst int) *RateLimiter[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `ratePerSecond` (`float64`) - Sustained rate limit (requests per second)
- `burst` (`int`) - Maximum burst capacity (immediate requests allowed)

## Returns

Returns a `*RateLimiter[T]` that implements `Chainable[T]`.

## Testing Configuration

### WithClock

```go
func (r *RateLimiter[T]) WithClock(clock clockz.Clock) *RateLimiter[T]
```

Sets a custom clock implementation for testing purposes. This method enables controlled time manipulation in tests using `clockz.FakeClock`.

**Parameters:**
- `clock` (`clockz.Clock`) - Clock implementation to use

**Returns:**
Returns the same connector instance for method chaining.

**Example:**
```go
// Use fake clock in tests
fakeClock := clockz.NewFakeClock()
rateLimiter := pipz.NewRateLimiter[string]("test", 10.0, 5).
    WithClock(fakeClock)

// Advance time in test to replenish tokens
fakeClock.Advance(1 * time.Second)
```

## Behavior

### Rate Limiting Algorithm
- **Token bucket** - Uses golang.org/x/time/rate package for proven implementation
- **Sustained rate** - Long-term average rate is maintained
- **Burst handling** - Allows controlled bursts up to the specified limit
- **Thread-safe** - Safe for concurrent access from multiple goroutines

### Operating Modes
- **Wait mode (default)** - Blocks until a token is available
- **Drop mode** - Returns error immediately if no tokens available

### Context Handling
- **Cancellation support** - Respects context cancellation during waits
- **Timeout detection** - Properly handles context deadline exceeded
- **Error enrichment** - Provides detailed error information

## Signals

RateLimiter emits typed signals for throttling and request handling via [capitan](https://github.com/zoobzio/capitan):

| Signal | When Emitted | Fields |
|--------|--------------|--------|
| `ratelimiter.allowed` | Request allowed, token consumed | `name`, `tokens`, `rate`, `burst` |
| `ratelimiter.throttled` | Request waiting for tokens (wait mode) | `name`, `wait_time`, `tokens`, `rate` |
| `ratelimiter.dropped` | Request dropped, no tokens available (drop mode) | `name`, `tokens`, `rate`, `burst`, `mode` |

**Example:**

```go
import "github.com/zoobzio/capitan"

// Hook rate limiter signals
capitan.Hook(pipz.SignalRateLimiterDropped, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    rate, _ := pipz.FieldRate.From(e)
    // Alert on dropped requests
})
```

See [Hooks Documentation](../../2.learn/5.hooks.md) for complete signal reference and usage examples.

## Configuration Methods

```go
// Runtime configuration
rateLimiter.SetRate(200)           // Update to 200 requests/second
rateLimiter.SetBurst(20)           // Update burst capacity to 20
rateLimiter.SetMode("drop")        // Switch to drop mode

// Getters
rate := rateLimiter.GetRate()      // Current rate limit
burst := rateLimiter.GetBurst()    // Current burst capacity
mode := rateLimiter.GetMode()      // Current mode ("wait" or "drop")
```

## Example

```go
// Basic rate limiter - 100 requests per second with burst of 10
rateLimiter := pipz.NewRateLimiter("api-limiter", 100, 10)

// Use in a pipeline
pipeline := pipz.NewSequence("throttled-api",
    rateLimiter,
    pipz.Apply("call-external-api", callExternalAPI),
)

// Runtime configuration
rateLimiter.SetMode("drop")        // Don't wait, fail fast
rateLimiter.SetRate(200)           // Increase rate during off-peak hours

// Per-user rate limiting
userLimiter := pipz.NewSwitch("user-limiter", getUserTier).
    AddRoute("premium", pipz.NewRateLimiter("premium-rate", 1000, 100)).
    AddRoute("standard", pipz.NewRateLimiter("standard-rate", 100, 10)).
    AddRoute("free", pipz.NewRateLimiter("free-rate", 10, 1))

// API Gateway with rate limiting
apiGateway := pipz.NewSequence("api-gateway",
    pipz.Apply("authenticate", authenticateRequest),
    pipz.NewRateLimiter("global-limit", 10000, 1000),
    userLimiter,
    pipz.Apply("route-request", routeToBackend),
)
```

## When to Use

Use `RateLimiter` when:
- **Protecting external APIs with rate limits** (Twitter, OpenAI, etc.)
- Preventing overwhelming of downstream services
- Implementing fair resource sharing
- Meeting SLA requirements
- Controlling database connection usage
- Throttling expensive operations
- Cost control (pay-per-request APIs)

Use **Wait mode** when:
- You want to honor all requests eventually
- Latency spikes are acceptable
- The calling system can handle delays
- Rate limits are soft boundaries

Use **Drop mode** when:
- Fast failure is preferred over delays
- System load shedding is required
- Rate limits are hard boundaries
- You want to fail fast under pressure

## When NOT to Use

Don't use `RateLimiter` when:
- No external rate limits exist
- All operations are equally cheap
- Backpressure isn't needed
- Different error handling is needed (use `CircuitBreaker`)
- You need per-user limits (create multiple limiters)

## Error Messages

RateLimiter provides clear error information:

```go
limiter := pipz.NewRateLimiter("api", 10, 1)
limiter.SetMode("drop")

_, err := limiter.Process(ctx, request)
if err != nil {
    var pipeErr *pipz.Error[Request]
    if errors.As(err, &pipeErr) {
        // Error path shows where rate limiting occurred
        // Example: "api failed after 0s: rate limit exceeded"
        fmt.Printf("Rate limited at: %s\n", strings.Join(pipeErr.Path, " → "))
    }
}
```

## Common Patterns

```go
// External API protection
httpClient := pipz.NewSequence("rate-limited-client",
    pipz.NewRateLimiter("api-rate", 100, 20),     // API rate limit
    pipz.NewTimeout("request-timeout", 
        pipz.Apply("http-request", makeHTTPRequest),
        30*time.Second,
    ),
)

// Database connection throttling
dbOperations := pipz.NewSequence("db-ops",
    pipz.NewRateLimiter("db-rate", 1000, 50),     // Protect database
    pipz.NewRetry("db-retry",
        pipz.Apply("execute-query", runQuery),
        3,
    ),
)

// Burst vs sustained rate
emailSender := pipz.NewRateLimiter("email-rate", 
    10,    // 10 emails per second sustained
    100,   // But allow burst of 100 emails
)

// Dynamic rate limiting
dynamicLimiter := pipz.NewRateLimiter("dynamic", 100, 10)
// Adjust rate based on time of day, load, etc.
go func() {
    for {
        if isOffPeakHours() {
            dynamicLimiter.SetRate(1000)  // Higher rate during off-peak
        } else {
            dynamicLimiter.SetRate(100)   // Lower rate during peak
        }
        time.Sleep(5 * time.Minute)
    }
}()

// Graceful degradation
gracefulAPI := pipz.NewFallback("graceful-api",
    pipz.NewSequence("full-api",
        pipz.NewRateLimiter("primary-rate", 1000, 100),
        primaryAPICall,
    ),
    pipz.NewSequence("fallback-api",
        pipz.NewRateLimiter("fallback-rate", 100, 10),
        fallbackAPICall,
    ),
)
```

## Gotchas

### ❌ Don't create rate limiters per request
```go
// WRONG - New limiter each time, no rate limiting!
func handleRequest(req Request) Response {
    limiter := pipz.NewRateLimiter("api", 100, 10) // New instance!
    return limiter.Process(ctx, req) // Useless!
}
```

### ✅ Create once, reuse
```go
// RIGHT - Shared limiter for all requests
var apiLimiter = pipz.NewRateLimiter("api", 100, 10)

func handleRequest(req Request) Response {
    return apiLimiter.Process(ctx, req)
}
```

### ❌ Don't ignore burst capacity
```go
// WRONG - Burst of 1 causes unnecessary blocking
limiter := pipz.NewRateLimiter("strict", 100, 1) // Burst of 1!
// Can't handle any burst traffic
```

### ✅ Set reasonable burst capacity
```go
// RIGHT - Allow some burst
limiter := pipz.NewRateLimiter("flexible", 100, 20) // 20% burst
// Can handle traffic spikes
```

### ❌ Don't use wait mode for user-facing APIs
```go
// WRONG - Users wait indefinitely
apiHandler := pipz.NewRateLimiter("user-api", 10, 1)
// Default is "wait" mode - users stuck waiting!
```

### ✅ Use drop mode for user-facing services
```go
// RIGHT - Fail fast for users
apiHandler := pipz.NewRateLimiter("user-api", 100, 10)
apiHandler.SetMode("drop") // Return 429 immediately
```

### ❌ Don't forget rate limits are per instance
```go
// WRONG - Each server has its own limit
// If you have 10 servers, actual rate is 10x!
limiter := pipz.NewRateLimiter("api", 100, 10)
```

### ✅ Adjust for number of instances
```go
// RIGHT - Divide by instance count
instanceCount := getInstanceCount()
limiter := pipz.NewRateLimiter("api", 
    100/float64(instanceCount), // Distributed rate
    10,
)
```

## Advanced Patterns

```go
// Multi-tier rate limiting
multiTierLimiter := pipz.NewSequence("multi-tier",
    pipz.NewRateLimiter("global", 10000, 1000),          // Global limit
    pipz.NewRateLimiter("per-service", 1000, 100),       // Per-service limit
    pipz.NewRateLimiter("per-endpoint", 100, 10),        // Per-endpoint limit
)

// Rate limiting with circuit breaker
resilientAPI := pipz.NewSequence("resilient",
    pipz.NewRateLimiter("rate-limit", 100, 10),
    pipz.NewCircuitBreaker("circuit", apiCall, 5, 30*time.Second),
    pipz.NewRetry("retry", apiCall, 3),
)

// Custom rate limit error handling
smartLimiter := pipz.NewHandle("smart-rate-limiter",
    pipz.NewRateLimiter("limiter", 100, 10),
    pipz.NewSwitch("rate-error-handler", 
        func(ctx context.Context, err *pipz.Error[Request]) string {
            if strings.Contains(err.Err.Error(), "rate limit exceeded") {
                return "rate-limited"
            }
            return "other"
        },
    ).
    AddRoute("rate-limited", 
        pipz.Effect("log-rate-limit", logRateLimitHit),
    ).
    AddRoute("other", 
        pipz.Effect("log-other", logOtherError),
    ),
)

// Adaptive rate limiting
type AdaptiveRateLimiter struct {
    limiter     *pipz.RateLimiter[Request]
    successRate float64
    mu          sync.Mutex
}

func (a *AdaptiveRateLimiter) Process(ctx context.Context, req Request) (Request, error) {
    result, err := a.limiter.Process(ctx, req)
    
    a.mu.Lock()
    if err == nil {
        a.successRate = a.successRate*0.9 + 0.1  // Increase success rate
        if a.successRate > 0.95 {
            // Increase rate if success rate is high
            currentRate := a.limiter.GetRate()
            a.limiter.SetRate(currentRate * 1.1)
        }
    } else {
        a.successRate = a.successRate * 0.9       // Decrease success rate
        if a.successRate < 0.8 {
            // Decrease rate if success rate is low
            currentRate := a.limiter.GetRate()
            a.limiter.SetRate(currentRate * 0.9)
        }
    }
    a.mu.Unlock()
    
    return result, err
}
```

## Performance Characteristics

- **Low overhead** - ~1μs per operation when not rate limited
- **Wait mode** - Blocks until token available (can add latency)
- **Drop mode** - Fast failure, no waiting
- **Memory usage** - Minimal, constant regardless of rate
- **Thread safety** - Fully concurrent, no contention

## See Also

- [CircuitBreaker](./circuitbreaker.md) - For handling service failures
- [Timeout](./timeout.md) - Often combined with rate limiting
- [Retry](./retry.md) - For handling rate limit errors
- [Switch](./switch.md) - For conditional rate limiting
- [Sequence](./sequence.md) - For building rate-limited pipelines