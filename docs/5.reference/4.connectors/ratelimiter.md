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
// Create rate limiter wrapping a processor with specified rate and burst capacity
func NewRateLimiter[T any](identity Identity, ratePerSecond float64, burst int, processor Chainable[T]) *RateLimiter[T]
```

## Parameters

- `identity` (`Identity`) - Identity containing name and description for debugging and documentation
- `ratePerSecond` (`float64`) - Sustained rate limit (requests per second)
- `burst` (`int`) - Maximum burst capacity (immediate requests allowed)
- `processor` (`Chainable[T]`) - The processor to rate limit

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
var (
    TestID      = pipz.NewIdentity("test", "test rate limiter with fake clock")
    ProcessorID = pipz.NewIdentity("processor", "the wrapped processor")
)

processor := pipz.Transform(ProcessorID, func(_ context.Context, s string) string { return s })
fakeClock := clockz.NewFakeClock()
rateLimiter := pipz.NewRateLimiter(TestID, 10.0, 5, processor).WithClock(fakeClock)

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
// Define identities upfront
var (
    APILimiterID     = pipz.NewIdentity("api-limiter", "limit API requests to 100/sec with burst of 10")
    CallExternalID   = pipz.NewIdentity("call-external-api", "call external API endpoint")
    UserLimiterID    = pipz.NewIdentity("user-limiter", "route to tier-specific rate limiter")
    PremiumRateID    = pipz.NewIdentity("premium-rate", "premium tier rate limit: 1000/sec")
    StandardRateID   = pipz.NewIdentity("standard-rate", "standard tier rate limit: 100/sec")
    FreeRateID       = pipz.NewIdentity("free-rate", "free tier rate limit: 10/sec")
    APIGatewayID     = pipz.NewIdentity("api-gateway", "API gateway with global and per-user rate limiting")
    AuthenticateID   = pipz.NewIdentity("authenticate", "authenticate incoming request")
    GlobalLimitID    = pipz.NewIdentity("global-limit", "global rate limit: 10000/sec")
    RouteRequestID   = pipz.NewIdentity("route-request", "route request to backend service")
    PremiumAPIID     = pipz.NewIdentity("premium-api", "premium tier API call")
    StandardAPIID    = pipz.NewIdentity("standard-api", "standard tier API call")
    FreeAPIID        = pipz.NewIdentity("free-api", "free tier API call")
)

// Basic rate limiter wrapping an API call - 100 requests per second with burst of 10
apiCall := pipz.Apply(CallExternalID, callExternalAPI)
rateLimiter := pipz.NewRateLimiter(APILimiterID, 100, 10, apiCall)

// Runtime configuration
rateLimiter.SetMode("drop")        // Don't wait, fail fast
rateLimiter.SetRate(200)           // Increase rate during off-peak hours

// Per-user rate limiting - each tier wraps its own API processor
userLimiter := pipz.NewSwitch(UserLimiterID, getUserTier).
    AddRoute("premium", pipz.NewRateLimiter(PremiumRateID, 1000, 100,
        pipz.Apply(PremiumAPIID, callPremiumAPI))).
    AddRoute("standard", pipz.NewRateLimiter(StandardRateID, 100, 10,
        pipz.Apply(StandardAPIID, callStandardAPI))).
    AddRoute("free", pipz.NewRateLimiter(FreeRateID, 10, 1,
        pipz.Apply(FreeAPIID, callFreeAPI)))

// API Gateway with rate limiting wrapping the backend router
apiGateway := pipz.NewSequence(APIGatewayID,
    pipz.Apply(AuthenticateID, authenticateRequest),
    pipz.NewRateLimiter(GlobalLimitID, 10000, 1000,
        pipz.Apply(RouteRequestID, routeToBackend)),
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
var (
    APIID       = pipz.NewIdentity("api", "API rate limiter at 10/sec")
    ProcessorID = pipz.NewIdentity("processor", "the rate-limited processor")
)

limiter := pipz.NewRateLimiter(APIID, 10, 1, pipz.Apply(ProcessorID, processRequest))
limiter.SetMode("drop")

_, err := limiter.Process(ctx, request)
if err != nil {
    var pipeErr *pipz.Error[Request]
    if errors.As(err, &pipeErr) {
        // Error path shows where rate limiting occurred
        // Example: "api failed after 0s: rate limit exceeded"
        fmt.Printf("Rate limited at: %v\n", pipeErr.Path)
    }
}
```

## Common Patterns

