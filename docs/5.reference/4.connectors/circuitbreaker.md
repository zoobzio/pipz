---
title: "CircuitBreaker"
description: "Prevents cascading failures by stopping requests to failing services and allowing time for recovery"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - resilience
  - circuit-breaker
  - fault-tolerance
---

# CircuitBreaker

Prevents cascading failures by stopping requests to failing services and allowing time for recovery.

## Function Signatures

```go
// Create circuit breaker with failure threshold and reset timeout
func NewCircuitBreaker[T any](identity Identity, processor Chainable[T], failureThreshold int, resetTimeout time.Duration) *CircuitBreaker[T]
```

## Parameters

- `identity` (`Identity`) - Identifier for the connector used in debugging
- `processor` (`Chainable[T]`) - The processor to protect with circuit breaking
- `failureThreshold` (`int`) - Number of consecutive failures before opening circuit
- `resetTimeout` (`time.Duration`) - Time to wait before attempting recovery

## Returns

Returns a `*CircuitBreaker[T]` that implements `Chainable[T]`.

## Testing Configuration

### WithClock

```go
func (cb *CircuitBreaker[T]) WithClock(clock clockz.Clock) *CircuitBreaker[T]
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
cb := pipz.NewCircuitBreaker(
    pipz.NewIdentity("test", "Test circuit breaker"),
    processor, 3, 30*time.Second).
    WithClock(fakeClock)

// Advance time in test to trigger state transitions
fakeClock.Advance(31 * time.Second)
```

## Behavior

### Circuit States
The circuit breaker implements the standard three-state pattern:

- **Closed (Normal)** - Requests pass through normally, failures are counted
- **Open (Blocking)** - All requests fail immediately without calling the processor
- **Half-Open (Testing)** - Limited requests are allowed to test service recovery

### State Machine Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                  Circuit Breaker State Machine                   │
└──────────────────────────────────────────────────────────────────┘

        ┌─────────────────────────────────────┐
        │            CLOSED                    │
        │         (Normal Operation)           │
        │                                      │
        │  • Requests pass through             │
        │  • Count consecutive failures        │
        │  • Reset count on success            │
        └──────────────┬───────────────────────┘
                       │
            failures >= threshold
                       │
                       ▼
        ┌─────────────────────────────────────┐
        │              OPEN                    │
        │          (Failing Fast)              │
        │                                      │
        │  • All requests fail immediately     │
        │  • No calls to protected service     │
        │  • Wait for reset timeout            │
        └──────────────┬───────────────────────┘
                       │
              after resetTimeout
                       │
                       ▼
        ┌─────────────────────────────────────┐
        │           HALF-OPEN                  │◄──┐
        │      (Testing Recovery)              │   │
        │                                      │   │
        │  • Limited requests allowed          │   │ any failure
        │  • Count successes                   │   │
        │  • Testing service health            │───┘
        └──────────────┬───────────────────────┘
                       │
         successes >= successThreshold
                       │
                       ▼
                  [CLOSED]

State Transition Rules:
═══════════════════════
CLOSED → OPEN:      After failureThreshold consecutive failures
OPEN → HALF-OPEN:   After resetTimeout duration expires
HALF-OPEN → CLOSED: After successThreshold consecutive successes
HALF-OPEN → OPEN:   On any failure during half-open state
```

### State Transitions
- **Closed → Open** - After `failureThreshold` consecutive failures
- **Open → Half-Open** - After `resetTimeout` duration
- **Half-Open → Closed** - After `successThreshold` consecutive successes
- **Half-Open → Open** - On any failure during half-open state

### Error Handling
- **Error propagation** - Preserves original error information and paths
- **Circuit context** - Adds circuit breaker information to error paths
- **State awareness** - Different errors for open vs processor failures

## Signals

CircuitBreaker emits typed signals at state transitions for observability and monitoring via [capitan](https://github.com/zoobzio/capitan):

| Signal | When Emitted | Fields |
|--------|--------------|--------|
| `circuitbreaker.opened` | Circuit opens after failure threshold reached | `name`, `state`, `failures`, `failure_threshold` |
| `circuitbreaker.closed` | Circuit closes after successful recovery | `name`, `state`, `successes`, `success_threshold` |
| `circuitbreaker.half-open` | Circuit transitions to half-open for testing | `name`, `state`, `generation` |
| `circuitbreaker.rejected` | Request rejected while circuit is open | `name`, `state`, `generation` |

**Example:**

```go
import "github.com/zoobzio/capitan"

// Hook circuit breaker signals
capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    failures, _ := pipz.FieldFailures.From(e)
    // Alert or log circuit opening
})
```

See [Hooks Documentation](../../2.learn/5.hooks.md) for complete signal reference and usage examples.

## Configuration Methods

```go
// Runtime configuration
breaker.SetFailureThreshold(10)               // Change failure threshold
breaker.SetSuccessThreshold(3)                // Successes needed to close from half-open
breaker.SetResetTimeout(time.Minute)          // Change recovery timeout

