---
title: "WorkerPool"
description: "Bounded parallel execution with a fixed number of workers for controlled resource usage"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - parallel
  - worker-pool
  - resource-control
---

# WorkerPool

Bounded parallel execution with a fixed number of workers.

## Function Signature

```go
func NewWorkerPool[T Cloner[T]](identity Identity, workers int, processors ...Chainable[T]) *WorkerPool[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `identity` (`Identity`) - Identifier with name and description for the connector used in debugging and observability
- `workers` (`int`) - Maximum number of concurrent processors (semaphore size)
- `processors` - Variable number of processors to execute with limited parallelism

## Returns

Returns a `*WorkerPool[T]` that implements `Chainable[T]`.

## Testing Configuration

### WithClock

```go
func (w *WorkerPool[T]) WithClock(clock clockz.Clock) *WorkerPool[T]
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
pool := pipz.NewWorkerPool(
    pipz.NewIdentity("test", "Test worker pool with fake clock for controlled time testing"),
    3,
    processor1,
    processor2,
).WithClock(fakeClock)

// Advance time in test for timeout testing
fakeClock.Advance(5 * time.Second)
```

## Behavior

- **Bounded parallelism** - Limits concurrent execution to worker count
- **Semaphore pattern** - Uses channel-based semaphore for concurrency control
- **Data isolation** - Each processor receives a clone of the input
- **Wait for all** - Waits for all processors to complete
- **Returns original** - Always returns the original input data
- **Context preservation** - Passes original context to all processors
- **Dynamic configuration** - Worker count and processors can be modified at runtime
- **Optional timeout** - Per-task timeout configuration available

## Example

```go
// Define a type that implements Cloner
type APIRequest struct {
    ID       string
    Endpoint string
    Payload  map[string]interface{}
}

func (r APIRequest) Clone() APIRequest {
    payload := make(map[string]interface{}, len(r.Payload))
    for k, v := range r.Payload {
        payload[k] = v
    }
    return APIRequest{
        ID:       r.ID,
        Endpoint: r.Endpoint,
        Payload:  payload,
    }
}

// Define identities upfront
var (
    APIBatchID    = pipz.NewIdentity("api-batch", "Bounded parallel execution limited to 3 concurrent requests")
    ServiceAID    = pipz.NewIdentity("service-a", "Call service A")
    ServiceBID    = pipz.NewIdentity("service-b", "Call service B")
    ServiceCID    = pipz.NewIdentity("service-c", "Call service C")
    ServiceDID    = pipz.NewIdentity("service-d", "Call service D")
    ServiceEID    = pipz.NewIdentity("service-e", "Call service E")
    APIFlowID     = pipz.NewIdentity("api-flow", "API request flow pipeline")
    ValidateID    = pipz.NewIdentity("validate", "Validate API request")
    AuthID        = pipz.NewIdentity("auth", "Add authentication")
)

// Create worker pool with limited concurrency
apiCalls := pipz.NewWorkerPool(APIBatchID,
    3,
    pipz.Apply(ServiceAID, callServiceA),
    pipz.Apply(ServiceBID, callServiceB),
    pipz.Apply(ServiceCID, callServiceC),
    pipz.Apply(ServiceDID, callServiceD),
    pipz.Apply(ServiceEID, callServiceE),
)

