# Bounded Parallelism Patterns

Control resource usage with WorkerPool for predictable parallel processing.

## Problem

You need parallel processing but face constraints:
- External APIs have rate limits
- Database connection pools are limited
- Memory or CPU resources are constrained
- You need predictable resource usage
- Testing requires controlled concurrency

## Solution

Use `WorkerPool` to limit concurrent execution while maintaining parallelism benefits.

## Basic Pattern

```go
// Limit to 5 concurrent operations
pool := pipz.NewWorkerPool[Data]("limited", 5,
    processor1,
    processor2,
    processor3,
    // ... can have many more processors
)
// Only 5 will run at any time
```

## Common Scenarios

### API Rate Limiting

**Problem:** External API allows only 10 concurrent connections.

```go
type APICall struct {
    Endpoint string
    UserID   string
    Data     map[string]interface{}
}

func (a APICall) Clone() APICall {
    data := make(map[string]interface{}, len(a.Data))
    for k, v := range a.Data {
        data[k] = v
    }
    return APICall{
        Endpoint: a.Endpoint,
        UserID:   a.UserID,
        Data:     data,
    }
}

// Create pool respecting API limits
var apiPool = pipz.NewWorkerPool("external-api", 10,
    pipz.Apply("user-data", fetchUserData),
    pipz.Apply("preferences", fetchPreferences),
    pipz.Apply("history", fetchHistory),
    pipz.Apply("recommendations", fetchRecommendations),
    pipz.Apply("notifications", fetchNotifications),
).WithTimeout(30 * time.Second)

// Use in pipeline
pipeline := pipz.NewSequence[APICall]("user-enrichment",
    pipz.Apply("validate", validateRequest),
    apiPool, // Respects 10 connection limit
    pipz.Apply("aggregate", aggregateResponses),
)
```

### Database Connection Management

**Problem:** Database allows only 25 connections, but you have hundreds of operations.

```go
type DBOperation struct {
    Query  string
    Params []interface{}
    Type   string // "read" or "write"
}

func (d DBOperation) Clone() DBOperation {
    params := make([]interface{}, len(d.Params))
    copy(params, d.Params)
    return DBOperation{
        Query:  d.Query,
        Params: params,
        Type:   d.Type,
    }
}

// Separate pools for read and write operations
var (
    readPool = pipz.NewWorkerPool("db-read", 20,
        pipz.Apply("fetch-users", fetchUsers),
        pipz.Apply("fetch-orders", fetchOrders),
        pipz.Apply("fetch-products", fetchProducts),
        // Many more read operations
    )
    
    writePool = pipz.NewWorkerPool("db-write", 5,
        pipz.Apply("update-user", updateUser),
        pipz.Apply("insert-order", insertOrder),
        pipz.Apply("log-event", logEvent),
        // Fewer write connections
    )
)

// Route based on operation type
dbRouter := pipz.NewSwitch[DBOperation]("db-router",
    func(ctx context.Context, op DBOperation) string {
        return op.Type
    },
).
AddRoute("read", readPool).
AddRoute("write", writePool)
```

### Memory-Constrained Processing

**Problem:** Image processing consumes significant memory; unlimited parallelism causes OOM.

```go
type ImageJob struct {
    SourcePath string
    TargetPath string
    Operations []string
}

func (i ImageJob) Clone() ImageJob {
    ops := make([]string, len(i.Operations))
    copy(ops, i.Operations)
    return ImageJob{
        SourcePath: i.SourcePath,
        TargetPath: i.TargetPath,
        Operations: ops,
    }
}

// Limit based on available memory
// Assume each operation uses ~100MB
availableMemory := getAvailableMemory()
maxWorkers := availableMemory / (100 * 1024 * 1024) // 100MB per worker

imageProcessor := pipz.NewWorkerPool("image-processing", maxWorkers,
    pipz.Transform("resize", resizeImage),
    pipz.Transform("crop", cropImage),
    pipz.Transform("watermark", addWatermark),
    pipz.Transform("optimize", optimizeImage),
).WithTimeout(60 * time.Second)
```

### Multi-Service Orchestration

**Problem:** Calling multiple microservices with different rate limits.

```go
type ServiceCall struct {
    Service  string
    Method   string
    Payload  interface{}
    Priority int
}

func (s ServiceCall) Clone() ServiceCall {
    // Clone based on payload type
    return ServiceCall{
        Service:  s.Service,
        Method:   s.Method,
        Payload:  clonePayload(s.Payload),
        Priority: s.Priority,
    }
}

// Different limits per service tier
var (
    criticalServices = pipz.NewWorkerPool("critical", 20,
        pipz.Apply("auth", callAuthService),
        pipz.Apply("payment", callPaymentService),
        pipz.Apply("inventory", callInventoryService),
    )
    
    standardServices = pipz.NewWorkerPool("standard", 10,
        pipz.Apply("recommendations", callRecommendationService),
        pipz.Apply("analytics", callAnalyticsService),
        pipz.Apply("notifications", callNotificationService),
    )
    
    backgroundServices = pipz.NewWorkerPool("background", 5,
        pipz.Apply("reporting", callReportingService),
        pipz.Apply("backup", callBackupService),
        pipz.Apply("cleanup", callCleanupService),
    )
)

// Route by priority
serviceRouter := pipz.NewSwitch[ServiceCall]("service-router",
    func(ctx context.Context, call ServiceCall) string {
        switch call.Priority {
        case 1:
            return "critical"
        case 2:
            return "standard"
        default:
            return "background"
        }
    },
).
AddRoute("critical", criticalServices).
AddRoute("standard", standardServices).
AddRoute("background", backgroundServices)
```