// State management
state := breaker.GetState()                   // "closed", "open", or "half-open"
breaker.Reset()                               // Manually reset to closed state

// Getters
failures := breaker.GetFailureThreshold()     // Current failure threshold
successes := breaker.GetSuccessThreshold()    // Current success threshold
timeout := breaker.GetResetTimeout()          // Current reset timeout
```

## Example

```go
// Define identities upfront
var (
    APIBreakerID    = pipz.NewIdentity("api-breaker", "Circuit breaker for external API")
    ExternalAPIID   = pipz.NewIdentity("external-api", "Call external API")
    ResilientAPIID  = pipz.NewIdentity("resilient-api", "Resilient API call pipeline")
    RetryID         = pipz.NewIdentity("retry", "Retry API calls")
)

// Basic circuit breaker - open after 5 failures, try recovery after 30 seconds
breaker := pipz.NewCircuitBreaker(APIBreakerID,
    pipz.Apply(ExternalAPIID, callExternalAPI),
    5,                    // Open after 5 consecutive failures
    30*time.Second,       // Try recovery after 30 seconds
)

// Use in a resilient pipeline
resilientAPI := pipz.NewSequence(ResilientAPIID,
    breaker,
    pipz.NewRetry(RetryID, apiCall, 3),
)

// Runtime configuration
breaker.SetFailureThreshold(10)               // More tolerant during peak hours
breaker.SetSuccessThreshold(3)                // Need 3 successes to fully recover

// Monitoring circuit state
if breaker.GetState() == "open" {
    log.Warn("Circuit breaker is open - service may be down")
}

// Manual intervention
if emergencyRecovery {
    breaker.Reset()  // Force reset during maintenance
}
```

## When to Use

Use `CircuitBreaker` when:
- Calling **external services that may fail** (APIs, databases, microservices)
- Preventing cascade failures in distributed systems
- Protecting against downstream service degradation
- Giving failing services time to recover
- Implementing fast failure for better user experience
- Reducing load on struggling services

Use with **low thresholds** (3-5) when:
- Services fail completely rather than gradually
- Fast failure is more important than trying
- You have good fallback mechanisms

Use with **higher thresholds** (10-20) when:
- Services have intermittent issues
- Temporary failures are common
- You want to be tolerant of occasional errors

## When NOT to Use

Don't use `CircuitBreaker` when:
- Calling internal services that should always work
- Failures are permanent (validation errors, business logic)
- You just need retries (use `Retry` - simpler)
- The service has no failure patterns
- Every request is unique and independent

## Error Messages

CircuitBreaker provides detailed error information:

```go
var PaymentBreakerID = pipz.NewIdentity("payment-breaker", "Circuit breaker for payment processing")
breaker := pipz.NewCircuitBreaker(PaymentBreakerID, paymentProcessor, 3, time.Minute)

_, err := breaker.Process(ctx, payment)
if err != nil {
    var pipeErr *pipz.Error[Payment]
    if errors.As(err, &pipeErr) {
        // Error path shows circuit breaker involvement
        // Example: "payment-pipeline → payment-breaker → payment-processor failed after 1.2s: connection timeout"
        // Or: "payment-breaker failed after 0s: circuit breaker is open"
        
        if strings.Contains(err.Error(), "circuit breaker is open") {
            // Handle open circuit differently
            log.Info("Payment service circuit is open, using fallback")
        }
    }
}
```

## Common Patterns

```go
// Define identities upfront
var (
    DBBreakerID      = pipz.NewIdentity("db-breaker", "Circuit breaker for database operations")
    ExecuteQueryID   = pipz.NewIdentity("execute-query", "Execute database query")
)

// Database operations with circuit breaker
dbConnection := pipz.NewCircuitBreaker(DBBreakerID,
    pipz.Apply(ExecuteQueryID, runDatabaseQuery),
    5,                    // Open after 5 database failures
    time.Minute,          // Try reconnection after 1 minute
)

// HTTP client with multiple protection layers
var (
    ProtectedHTTPID  = pipz.NewIdentity("protected-http", "HTTP client with protection layers")
    RequestTimeoutID = pipz.NewIdentity("request-timeout", "Request timeout wrapper")
    HTTPBreakerID    = pipz.NewIdentity("http-breaker", "HTTP circuit breaker")
    HTTPRetryID      = pipz.NewIdentity("http-retry", "HTTP retry handler")
    HTTPCallID       = pipz.NewIdentity("http-call", "HTTP request call")
)