// Use in a pipeline
pipeline := pipz.NewSequence[APIRequest](APIFlowID,
    pipz.Apply(ValidateID, validateRequest),
    pipz.Apply(AuthID, addAuthentication),
    apiCalls, // Only 3 API calls run concurrently
)
```

## When to Use

Use `WorkerPool` when:
- You have **many processors but limited resources**
- External services have **rate limits or connection limits**
- You want to **prevent resource exhaustion**
- **Memory or CPU constraints** require bounded parallelism
- Database connection pools have **limited connections**
- You need **predictable resource usage**
- Testing with **controlled concurrency levels**

## When NOT to Use

Don't use `WorkerPool` when:
- You need maximum parallelism (use `Concurrent`)
- Operations must run in order (use `Sequence`)
- Fire-and-forget semantics needed (use `Scaffold`)
- Only have a few processors (overhead not worth it)
- Type doesn't implement `Cloner[T]` (compilation error)

## WorkerPool vs Other Parallel Connectors

| Feature | WorkerPool | Concurrent | Scaffold |
|---------|------------|------------|----------|
| Parallelism | Bounded (N workers) | Unbounded | Unbounded |
| Returns | After all complete | After all complete | Immediately |
| Resource Control | Yes | No | No |
| Use Case | Limited resources | Max parallelism | Fire-and-forget |
| Memory Usage | Predictable | Can spike | Can spike |

## Signals

WorkerPool emits typed signals for worker acquisition and saturation via [capitan](https://github.com/zoobzio/capitan):

| Signal | When Emitted | Fields |
|--------|--------------|--------|
| `workerpool.saturated` | All worker slots occupied, task will block | `name`, `worker_count`, `active_workers` |
| `workerpool.acquired` | Worker slot acquired, task starting | `name`, `worker_count`, `active_workers` |
| `workerpool.released` | Worker slot released, task completed | `name`, `worker_count`, `active_workers` |

**Example:**

```go
import "github.com/zoobzio/capitan"

// Hook worker pool signals
capitan.Hook(pipz.SignalWorkerPoolSaturated, func(ctx context.Context, e *capitan.Event) {
    name, _ := pipz.FieldName.From(e)
    workers, _ := pipz.FieldWorkerCount.From(e)
    // Alert on saturation
})
```

See [Hooks Documentation](../../2.learn/5.hooks.md) for complete signal reference and usage examples.

## Configuration Methods

```go
var PoolID = pipz.NewIdentity("pool", "Worker pool")
pool := pipz.NewWorkerPool(PoolID, 5, processors...)

// Adjust worker count dynamically
pool.SetWorkerCount(10)

// Add timeout per task
pool.WithTimeout(30 * time.Second)

// Add processors dynamically
pool.Add(newProcessor)

// Replace all processors
pool.SetProcessors(proc1, proc2, proc3)

// Query current state
workers := pool.GetWorkerCount()     // Maximum workers
active := pool.GetActiveWorkers()    // Currently active
```

## Performance Characteristics

- Creates one goroutine per processor (up to worker limit)
- Blocked processors wait for available worker slot
- Semaphore acquisition overhead: ~50ns
- Memory usage: O(processors) + cloning cost
- Scales linearly up to worker count, then queues

## Common Patterns

### Rate-Limited API Calls
```go
// Define identities upfront
var (
    APIRateLimitedID = pipz.NewIdentity("api-rate-limited", "Rate-limited API pool with 10 workers")
    FetchUserID      = pipz.NewIdentity("fetch-user", "Fetch user data")
    FetchOrdersID    = pipz.NewIdentity("fetch-orders", "Fetch order history")
    FetchPrefsID     = pipz.NewIdentity("fetch-prefs", "Fetch preferences")
    FetchActivityID  = pipz.NewIdentity("fetch-activity", "Fetch activity log")
)

// Respect API rate limits
apiPool := pipz.NewWorkerPool(APIRateLimitedID,
    10,
    pipz.Apply(FetchUserID, fetchUserData),
    pipz.Apply(FetchOrdersID, fetchOrderHistory),
    pipz.Apply(FetchPrefsID, fetchPreferences),
    pipz.Apply(FetchActivityID, fetchActivityLog),
    // ... many more API calls
).WithTimeout(5 * time.Second)
```

### Database Connection Pool
```go
// Define identities upfront
var (
    DBOpsID        = pipz.NewIdentity("db-ops", "Database operations pool with 5 workers")
    InsertUserID   = pipz.NewIdentity("insert-user", "Insert user to database")
    UpdateProfileID = pipz.NewIdentity("update-profile", "Update user profile")
    LogActivityID  = pipz.NewIdentity("log-activity", "Log user activity")
    UpdateStatsID  = pipz.NewIdentity("update-stats", "Update statistics")
)

