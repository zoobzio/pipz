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
func NewWorkerPool[T Cloner[T]](name Name, workers int, processors ...Chainable[T]) *WorkerPool[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
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
pool := pipz.NewWorkerPool("test", 3, processor1, processor2).
    WithClock(fakeClock)

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

// Create worker pool with limited concurrency
apiCalls := pipz.NewWorkerPool("api-batch", 3,
    pipz.Apply("service-a", callServiceA),
    pipz.Apply("service-b", callServiceB),
    pipz.Apply("service-c", callServiceC),
    pipz.Apply("service-d", callServiceD),
    pipz.Apply("service-e", callServiceE),
)

// Use in a pipeline
pipeline := pipz.NewSequence[APIRequest]("api-flow",
    pipz.Apply("validate", validateRequest),
    pipz.Apply("auth", addAuthentication),
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
pool := pipz.NewWorkerPool("pool", 5, processors...)

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
// Respect API rate limits
apiPool := pipz.NewWorkerPool("api-rate-limited", 10,
    pipz.Apply("fetch-user", fetchUserData),
    pipz.Apply("fetch-orders", fetchOrderHistory),
    pipz.Apply("fetch-prefs", fetchPreferences),
    pipz.Apply("fetch-activity", fetchActivityLog),
    // ... many more API calls
).WithTimeout(5 * time.Second)
```

### Database Connection Pool
```go
// Limit concurrent database operations
dbPool := pipz.NewWorkerPool("db-ops", 5,
    pipz.Apply("insert-user", insertUser),
    pipz.Apply("update-profile", updateProfile),
    pipz.Apply("log-activity", logActivity),
    pipz.Apply("update-stats", updateStatistics),
)
```

### CPU-Intensive Operations
```go
// Control CPU usage
processing := pipz.NewWorkerPool("cpu-bound", runtime.NumCPU(),
    pipz.Transform("resize-image", resizeImage),
    pipz.Transform("generate-thumbnail", generateThumbnail),
    pipz.Transform("extract-metadata", extractMetadata),
    pipz.Transform("apply-watermark", applyWatermark),
)
```

### Batch Processing with Controlled Concurrency
```go
// Process large batches with resource limits
batchProcessor := pipz.NewSequence[Batch]("batch-flow",
    pipz.Apply("validate", validateBatch),
    pipz.NewWorkerPool("process-items", 20,
        pipz.Apply("enrich-1", enrichWithService1),
        pipz.Apply("enrich-2", enrichWithService2),
        pipz.Apply("enrich-3", enrichWithService3),
        // ... potentially hundreds of items
    ),
    pipz.Apply("aggregate", aggregateResults),
)
```

## Gotchas

### ❌ Don't create per-request instances
```go
// WRONG - Creates new pool for each request!
func handleRequest(req Request) Response {
    pool := pipz.NewWorkerPool("pool", 5, processors...)
    return pool.Process(ctx, req) // Defeats the purpose
}
```

### ✅ Use singleton instances
```go
// RIGHT - Shared pool across requests
var apiPool = pipz.NewWorkerPool("api", 5, processors...)

func handleRequest(req Request) Response {
    return apiPool.Process(ctx, req) // Properly limited
}
```

### ❌ Don't forget WorkerPool doesn't transform data
```go
// WRONG - Expecting modified data
pool := pipz.NewWorkerPool("transform", 3,
    pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := pool.Process(ctx, 5)
// result is still 5, not 10!
```

### ✅ Use for side effects only
```go
// RIGHT - Side effects, not transformations
pool := pipz.NewWorkerPool("effects", 3,
    pipz.Effect("save-db", saveToDatabase),
    pipz.Effect("send-event", publishEvent),
    pipz.Effect("update-cache", updateCache),
)
```

### ❌ Don't set workers to 0
```go
// WRONG - No workers means nothing runs!
pool := pipz.NewWorkerPool("broken", 0, processors...)
```

### ✅ Use reasonable worker counts
```go
// RIGHT - Based on actual constraints
pool := pipz.NewWorkerPool("balanced", 10, processors...)
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
// Adjust workers based on load
pool := pipz.NewWorkerPool("adaptive", 5, processors...)

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
// Protect external services with both patterns
protected := pipz.NewCircuitBreaker("protected",
    pipz.NewWorkerPool("limited", 10,
        pipz.Apply("api-1", callAPI1),
        pipz.Apply("api-2", callAPI2),
        pipz.Apply("api-3", callAPI3),
    ),
    pipz.WithCircuitBreakerThreshold(5),
)
```

## See Also

- [Concurrent](./concurrent.md) - For unbounded parallel execution
- [Scaffold](../3.processors/scaffold.md) - For fire-and-forget operations
- [Sequence](./sequence.md) - For sequential execution
- [RateLimiter](./ratelimiter.md) - For time-based rate limiting