resilientHTTP := pipz.NewSequence(ProtectedHTTPID,
    pipz.NewTimeout(RequestTimeoutID,
        pipz.NewCircuitBreaker(HTTPBreakerID,
            pipz.NewRetry(HTTPRetryID,
                pipz.Apply(HTTPCallID, makeHTTPRequest),
                3,
            ),
            10,                // Open after 10 failures
            2*time.Minute,     // Try recovery after 2 minutes
        ),
        30*time.Second,        // Overall timeout
    ),
)

// Service mesh pattern
var (
    ServiceMeshID     = pipz.NewIdentity("service-mesh", "Service mesh with fallback")
    PrimaryServiceID  = pipz.NewIdentity("primary-service", "Primary service circuit breaker")
    SecondaryServiceID = pipz.NewIdentity("secondary-service", "Secondary service circuit breaker")
)

serviceCall := pipz.NewFallback(ServiceMeshID,
    pipz.NewCircuitBreaker(PrimaryServiceID,
        primaryServiceCall,
        5, 30*time.Second,
    ),
    pipz.NewCircuitBreaker(SecondaryServiceID,
        secondaryServiceCall,
        3, time.Minute,
    ),
)

// Microservice with graceful degradation
var (
    UserServiceID  = pipz.NewIdentity("user-service", "User service router")
    FullServiceID  = pipz.NewIdentity("full-service", "Full user service circuit breaker")
    BasicServiceID = pipz.NewIdentity("basic-service", "Basic user service circuit breaker")
)

userService := pipz.NewSwitch(UserServiceID, checkServiceHealth).
    AddRoute("healthy",
        pipz.NewCircuitBreaker(FullServiceID,
            fullUserService,
            5, time.Minute,
        ),
    ).
    AddRoute("degraded",
        pipz.NewCircuitBreaker(BasicServiceID,
            basicUserService,
            10, 30*time.Second,  // More tolerant in degraded mode
        ),
    )
```

## Gotchas

### ❌ Don't create circuit breakers per request
```go
// WRONG - New breaker each time, no shared state!
func handleRequest(req Request) Response {
    breakerID := pipz.NewIdentity("api", "API circuit breaker")
    breaker := pipz.NewCircuitBreaker(breakerID, apiCall, 5, time.Minute)
    return breaker.Process(ctx, req) // Useless! New Identity each call
}
```

### ✅ Create once, reuse
```go
// RIGHT - Shared state across requests with package-level Identity
var APIBreakerID = pipz.NewIdentity("api", "API circuit breaker")
var apiBreaker = pipz.NewCircuitBreaker(APIBreakerID, apiCall, 5, time.Minute)

func handleRequest(req Request) Response {
    return apiBreaker.Process(ctx, req)
}
```

### ❌ Don't use for permanent errors
```go
// WRONG - Validation errors aren't transient
var (
    ValidationBreakerID = pipz.NewIdentity("validation", "Validation circuit breaker")
    ValidateID          = pipz.NewIdentity("validate", "Validate data")
)

breaker := pipz.NewCircuitBreaker(ValidationBreakerID,
    pipz.Apply(ValidateID, validateData), // Always fails for bad data
    3, time.Minute,
)
```

### ✅ Only protect transient failures
```go
// RIGHT - Network calls can recover
var (
    NetworkBreakerID = pipz.NewIdentity("network", "Network circuit breaker")
    APIID            = pipz.NewIdentity("api", "API call")
)

breaker := pipz.NewCircuitBreaker(NetworkBreakerID,
    pipz.Apply(APIID, callExternalAPI),
    5, time.Minute,
)
```

## Advanced Patterns

```go
// Circuit breaker with custom recovery logic
var (
    SmartBreakerID    = pipz.NewIdentity("smart-breaker", "Smart circuit breaker with recovery")
    CircuitID         = pipz.NewIdentity("circuit", "Inner circuit breaker")
    RecoveryHandlerID = pipz.NewIdentity("recovery-handler", "Error recovery router")
    NotifyOpsID       = pipz.NewIdentity("notify-ops", "Notify operations team")
    LogErrorID        = pipz.NewIdentity("log-error", "Log error")
)

smartBreaker := pipz.NewHandle(SmartBreakerID,
    pipz.NewCircuitBreaker(CircuitID,
        riskyOperation,
        5, time.Minute,
    ),
    pipz.NewSwitch(RecoveryHandlerID,
        func(ctx context.Context, err *pipz.Error[Data]) string {
            if strings.Contains(err.Err.Error(), "circuit breaker is open") {
                return "circuit-open"
            }
            return "other-error"
        },
    ).
    AddRoute("circuit-open",
        pipz.Effect(NotifyOpsID, notifyOperations),
    ).
    AddRoute("other-error",
        pipz.Effect(LogErrorID, logError),
    ),
)