// Limit concurrent database operations
dbPool := pipz.NewWorkerPool(DBOpsID,
    5,
    pipz.Apply(InsertUserID, insertUser),
    pipz.Apply(UpdateProfileID, updateProfile),
    pipz.Apply(LogActivityID, logActivity),
    pipz.Apply(UpdateStatsID, updateStatistics),
)
```

### CPU-Intensive Operations
```go
// Define identities upfront
var (
    CPUBoundID       = pipz.NewIdentity("cpu-bound", "CPU-intensive processing pool")
    ResizeImageID    = pipz.NewIdentity("resize-image", "Resize image")
    GenThumbnailID   = pipz.NewIdentity("generate-thumbnail", "Generate thumbnail")
    ExtractMetaID    = pipz.NewIdentity("extract-metadata", "Extract metadata")
    ApplyWatermarkID = pipz.NewIdentity("apply-watermark", "Apply watermark")
)

// Control CPU usage
processing := pipz.NewWorkerPool(CPUBoundID,
    runtime.NumCPU(),
    pipz.Transform(ResizeImageID, resizeImage),
    pipz.Transform(GenThumbnailID, generateThumbnail),
    pipz.Transform(ExtractMetaID, extractMetadata),
    pipz.Transform(ApplyWatermarkID, applyWatermark),
)
```

### Batch Processing with Controlled Concurrency
```go
// Define identities upfront
var (
    BatchFlowID     = pipz.NewIdentity("batch-flow", "Batch processing flow")
    ValidateBatchID = pipz.NewIdentity("validate", "Validate batch")
    ProcessItemsID  = pipz.NewIdentity("process-items", "Batch item enrichment pool with 20 workers")
    Enrich1ID       = pipz.NewIdentity("enrich-1", "Enrich from service 1")
    Enrich2ID       = pipz.NewIdentity("enrich-2", "Enrich from service 2")
    Enrich3ID       = pipz.NewIdentity("enrich-3", "Enrich from service 3")
    AggregateID     = pipz.NewIdentity("aggregate", "Aggregate results")
)

// Process large batches with resource limits
batchProcessor := pipz.NewSequence[Batch](BatchFlowID,
    pipz.Apply(ValidateBatchID, validateBatch),
    pipz.NewWorkerPool(ProcessItemsID,
        20,
        pipz.Apply(Enrich1ID, enrichWithService1),
        pipz.Apply(Enrich2ID, enrichWithService2),
        pipz.Apply(Enrich3ID, enrichWithService3),
        // ... potentially hundreds of items
    ),
    pipz.Apply(AggregateID, aggregateResults),
)
```

## Gotchas

### ❌ Don't create per-request instances
```go
// WRONG - Creates new pool for each request!
func handleRequest(req Request) Response {
    poolID := pipz.NewIdentity("pool", "Worker pool")
    pool := pipz.NewWorkerPool(poolID, 5, processors...)
    return pool.Process(ctx, req) // Defeats the purpose
}
```

### ✅ Use singleton instances
```go
// RIGHT - Package-level Identity and pool shared across requests
var APIID = pipz.NewIdentity("api", "Shared API pool for all requests with 5 worker limit")
var apiPool = pipz.NewWorkerPool(APIID, 5, processors...)

func handleRequest(req Request) Response {
    return apiPool.Process(ctx, req) // Properly limited
}
```

### ❌ Don't forget WorkerPool doesn't transform data
```go
// WRONG - Expecting modified data
var (
    TransformPoolID = pipz.NewIdentity("transform", "Incorrectly expecting transformation results")
    DoubleID        = pipz.NewIdentity("double", "Double the value")
)