```go
// External API protection - rate limiter wraps the timeout chain
var (
    RateLimitedClientID = pipz.NewIdentity("rate-limited-client", "HTTP client with rate limiting and timeout")
    APIRateID           = pipz.NewIdentity("api-rate", "limit API requests to 100/sec")
    RequestTimeoutID    = pipz.NewIdentity("request-timeout", "30 second timeout for HTTP requests")
    HTTPRequestID       = pipz.NewIdentity("http-request", "make HTTP request")
)

httpClient := pipz.NewRateLimiter(APIRateID, 100, 20,
    pipz.NewTimeout(RequestTimeoutID,
        pipz.Apply(HTTPRequestID, makeHTTPRequest),
        30*time.Second,
    ),
)

// Database connection throttling - rate limiter wraps retry chain
var (
    DBRateID       = pipz.NewIdentity("db-rate", "protect database with 1000/sec rate limit")
    DBRetryID      = pipz.NewIdentity("db-retry", "retry failed database queries")
    ExecuteQueryID = pipz.NewIdentity("execute-query", "execute database query")
)

dbOperations := pipz.NewRateLimiter(DBRateID, 1000, 50,
    pipz.NewRetry(DBRetryID,
        pipz.Apply(ExecuteQueryID, runQuery),
        3,
    ),
)

// Burst vs sustained rate
var (
    EmailRateID  = pipz.NewIdentity("email-rate", "email sending with 10/sec sustained, 100 burst capacity")
    SendEmailID  = pipz.NewIdentity("send-email", "send email via SMTP")
)

emailSender := pipz.NewRateLimiter(EmailRateID,
    10,    // 10 emails per second sustained
    100,   // But allow burst of 100 emails
    pipz.Apply(SendEmailID, sendEmail),
)

// Dynamic rate limiting
var (
    DynamicID     = pipz.NewIdentity("dynamic", "rate limiter with dynamic adjustment based on load")
    ProcessorID   = pipz.NewIdentity("processor", "the rate-limited processor")
)

dynamicLimiter := pipz.NewRateLimiter(DynamicID, 100, 10,
    pipz.Apply(ProcessorID, processRequest),
)

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

// Graceful degradation - each fallback option has its own rate limiter
var (
    GracefulAPIID    = pipz.NewIdentity("graceful-api", "graceful degradation from primary to fallback API")
    PrimaryRateID    = pipz.NewIdentity("primary-rate", "primary API rate limit: 1000/sec")
    FallbackRateID   = pipz.NewIdentity("fallback-rate", "fallback API rate limit: 100/sec")
    PrimaryCallID    = pipz.NewIdentity("primary-call", "call primary API")
    FallbackCallID   = pipz.NewIdentity("fallback-call", "call fallback API")
)

gracefulAPI := pipz.NewFallback(GracefulAPIID,
    pipz.NewRateLimiter(PrimaryRateID, 1000, 100,
        pipz.Apply(PrimaryCallID, callPrimaryAPI)),
    pipz.NewRateLimiter(FallbackRateID, 100, 10,
        pipz.Apply(FallbackCallID, callFallbackAPI)),
)
```

## Gotchas

### ❌ Don't create rate limiters per request
```go
// WRONG - New limiter each time, no rate limiting!
var (
    APIID       = pipz.NewIdentity("api", "API rate limiter")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

func handleRequest(req Request) Response {
    processor := pipz.Apply(ProcessorID, processRequest)
    limiter := pipz.NewRateLimiter(APIID, 100, 10, processor) // New instance!
    return limiter.Process(ctx, req) // Useless!
}
```

### ✅ Create once, reuse
```go
// RIGHT - Shared limiter for all requests
var (
    APIID       = pipz.NewIdentity("api", "shared API rate limiter at 100/sec")
    ProcessorID = pipz.NewIdentity("processor", "process request")
    apiLimiter  = pipz.NewRateLimiter(APIID, 100, 10,
        pipz.Apply(ProcessorID, processRequest))
)

func handleRequest(req Request) Response {
    return apiLimiter.Process(ctx, req)
}
```

### ❌ Don't ignore burst capacity
```go
// WRONG - Burst of 1 causes unnecessary blocking
var (
    StrictID    = pipz.NewIdentity("strict", "strict rate limiter with minimal burst")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

limiter := pipz.NewRateLimiter(StrictID, 100, 1, // Burst of 1!
    pipz.Apply(ProcessorID, processRequest))
// Can't handle any burst traffic
```

### ✅ Set reasonable burst capacity
```go
// RIGHT - Allow some burst
var (
    FlexibleID  = pipz.NewIdentity("flexible", "flexible rate limiter with 20% burst capacity")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

limiter := pipz.NewRateLimiter(FlexibleID, 100, 20, // 20% burst
    pipz.Apply(ProcessorID, processRequest))
// Can handle traffic spikes
```