// Multi-tier circuit breaking
var (
    TieredProtectionID = pipz.NewIdentity("tiered-protection", "Multi-tier circuit breaking")
    ServiceBreakerID   = pipz.NewIdentity("service-breaker", "Service-level circuit breaker")
    EndpointBreakerID  = pipz.NewIdentity("endpoint-breaker", "Endpoint-level circuit breaker")
)

tieredBreaker := pipz.NewSequence(TieredProtectionID,
    pipz.NewCircuitBreaker(ServiceBreakerID,     // Service-level protection
        pipz.NewCircuitBreaker(EndpointBreakerID, // Endpoint-level protection
            endpointCall,
            3, 30*time.Second,
        ),
        10, 2*time.Minute,
    ),
)

// Circuit breaker with metrics
type MetricsCircuitBreaker[T any] struct {
    breaker *pipz.CircuitBreaker[T]
    metrics MetricsCollector
}

func (m *MetricsCircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
    state := m.breaker.GetState()
    m.metrics.RecordGauge("circuit.state", stateToFloat(state))
    
    result, err := m.breaker.Process(ctx, data)
    
    if err != nil {
        if strings.Contains(err.Error(), "circuit breaker is open") {
            m.metrics.Increment("circuit.blocked")
        } else {
            m.metrics.Increment("circuit.failures")
        }
    } else {
        m.metrics.Increment("circuit.successes")
    }
    
    return result, err
}

// Adaptive circuit breaker
type AdaptiveCircuitBreaker[T any] struct {
    breaker      *pipz.CircuitBreaker[T]
    errorRate    float64
    requestCount int
    mu           sync.Mutex
}

func (a *AdaptiveCircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
    a.mu.Lock()
    a.requestCount++
    
    // Adjust threshold based on error rate
    if a.requestCount%100 == 0 {
        if a.errorRate > 0.5 {
            a.breaker.SetFailureThreshold(3)  // More sensitive
        } else if a.errorRate < 0.1 {
            a.breaker.SetFailureThreshold(10) // Less sensitive
        }
    }
    a.mu.Unlock()
    
    result, err := a.breaker.Process(ctx, data)
    
    a.mu.Lock()
    if err != nil {
        a.errorRate = a.errorRate*0.9 + 0.1
    } else {
        a.errorRate = a.errorRate * 0.99
    }
    a.mu.Unlock()
    
    return result, err
}

// Circuit breaker with health checks
var (
    HealthAwareID       = pipz.NewIdentity("health-aware", "Health-aware circuit breaker")
    HealthCheckID       = pipz.NewIdentity("health-check", "Service health check")
    ProtectedServiceID  = pipz.NewIdentity("protected-service", "Protected service circuit breaker")
)

healthAwareBreaker := pipz.NewSequence(HealthAwareID,
    pipz.Apply(HealthCheckID, func(ctx context.Context, req Request) (Request, error) {
        if !healthChecker.IsHealthy() {
            return req, errors.New("service unhealthy")
        }
        return req, nil
    }),
    pipz.NewCircuitBreaker(ProtectedServiceID,
        serviceCall,
        5, time.Minute,
    ),
)
```

## State Management

```go
// Monitor circuit state
func monitorCircuit(breaker *pipz.CircuitBreaker[Request]) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        state := breaker.GetState()
        switch state {
        case "open":
            log.Warn("Circuit is open - service may be down")
            // Trigger alerts, health checks, etc.
        case "half-open":
            log.Info("Circuit is half-open - testing service recovery")
        case "closed":
            log.Debug("Circuit is closed - service operating normally")
        }
    }
}

// Coordinated circuit management
type CircuitManager struct {
    circuits map[string]*pipz.CircuitBreaker[any]
    mu       sync.RWMutex
}

func (cm *CircuitManager) GetCircuitStates() map[string]string {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    
    states := make(map[string]string)
    for name, circuit := range cm.circuits {
        states[name] = circuit.GetState()
    }
    return states
}

func (cm *CircuitManager) ResetAllCircuits() {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    
    for name, circuit := range cm.circuits {
        circuit.Reset()
        log.Infof("Reset circuit: %s", name)
    }
}
```

## Performance Characteristics

- **Closed state** - ~67ns per operation, minimal overhead
- **Open state** - ~443ns per operation, fast failure
- **Half-open state** - Similar to closed, with state tracking
- **Memory usage** - Minimal, constant per circuit breaker
- **Thread safety** - Fully concurrent, uses efficient locking

## See Also

- [RateLimiter](./ratelimiter.md) - For controlling request rates
- [Retry](./retry.md) - Often combined with circuit breakers
- [Fallback](./fallback.md) - For alternative processors
- [Timeout](./timeout.md) - For time-based failure detection
- [Handle](../3.processors/handle.md) - For custom error handling patterns