# Connector Selection Guide

Quick decisions for choosing the right connector.

## Decision Tree

```
What do you need?
│
├─ Sequential processing? → Sequence
│
├─ Parallel processing?
│   ├─ Need all results? → Concurrent
│   ├─ Bounded parallelism? → WorkerPool
│   ├─ Fire and forget? → Scaffold
│   ├─ Need fastest? → Race
│   └─ Need best match? → Contest
│
├─ Conditional routing? → Switch
│
├─ Error handling?
│   ├─ Have fallback? → Fallback
│   └─ Transient errors? → Retry
│
└─ Resilience?
    ├─ Prevent cascading failures? → CircuitBreaker
    ├─ Control throughput? → RateLimiter
    └─ Bound execution time? → Timeout
```

## Connector Comparison Matrix

```
┌────────────────┬──────────┬────────────┬──────────┬─────────────────────┐
│   Connector    │ Parallel │ All Run?   │ Returns  │ Primary Use Case    │
├────────────────┼──────────┼────────────┼──────────┼─────────────────────┤
│ Sequence       │    No    │ Until fail │ Last     │ Step-by-step flow   │
│ Concurrent     │   Yes    │    Yes     │ Original │ Side effects        │
│ WorkerPool     │   Yes*   │    Yes     │ Original │ Bounded parallelism │
│ Scaffold       │   Yes    │    Yes     │ Original │ Fire-and-forget     │
│ Race           │   Yes    │ First wins │ First    │ Fastest response    │
│ Contest        │   Yes    │ Until pass │ Matching │ Quality threshold   │
│ Switch         │    No    │ One branch │ Selected │ Conditional routing │
│ Fallback       │    No    │ On failure │ Primary  │ Error recovery      │
│ Retry          │    No    │ Until pass │ Success  │ Transient failures  │
│ CircuitBreaker │    No    │ If closed  │ Result   │ Cascade prevention  │
│ RateLimiter    │    No    │ If allowed │ Result   │ Throughput control  │
│ Timeout        │    No    │ Time bound │ Result   │ Execution limits    │
└────────────────┴──────────┴────────────┴──────────┴─────────────────────┘

Legend:
• Parallel: Whether processors run concurrently (*WorkerPool limits concurrency)
• All Run?: Whether all processors execute or stop early
• Returns: What data is returned to caller
• Primary Use Case: Main scenario for using this connector
```

## Problem-Solution Guide

### You need to: Process data through multiple steps in order

**Solution:** `Sequence`

```go
pipeline := pipz.NewSequence[T]("pipeline", step1, step2, step3)
```

**When to use:**
- Order matters
- Each step depends on previous
- Building up state through transformations

**Don't use when:**
- Steps are independent (use Concurrent instead)

---

### You need to: Run multiple operations in parallel

**Solution:** `Concurrent`

```go
concurrent := pipz.NewConcurrent[T]("parallel", proc1, proc2, proc3)
```

**When to use:**
- Operations are independent
- Running side effects (notifications, logging)
- Want to parallelize for performance

**Requirements:**
- Type T must implement `Cloner[T]`

**Don't use when:**
- Order matters
- Operations depend on each other

---

### You need to: Run parallel operations with limited resources

**Solution:** `WorkerPool`

```go
pool := pipz.NewWorkerPool[T]("limited", 3, proc1, proc2, proc3, proc4, proc5)
```

**When to use:**
- Resource-constrained environments
- Rate-limited external services
- Controlled database connections
- Preventing memory exhaustion
- Managing CPU-intensive operations

**Requirements:**
- Type T must implement `Cloner[T]`

**Don't use when:**
- Need unbounded parallelism (use `Concurrent`)
- Operations must complete in order (use `Sequence`)
- Fire-and-forget semantics needed (use `Scaffold`)

---

### You need to: Get the fastest result from multiple sources

**Solution:** `Race`

```go
race := pipz.NewRace[T]("fastest", primary, backup1, backup2)
```

**When to use:**
- Multiple sources for same data
- Want lowest latency
- Have fallback options

**Requirements:**
- Type T must implement `Cloner[T]`

**Don't use when:**
- Need all results
- Sources have different costs

---

### You need to: Find first result meeting quality criteria

**Solution:** `Contest`

```go
contest := pipz.NewContest[T]("best",
    func(ctx context.Context, result T) bool {
        return result.Score > 0.9
    },
    model1, model2, model3,
)
```

**When to use:**
- Quality matters more than speed
- Have multiple approaches
- Want first acceptable result