## Advanced Patterns

### Dynamic Worker Adjustment

Adjust worker count based on system load:

```go
type AdaptivePool struct {
    pool        *pipz.WorkerPool[Data]
    minWorkers  int
    maxWorkers  int
    adjustEvery time.Duration
}

func NewAdaptivePool(name string, min, max int) *AdaptivePool {
    return &AdaptivePool{
        pool:        pipz.NewWorkerPool[Data](name, min),
        minWorkers:  min,
        maxWorkers:  max,
        adjustEvery: 30 * time.Second,
    }
}

func (a *AdaptivePool) Start(ctx context.Context) {
    ticker := time.NewTicker(a.adjustEvery)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                a.adjustWorkers()
            }
        }
    }()
}

func (a *AdaptivePool) adjustWorkers() {
    active := a.pool.GetActiveWorkers()
    current := a.pool.GetWorkerCount()
    
    utilization := float64(active) / float64(current)
    
    if utilization > 0.8 && current < a.maxWorkers {
        // Scale up
        newCount := min(current+5, a.maxWorkers)
        a.pool.SetWorkerCount(newCount)
        log.Printf("Scaled up to %d workers", newCount)
    } else if utilization < 0.3 && current > a.minWorkers {
        // Scale down
        newCount := max(current-5, a.minWorkers)
        a.pool.SetWorkerCount(newCount)
        log.Printf("Scaled down to %d workers", newCount)
    }
}
```

### Combining with Circuit Breaker

Protect services with both bounded parallelism and circuit breaking:

```go
// Limit connections AND protect from cascading failures
protectedAPI := pipz.NewCircuitBreaker("api-protection",
    pipz.NewWorkerPool("api-limited", 10,
        pipz.Apply("endpoint-1", callEndpoint1),
        pipz.Apply("endpoint-2", callEndpoint2),
        pipz.Apply("endpoint-3", callEndpoint3),
    ).WithTimeout(5 * time.Second),
    pipz.WithCircuitBreakerThreshold(5),
    pipz.WithCircuitBreakerWindow(30 * time.Second),
)
```

### Priority Queue with Worker Pool

Process high-priority items first:

```go
type PriorityProcessor struct {
    highPriority   *pipz.WorkerPool[Task]
    normalPriority *pipz.WorkerPool[Task]
    lowPriority    *pipz.WorkerPool[Task]
}

func NewPriorityProcessor() *PriorityProcessor {
    return &PriorityProcessor{
        highPriority: pipz.NewWorkerPool[Task]("high", 10,
            // High priority processors
        ),
        normalPriority: pipz.NewWorkerPool[Task]("normal", 5,
            // Normal priority processors
        ),
        lowPriority: pipz.NewWorkerPool[Task]("low", 2,
            // Low priority processors
        ),
    }
}

func (p *PriorityProcessor) Process(ctx context.Context, task Task) (Task, error) {
    switch task.Priority {
    case "high":
        return p.highPriority.Process(ctx, task)
    case "normal":
        return p.normalPriority.Process(ctx, task)
    default:
        return p.lowPriority.Process(ctx, task)
    }
}
```

## Testing with Worker Pools

Control concurrency for deterministic tests:

```go
func TestConcurrentOperations(t *testing.T) {
    // Use single worker for deterministic behavior
    pool := pipz.NewWorkerPool[TestData]("test", 1,
        pipz.Apply("op1", operation1),
        pipz.Apply("op2", operation2),
        pipz.Apply("op3", operation3),
    )
    
    // Operations run sequentially despite being in pool
    result, err := pool.Process(context.Background(), testData)
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}

func BenchmarkWorkerPoolScaling(b *testing.B) {
    for workers := 1; workers <= 16; workers *= 2 {
        b.Run(fmt.Sprintf("workers-%d", workers), func(b *testing.B) {
            pool := pipz.NewWorkerPool[Data]("bench", workers,
                // Add processors
            )
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                pool.Process(context.Background(), benchData)
            }
        })
    }
}
```

## Common Pitfalls

### ❌ Creating pools per request
```go
// WRONG - Defeats the purpose
func handleRequest(req Request) {
    pool := pipz.NewWorkerPool("pool", 5, processors...)
    return pool.Process(ctx, req)
}
```

### ✅ Use singleton pools
```go
// RIGHT - Shared pool
var pool = pipz.NewWorkerPool("pool", 5, processors...)

func handleRequest(req Request) {
    return pool.Process(ctx, req)
}
```

### ❌ Too few workers
```go
// WRONG - Essentially sequential
pool := pipz.NewWorkerPool("tiny", 1, 
    proc1, proc2, proc3, proc4, proc5) // Bottleneck!
```

### ✅ Balance workers with workload
```go
// RIGHT - Appropriate for workload
pool := pipz.NewWorkerPool("balanced", 10, 
    proc1, proc2, proc3, proc4, proc5) // Good parallelism
```

## When to Use WorkerPool

✅ **Use WorkerPool when:**
- External services have connection limits
- Memory or CPU constraints exist
- You need predictable resource usage
- Testing requires controlled concurrency
- Database connection pools are limited

❌ **Don't use WorkerPool when:**
- You need maximum parallelism (use Concurrent)
- Operations must run sequentially (use Sequence)
- Fire-and-forget is needed (use Scaffold)
- You only have a few processors

## See Also

- [Concurrent](../reference/connectors/concurrent.md) - Unbounded parallelism
- [Scaffold](../reference/processors/scaffold.md) - Fire-and-forget execution
- [RateLimiter](../reference/connectors/ratelimiter.md) - Time-based rate limiting
- [Resilient API Calls](./resilient-api-calls.md) - Complete API resilience patterns