### ❌ Don't use wait mode for user-facing APIs
```go
// WRONG - Users wait indefinitely
var (
    UserAPIID   = pipz.NewIdentity("user-api", "user-facing API rate limiter")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

apiHandler := pipz.NewRateLimiter(UserAPIID, 10, 1,
    pipz.Apply(ProcessorID, processRequest))
// Default is "wait" mode - users stuck waiting!
```

### ✅ Use drop mode for user-facing services
```go
// RIGHT - Fail fast for users
var (
    UserAPIID   = pipz.NewIdentity("user-api", "user-facing API with fail-fast rate limiting")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

apiHandler := pipz.NewRateLimiter(UserAPIID, 100, 10,
    pipz.Apply(ProcessorID, processRequest))
apiHandler.SetMode("drop") // Return 429 immediately
```

### ❌ Don't forget rate limits are per instance
```go
// WRONG - Each server has its own limit
// If you have 10 servers, actual rate is 10x!
var (
    APIID       = pipz.NewIdentity("api", "API rate limiter")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

limiter := pipz.NewRateLimiter(APIID, 100, 10,
    pipz.Apply(ProcessorID, processRequest))
```

### ✅ Adjust for number of instances
```go
// RIGHT - Divide by instance count
var (
    APIID       = pipz.NewIdentity("api", "distributed API rate limiter adjusted for instance count")
    ProcessorID = pipz.NewIdentity("processor", "process request")
)

instanceCount := getInstanceCount()
limiter := pipz.NewRateLimiter(APIID, 100/float64(instanceCount), 10, // Distributed rate
    pipz.Apply(ProcessorID, processRequest))
```

## Advanced Patterns

```go
// Multi-tier rate limiting - nested wrappers provide layered protection
var (
    GlobalID       = pipz.NewIdentity("global", "global rate limit: 10000/sec")
    PerServiceID   = pipz.NewIdentity("per-service", "per-service rate limit: 1000/sec")
    PerEndpointID  = pipz.NewIdentity("per-endpoint", "per-endpoint rate limit: 100/sec")
    ProcessorID    = pipz.NewIdentity("processor", "the rate-limited processor")
)

multiTierLimiter := pipz.NewRateLimiter(GlobalID, 10000, 1000,
    pipz.NewRateLimiter(PerServiceID, 1000, 100,
        pipz.NewRateLimiter(PerEndpointID, 100, 10,
            pipz.Apply(ProcessorID, processRequest))))

// Rate limiting with circuit breaker - rate limiter wraps the resilience chain
var (
    RateLimitID   = pipz.NewIdentity("rate-limit", "rate limit before circuit breaker")
    CircuitID     = pipz.NewIdentity("circuit", "circuit breaker for API calls")
    RetryID       = pipz.NewIdentity("retry", "retry failed API calls")
    APICallID     = pipz.NewIdentity("api-call", "the actual API call")
)

resilientAPI := pipz.NewRateLimiter(RateLimitID, 100, 10,
    pipz.NewCircuitBreaker(CircuitID,
        pipz.NewRetry(RetryID,
            pipz.Apply(APICallID, callAPI),
            3),
        5, 30*time.Second))

// Custom rate limit error handling
var (
    SmartLimiterID      = pipz.NewIdentity("smart-rate-limiter", "rate limiter with custom error handling")
    LimiterID           = pipz.NewIdentity("limiter", "base rate limiter at 100/sec")
    ProcessID           = pipz.NewIdentity("process", "process the request")
    RateErrorHandlerID  = pipz.NewIdentity("rate-error-handler", "route based on error type")
    LogRateLimitID      = pipz.NewIdentity("log-rate-limit", "log rate limit hit")
    LogOtherID          = pipz.NewIdentity("log-other", "log other errors")
)

smartLimiter := pipz.NewHandle(SmartLimiterID,
    pipz.NewRateLimiter(LimiterID, 100, 10,
        pipz.Apply(ProcessID, processRequest)),
    pipz.NewSwitch(RateErrorHandlerID,
        func(ctx context.Context, err *pipz.Error[Request]) string {
            if strings.Contains(err.Err.Error(), "rate limit exceeded") {
                return "rate-limited"
            }
            return "other"
        },
    ).
    AddRoute("rate-limited",
        pipz.Effect(LogRateLimitID, logRateLimitHit),
    ).
    AddRoute("other",
        pipz.Effect(LogOtherID, logOtherError),
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