**Requirements:**
- Type T must implement `Cloner[T]`

---

### You need to: Route data based on conditions

**Solution:** `Switch`

```go
switch := pipz.NewSwitch[T]("router",
    func(ctx context.Context, data T) string {
        if data.Premium {
            return "premium"
        }
        return "standard"
    },
).
AddRoute("premium", premiumPipeline).
AddRoute("standard", standardPipeline)
```

**When to use:**
- Different processing for different data types
- Conditional logic
- A/B testing

**Don't use when:**
- All data follows same path

---

### You need to: Recover from errors gracefully

**Solution:** `Fallback`

```go
fallback := pipz.NewFallback[T]("safe", riskyOperation, safeDefault)
```

**When to use:**
- Have a safe default
- Want graceful degradation
- Errors are expected

**Don't use when:**
- Errors should stop processing
- No reasonable fallback exists

---

### You need to: Retry failed operations

**Solution:** `Retry`

```go
retry := pipz.NewRetry[T]("reliable", processor, 3)
```

**When to use:**
- Transient errors (network, temporary unavailability)
- External service calls
- Database operations

**Don't use when:**
- Errors are permanent (validation failures)
- No backoff needed (can overwhelm service)

---

### You need to: Prevent cascading failures

**Solution:** `CircuitBreaker`

```go
breaker := pipz.NewCircuitBreaker[T]("protected", processor,
    pipz.WithCircuitBreakerThreshold(5),
    pipz.WithCircuitBreakerWindow(time.Minute),
)
```

**When to use:**
- Calling external services
- Protecting downstream systems
- Failing fast is acceptable

**Don't use when:**
- Every request must be attempted
- Failures are independent

---

### You need to: Control processing rate

**Solution:** `RateLimiter`

```go
limiter := pipz.NewRateLimiter[T]("throttled", processor,
    pipz.WithRateLimiterRate(100),
    pipz.WithRateLimiterPeriod(time.Second),
)
```

**When to use:**
- API rate limits
- Resource protection
- Cost control

**Important:**
- Must use singleton instance (don't create per request)

---

### You need to: Bound execution time

**Solution:** `Timeout`

```go
timeout := pipz.NewTimeout[T]("bounded", processor, 5*time.Second)
```

**When to use:**
- Network operations
- User-facing APIs
- SLA requirements

**Don't use when:**
- Operations must complete
- Time is unpredictable

## Quick Comparison

| Connector | Parallel? | Can Fail? | Needs Clone? | Stateful? |
|-----------|-----------|-----------|--------------|-----------|
| Sequence | No | Yes | No | No |
| Concurrent | Yes | Yes | Yes | No |
| WorkerPool | Limited | Yes | Yes | No |
| Scaffold | Yes | No** | Yes | No |
| Race | Yes | Yes | Yes | No |
| Contest | Yes | Yes | Yes | No |
| Switch | No | Yes | No | No |
| Fallback | No | No* | No | No |
| Retry | No | Yes | No | Yes |
| CircuitBreaker | No | Yes | No | Yes |
| RateLimiter | No | Yes | No | Yes |
| Timeout | No | Yes | No | No |

*Fallback always returns a value (uses fallback on error)
**Scaffold errors are not reported back

## Common Combinations

### Resilient External API Call
```go
api := pipz.NewRateLimiter("rate",
    pipz.NewCircuitBreaker("breaker",
        pipz.NewTimeout("timeout",
            pipz.NewRetry("retry", apiCall, 3),
            5*time.Second,
        ),
    ),
)
```

### Multi-Source with Fallback
```go
fetch := pipz.NewFallback("fetch",
    pipz.NewRace[T]("sources", primary, secondary),
    staticDefault,
)
```

### Conditional Parallel Processing
```go
router := pipz.NewSwitch[T]("router", routeFunc).
    AddRoute("batch", pipz.NewConcurrent[T]("batch", processors...)).
    AddRoute("sequential", pipz.NewSequence[T]("seq", processors...))
```

### Resource-Constrained Processing
```go
// Limit concurrent API calls to avoid rate limits
apiCalls := pipz.NewWorkerPool[T]("api-limited", 5,
    pipz.Apply("service-a", callServiceA),
    pipz.Apply("service-b", callServiceB),
    pipz.Apply("service-c", callServiceC),
    pipz.Apply("service-d", callServiceD),
    pipz.Apply("service-e", callServiceE),
    pipz.Apply("service-f", callServiceF),
    // Only 5 will run concurrently
)
```