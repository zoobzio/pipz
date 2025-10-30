# Hooks and Observability

Pipz integrates with [capitan](https://github.com/zoobzio/capitan) to provide type-safe event hooks for observability, monitoring, and debugging. Stateful connectors emit signals at critical state transitions and decision points, allowing you to observe system behavior without modifying your processing logic.

## Overview

Hooks enable you to:
- **Monitor** circuit breaker state changes and timeout events
- **Track** rate limiting behavior and backpressure
- **Observe** worker pool saturation, retry exhaustion, and backoff patterns
- **Detect** when fallback processors are being used
- **Alert** on threshold violations and failure patterns
- **Collect** metrics for dashboards
- **Debug** pipeline behavior in production

All events are emitted asynchronously via per-signal worker goroutines, ensuring hooks don't impact pipeline performance.

## Event Severity

As of capitan v0.0.5, all events include a severity level that indicates their importance:

- **Error**: System failures requiring immediate attention (circuit opened, requests rejected/dropped, all retries exhausted, timeouts)
- **Warn**: Degraded performance or fallback scenarios (circuit half-open, rate limiting throttled, pool saturated, individual retry failures, using fallback processors, backoff delays)
- **Info**: Normal operations (circuit closed, rate limiter allowed, worker acquired/released, retry attempts, using primary processor)
- **Debug**: Detailed operational information (currently unused, but available for verbose logging)

Events can be filtered by severity in your hooks using `e.Severity()`.

## Available Signals

### CircuitBreaker

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `circuitbreaker.opened` | Circuit opens after failure threshold reached | `name`, `state`, `failures`, `failure_threshold` |
| `circuitbreaker.closed` | Circuit closes after successful recovery | `name`, `state`, `successes`, `success_threshold` |
| `circuitbreaker.half-open` | Circuit transitions to half-open for testing | `name`, `state`, `generation` |
| `circuitbreaker.rejected` | Request rejected while circuit is open | `name`, `state`, `generation` |

### RateLimiter

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `ratelimiter.allowed` | Request allowed, token consumed | `name`, `tokens`, `rate`, `burst` |
| `ratelimiter.throttled` | Request waiting for tokens (wait mode) | `name`, `wait_time`, `tokens`, `rate` |
| `ratelimiter.dropped` | Request dropped, no tokens available (drop mode) | `name`, `tokens`, `rate`, `burst`, `mode` |

### WorkerPool

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `workerpool.saturated` | All worker slots occupied, task will block | `name`, `worker_count`, `active_workers` |
| `workerpool.acquired` | Worker slot acquired, task starting | `name`, `worker_count`, `active_workers` |
| `workerpool.released` | Worker slot released, task completed | `name`, `worker_count`, `active_workers` |

### Retry

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `retry.attempt-start` | Starting a retry attempt | `name`, `attempt`, `max_attempts` |
| `retry.attempt-fail` | Retry attempt failed | `name`, `attempt`, `max_attempts`, `error` |
| `retry.exhausted` | All retry attempts exhausted | `name`, `max_attempts`, `error` |

### Fallback

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `fallback.attempt` | Attempting a fallback processor | `name`, `processor_index`, `processor_name` |
| `fallback.failed` | All fallback processors failed | `name`, `error` |

### Timeout

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `timeout.triggered` | Operation exceeded timeout duration | `name`, `duration` |

### Backoff

| Signal | When Emitted | Key Fields |
|--------|--------------|------------|
| `backoff.waiting` | Entering exponential backoff delay | `name`, `attempt`, `max_attempts`, `delay`, `next_delay` |

## Field Reference

All fields use primitive types for easy integration with monitoring systems:

| Field Key | Type | Description |
|-----------|------|-------------|
| `FieldName` | string | Connector instance name |
| `FieldError` | string | Error message |
| `FieldTimestamp` | float64 | Unix timestamp |
| `FieldState` | string | Circuit state: "closed", "open", "half-open" |
| `FieldFailures` | int | Current failure count |
| `FieldSuccesses` | int | Current success count |
| `FieldFailureThreshold` | int | Failures needed to open circuit |
| `FieldSuccessThreshold` | int | Successes needed to close from half-open |
| `FieldResetTimeout` | float64 | Reset timeout in seconds |
| `FieldGeneration` | int | Circuit generation number |
| `FieldLastFailTime` | float64 | Last failure timestamp |
| `FieldRate` | float64 | Requests per second |
| `FieldBurst` | int | Maximum burst capacity |
| `FieldTokens` | float64 | Current available tokens |
| `FieldMode` | string | Rate limiter mode: "wait" or "drop" |
| `FieldWaitTime` | float64 | Wait time in seconds |
| `FieldWorkerCount` | int | Total worker slots |
| `FieldActiveWorkers` | int | Currently active workers |
| `FieldAttempt` | int | Current retry attempt number |
| `FieldMaxAttempts` | int | Maximum retry attempts |
| `FieldProcessorIndex` | int | Fallback processor index |
| `FieldProcessorName` | string | Fallback processor name |
| `FieldDuration` | float64 | Timeout duration in seconds |
| `FieldDelay` | float64 | Current backoff delay in seconds |
| `FieldNextDelay` | float64 | Next backoff delay in seconds |

## Usage Examples

### Basic Hook Registration

```go
import (
    "context"
    "fmt"

    "github.com/zoobzio/capitan"
    "github.com/zoobzio/pipz"
)

func main() {
    // Configure capitan (optional, before any hooks)
    capitan.Configure(capitan.WithBufferSize(64))

    // Hook circuit breaker signals
    capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        failures, _ := pipz.FieldFailures.From(e)
        threshold, _ := pipz.FieldFailureThreshold.From(e)

        fmt.Printf("ALERT: Circuit %s opened (failures=%d, threshold=%d)\n",
            name, failures, threshold)
    })

    // Your pipeline code...

    // Shutdown capitan to drain pending events
    defer capitan.Shutdown()
}
```

### Metrics Collection

```go
import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    circuitState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "pipz_circuit_state",
            Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
        },
        []string{"name"},
    )

    rateLimitDropped = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "pipz_ratelimit_dropped_total",
            Help: "Total requests dropped by rate limiter",
        },
        []string{"name"},
    )
)

func init() {
    prometheus.MustRegister(circuitState, rateLimitDropped)

    // Track circuit state
    capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        circuitState.WithLabelValues(name).Set(2) // open
    })

    capitan.Hook(pipz.SignalCircuitBreakerClosed, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        circuitState.WithLabelValues(name).Set(0) // closed
    })

    capitan.Hook(pipz.SignalCircuitBreakerHalfOpen, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        circuitState.WithLabelValues(name).Set(1) // half-open
    })

    // Track dropped requests
    capitan.Hook(pipz.SignalRateLimiterDropped, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        rateLimitDropped.WithLabelValues(name).Inc()
    })
}
```

### Structured Logging

```go
import (
    "log/slog"
)

func setupHooks() {
    // Log all rate limiter events
    capitan.Hook(pipz.SignalRateLimiterThrottled, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        waitTime, _ := pipz.FieldWaitTime.From(e)
        tokens, _ := pipz.FieldTokens.From(e)

        slog.WarnContext(ctx, "rate limiter throttled",
            "connector", name,
            "wait_seconds", waitTime,
            "tokens_remaining", tokens,
        )
    })

    // Log worker pool saturation
    capitan.Hook(pipz.SignalWorkerPoolSaturated, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        workers, _ := pipz.FieldWorkerCount.From(e)

        slog.WarnContext(ctx, "worker pool saturated",
            "connector", name,
            "worker_count", workers,
        )
    })
}
```

### Alerting

```go
func setupAlerts() {
    // Alert when circuit opens
    capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        failures, _ := pipz.FieldFailures.From(e)

        // Send to alerting system
        sendAlert(Alert{
            Severity: "critical",
            Title:    fmt.Sprintf("Circuit Breaker Opened: %s", name),
            Message:  fmt.Sprintf("Failures reached threshold: %d", failures),
        })
    })

    // Alert when rate limiter starts dropping requests
    capitan.Hook(pipz.SignalRateLimiterDropped, func(ctx context.Context, e *capitan.Event) {
        name, _ := pipz.FieldName.From(e)
        rate, _ := pipz.FieldRate.From(e)

        sendAlert(Alert{
            Severity: "warning",
            Title:    fmt.Sprintf("Rate Limiter Dropping: %s", name),
            Message:  fmt.Sprintf("Capacity exceeded (rate=%.1f/s)", rate),
        })
    })
}
```

### Severity-Based Filtering

```go
// Only process error-level events
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    if e.Severity() != capitan.SeverityError {
        return
    }

    name, _ := pipz.FieldName.From(e)
    log.Printf("ERROR event from %s: %s", name, e.Signal())

    // Send to error tracking system
    sendToErrorTracker(e)
})

// Route events by severity
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    switch e.Severity() {
    case capitan.SeverityError:
        sendToAlertingSystem(e)
    case capitan.SeverityWarn:
        sendToMonitoringDashboard(e)
    case capitan.SeverityInfo:
        sendToMetricsCollector(e)
    case capitan.SeverityDebug:
        sendToDebugLogs(e)
    }
})
```

## Observer Pattern

Use `Observe()` to listen to multiple signals with a single handler:

```go
// Observe all circuit breaker events
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)

    switch e.Signal() {
    case pipz.SignalCircuitBreakerOpened:
        log.Printf("Circuit %s: OPENED", name)
    case pipz.SignalCircuitBreakerClosed:
        log.Printf("Circuit %s: CLOSED", name)
    case pipz.SignalCircuitBreakerHalfOpen:
        log.Printf("Circuit %s: TESTING", name)
    }
},
    pipz.SignalCircuitBreakerOpened,
    pipz.SignalCircuitBreakerClosed,
    pipz.SignalCircuitBreakerHalfOpen,
)

// Or observe ALL signals
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Log everything for debugging
    log.Printf("Event: %s", e.Signal())
})
```

## Performance Considerations

### Asynchronous Processing

All events are processed asynchronously in per-signal worker goroutines. This means:

- ✅ Hooks **never block** pipeline processing
- ✅ Slow handlers don't impact throughput
- ✅ Handler panics are recovered automatically
- ❌ Events may be **buffered** if handlers are slow
- ❌ No guaranteed **delivery** if process crashes

### Buffer Sizing

Configure buffer size based on emission rate:

```go
// Default: 16 events per signal
capitan.Configure(capitan.WithBufferSize(16))

// High-volume: increase buffer
capitan.Configure(capitan.WithBufferSize(128))

// Low-latency: smaller buffer (fails faster if handler is slow)
capitan.Configure(capitan.WithBufferSize(4))
```

If a signal's buffer fills, `Emit()` becomes blocking until the handler catches up.

### Handler Best Practices

1. **Keep handlers fast** - Emit to external queues/channels rather than doing heavy work
2. **Don't block** - Avoid synchronous I/O in handlers
3. **Handle panics** - Capitan recovers, but you should still be defensive
4. **Use context** - Respect cancellation in long-running handlers

```go
// ❌ Bad: Blocking I/O in handler
capitan.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    http.Post("https://alerting.com/api", ...)  // Blocks!
})

// ✅ Good: Queue for async processing
var alertQueue = make(chan Alert, 100)

capitan.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    select {
    case alertQueue <- buildAlert(e):
    default:
        // Queue full, drop (don't block pipeline)
    }
})
```

## Shutdown

Always call `Shutdown()` to drain pending events:

```go
func main() {
    // Setup hooks...

    // Run application...

    // Drain events before exit
    capitan.Shutdown()
}
```

Without `Shutdown()`, buffered events may be lost on process exit.

## Integration with Connectors

### CircuitBreaker

Emits signals on state transitions:

```go
var apiBreaker = pipz.NewCircuitBreaker(
    "api-breaker",
    apiProcessor,
    5,                  // failureThreshold
    30 * time.Second,   // resetTimeout
)

// Hook to track state
capitan.Hook(pipz.SignalCircuitBreakerOpened, trackCircuitState)
capitan.Hook(pipz.SignalCircuitBreakerClosed, trackCircuitState)
```

See [CircuitBreaker reference](../reference/connectors/circuitbreaker.md) for details.

### RateLimiter

Emits signals for throttling and dropping:

```go
var apiLimiter = pipz.NewRateLimiter[Request](
    "api-limiter",
    100,    // rate per second
    10,     // burst
).SetMode("drop")

// Hook to track dropped requests
capitan.Hook(pipz.SignalRateLimiterDropped, trackDrops)
```

See [RateLimiter reference](../reference/connectors/ratelimiter.md) for details.

### WorkerPool

Emits signals for worker acquisition and saturation:

```go
var pool = pipz.NewWorkerPool[Task](
    "worker-pool",
    10,  // worker count
    processors...,
)

// Hook to track saturation
capitan.Hook(pipz.SignalWorkerPoolSaturated, alertOnSaturation)
```

See [WorkerPool reference](../reference/connectors/workerpool.md) for details.

### Retry

Emits signals for retry attempts and exhaustion:

```go
var retryProcessor = pipz.NewRetry(
    "api-retry",
    apiProcessor,
    3,  // maxAttempts
)

// Hook to track retry exhaustion
capitan.Hook(pipz.SignalRetryExhausted, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    err, _ := pipz.FieldError.From(e)
    log.Printf("ALERT: Retry exhausted for %s: %s", name, err)
})
```

See [Retry reference](../reference/connectors/retry.md) for details.

### Fallback

Emits signals when attempting fallback processors:

```go
var fallbackChain = pipz.NewFallback(
    "payment-fallback",
    stripeProcessor,
    paypalProcessor,
    squareProcessor,
)

// Hook to track when fallbacks are used
capitan.Hook(pipz.SignalFallbackAttempt, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    procName, _ := pipz.FieldProcessorName.From(e)
    index, _ := pipz.FieldProcessorIndex.From(e)

    if index > 0 {
        log.Printf("WARNING: Using fallback processor %s[%d]: %s", name, index, procName)
    }
})
```

See [Fallback reference](../reference/connectors/fallback.md) for details.

### Timeout

Emits signals when operations exceed timeout duration:

```go
var apiTimeout = pipz.NewTimeout(
    "api-timeout",
    apiProcessor,
    5 * time.Second,
)

// Hook to track timeout events
capitan.Hook(pipz.SignalTimeoutTriggered, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    duration, _ := pipz.FieldDuration.From(e)
    log.Printf("ALERT: Operation %s timed out after %.2fs", name, duration)
})
```

See [Timeout reference](../reference/connectors/timeout.md) for details.

### Backoff

Emits signals when entering exponential backoff delays:

```go
var backoffProcessor = pipz.NewBackoff(
    "api-backoff",
    apiProcessor,
    5,                  // maxAttempts
    1 * time.Second,    // baseDelay
)

// Hook to track backoff behavior
capitan.Hook(pipz.SignalBackoffWaiting, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    attempt, _ := pipz.FieldAttempt.From(e)
    delay, _ := pipz.FieldDelay.From(e)
    log.Printf("WARNING: %s backing off on attempt %d, waiting %.2fs", name, attempt, delay)
})
```

See [Backoff reference](../reference/connectors/backoff.md) for details.

## Testing with Hooks

### Sync Mode (v0.0.2+)

Use `WithSyncMode()` for deterministic testing without timing dependencies:

```go
func TestCircuitBreakerHooks(t *testing.T) {
    // Configure with sync mode before first use
    capitan.Configure(capitan.WithSyncMode())

    var opened bool

    capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
        opened = true
    })

    // Trigger circuit opening...

    // No waiting needed - sync mode processes immediately
    if !opened {
        t.Error("circuit should have opened")
    }

    capitan.Shutdown()
}
```

**Important**: `Configure()` must be called before any other capitan operations. In tests, each test function should use a fresh process or the default instance will already be initialized.

### Async Mode

For testing async behavior:

```go
func TestCircuitBreakerHooks(t *testing.T) {
    var opened bool
    var mu sync.Mutex

    capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
        mu.Lock()
        opened = true
        mu.Unlock()
    })

    // Trigger circuit opening...

    // Wait for async processing
    time.Sleep(50 * time.Millisecond)

    mu.Lock()
    if !opened {
        t.Error("circuit should have opened")
    }
    mu.Unlock()

    capitan.Shutdown()
}
```

For production code, hooks are for observability, not control flow.

## Further Reading

- [Capitan Documentation](https://github.com/zoobzio/capitan)
- [Architecture Overview](./architecture.md)
- [CircuitBreaker Reference](../reference/connectors/circuitbreaker.md)
- [RateLimiter Reference](../reference/connectors/ratelimiter.md)
- [WorkerPool Reference](../reference/connectors/workerpool.md)
- [Retry Reference](../reference/connectors/retry.md)
- [Fallback Reference](../reference/connectors/fallback.md)
- [Timeout Reference](../reference/connectors/timeout.md)
- [Backoff Reference](../reference/connectors/backoff.md)