pool := pipz.NewWorkerPool(TransformPoolID,
    3,
    pipz.Transform(DoubleID, func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := pool.Process(ctx, 5)
// result is still 5, not 10!
```

### ✅ Use for side effects only
```go
// RIGHT - Side effects, not transformations
var (
    EffectsPoolID = pipz.NewIdentity("effects", "Side effect operations pool")
    SaveDBID      = pipz.NewIdentity("save-db", "Save to database")
    SendEventID   = pipz.NewIdentity("send-event", "Publish event")
    UpdateCacheID = pipz.NewIdentity("update-cache", "Update cache")
)

pool := pipz.NewWorkerPool(EffectsPoolID,
    3,
    pipz.Effect(SaveDBID, saveToDatabase),
    pipz.Effect(SendEventID, publishEvent),
    pipz.Effect(UpdateCacheID, updateCache),
)
```

### ❌ Don't set workers to 0
```go
// WRONG - No workers means nothing runs!
var BrokenID = pipz.NewIdentity("broken", "Misconfigured pool with zero workers")
pool := pipz.NewWorkerPool(BrokenID, 0, processors...)
```

### ✅ Use reasonable worker counts
```go
// RIGHT - Based on actual constraints
var BalancedID = pipz.NewIdentity("balanced", "Balanced worker pool based on resource constraints")
pool := pipz.NewWorkerPool(BalancedID, 10, processors...)
```

## Implementation Requirements

Your type must implement `Clone()` for thread safety:

```go
// Complex type with proper cloning
type Task struct {
    ID         string
    Priority   int
    Data       []byte
    Metadata   map[string]string
    Processors []string
}

func (t Task) Clone() Task {
    // Deep copy slice
    data := make([]byte, len(t.Data))
    copy(data, t.Data)
    
    // Deep copy map
    metadata := make(map[string]string, len(t.Metadata))
    for k, v := range t.Metadata {
        metadata[k] = v
    }
    
    // Deep copy string slice
    processors := make([]string, len(t.Processors))
    copy(processors, t.Processors)
    
    return Task{
        ID:         t.ID,
        Priority:   t.Priority,
        Data:       data,
        Metadata:   metadata,
        Processors: processors,
    }
}
```

## Advanced Usage

### Dynamic Worker Adjustment
```go
// Define identity upfront
var AdaptivePoolID = pipz.NewIdentity("adaptive", "Self-adjusting worker pool that scales based on utilization")

// Adjust workers based on load
pool := pipz.NewWorkerPool(AdaptivePoolID, 5, processors...)

// Monitor and adjust
go func() {
    for {
        active := pool.GetActiveWorkers()
        max := pool.GetWorkerCount()

        utilization := float64(active) / float64(max)
        if utilization > 0.8 {
            pool.SetWorkerCount(max + 5) // Scale up
        } else if utilization < 0.2 && max > 5 {
            pool.SetWorkerCount(max - 5) // Scale down
        }

        time.Sleep(30 * time.Second)
    }
}()
```

### Combining with Circuit Breaker
```go
// Define identities upfront
var (
    ProtectedBreakerID = pipz.NewIdentity("protected", "Circuit breaker for worker pool")
    LimitedPoolID      = pipz.NewIdentity("limited", "Protected API pool with worker limit")
    API1ID             = pipz.NewIdentity("api-1", "Call API 1")
    API2ID             = pipz.NewIdentity("api-2", "Call API 2")
    API3ID             = pipz.NewIdentity("api-3", "Call API 3")
)

// Protect external services with both patterns
protected := pipz.NewCircuitBreaker(ProtectedBreakerID,
    pipz.NewWorkerPool(LimitedPoolID,
        10,
        pipz.Apply(API1ID, callAPI1),
        pipz.Apply(API2ID, callAPI2),
        pipz.Apply(API3ID, callAPI3),
    ),
    5, time.Minute,
)
```

## See Also

- [Concurrent](./concurrent.md) - For unbounded parallel execution
- [Scaffold](../3.processors/scaffold.md) - For fire-and-forget operations
- [Sequence](./sequence.md) - For sequential execution
- [RateLimiter](./ratelimiter.md) - For time-based rate limiting