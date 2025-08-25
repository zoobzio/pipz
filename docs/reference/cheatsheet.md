# pipz Quick Reference

## Processor Selection (10 seconds)

| If you need to... | Use... | Can fail? | Example |
|-------------------|--------|-----------|---------|
| Transform data (pure) | `Transform` | No | `strings.ToUpper` |
| Transform data (can fail) | `Apply` | Yes | Parse JSON, validate |
| Side effects only | `Effect` | Yes | Logging, metrics |
| Conditional changes | `Mutate` | No | Feature flags |
| Optional enhancement | `Enrich` | Logs errors | Add metadata |
| Pass/block data | `Filter` | No | Access control |

## Connector Decision (10 seconds)

```
Need parallel? ──No──→ Need conditions? ──No──→ Sequence
      │                       │
      │                      Yes → Switch
      │
     Yes → Bounded? ──Yes──→ WorkerPool
            │
           No → Need all results? ──No──→ Fastest? → Race
                      │                        │
                      │                       Best? → Contest
                      │
                     Yes → Fire & forget? ──Yes──→ Scaffold
                                 │
                                No → Concurrent
```

## Common Patterns (Copy & Paste)

### Basic Pipeline
```go
pipeline := pipz.NewSequence[T]("name",
    pipz.Apply("validate", validateFunc),
    pipz.Transform("normalize", normalizeFunc),
    pipz.Effect("log", logFunc),
)
```

### Retry with Backoff
```go
reliable := pipz.NewRetry("api", apiCall, 3)
```

### Circuit Breaker
```go
protected := pipz.NewCircuitBreaker("service", processor,
    pipz.WithCircuitBreakerThreshold(5),
    pipz.WithCircuitBreakerWindow(time.Minute),
)
```

### Rate Limiting (Singleton!)
```go
var limiter = pipz.NewRateLimiter("api", processor,
    pipz.WithRateLimiterRate(100),
    pipz.WithRateLimiterPeriod(time.Second),
)
```

### Timeout Protection
```go
bounded := pipz.NewTimeout("slow", processor, 5*time.Second)
```

### Fallback on Error
```go
safe := pipz.NewFallback("safe", riskyOp, safeDefault)
```

### Parallel Processing
```go
// Unbounded - Type must implement Cloner[T]
parallel := pipz.NewConcurrent[T]("notify",
    sendEmail, sendSMS, logEvent,
)

// Bounded - limit to N concurrent operations
pool := pipz.NewWorkerPool[T]("limited", 5,
    apiCall1, apiCall2, apiCall3, // ... many more
)
```

### Conditional Routing
```go
router := pipz.NewSwitch[T]("route", routeFunc).
    AddRoute("premium", premiumPipeline).
    AddRoute("standard", standardPipeline)
```

### Resilient API Call
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

### Error Pipeline
```go
errorHandler := pipz.NewSequence[*pipz.Error[T]]("errors",
    pipz.Effect("log", logError),
    pipz.Apply("classify", classifyError),
    pipz.Switch("recover", selectRecovery),
)
```

## Clone Implementation

```go
type Data struct {
    Items []Item
    Meta  map[string]string
}

func (d Data) Clone() Data {
    // Deep copy slices
    items := make([]Item, len(d.Items))
    copy(items, d.Items)
    
    // Deep copy maps
    meta := make(map[string]string, len(d.Meta))
    for k, v := range d.Meta {
        meta[k] = v
    }
    
    return Data{Items: items, Meta: meta}
}
```

## Error Handling

### Access Error Details
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[T]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Failed at: %s\n", pipeErr.Stage)
        fmt.Printf("Cause: %v\n", pipeErr.Cause)
        fmt.Printf("Data state: %+v\n", pipeErr.State)
    }
}
```

### Handle Specific Errors
```go
pipz.Handle("recover", pipeline,
    func(ctx context.Context, data T, err error) (T, error) {
        if errors.Is(err, ErrTemporary) {
            return getDefault(), nil // Recover
        }
        return data, err // Propagate
    },
)
```

## Testing Patterns

### Mock Processor
```go
type Mock[T any] struct {
    Returns T
    Error   error
}

func (m *Mock[T]) Process(ctx context.Context, data T) (T, error) {
    return m.Returns, m.Error
}

func (m *Mock[T]) Name() string { return "mock" }
```

### Test Error Location
```go
_, err := pipeline.Process(ctx, data)
var pipeErr *pipz.Error[T]
if errors.As(err, &pipeErr) {
    assert.Equal(t, "expected-stage", pipeErr.Stage)
}
```

## Gotchas & Tips

### ❌ Common Mistakes

1. **Creating rate limiters per request**
```go
// WRONG - New instance each time
func handle(req Request) {
    limiter := pipz.NewRateLimiter(...) // ❌
}
```

2. **Shallow copying in Clone()**
```go
// WRONG - Shares slice memory
func (d Data) Clone() Data {
    return Data{Items: d.Items} // ❌
}
```

3. **Not checking context in long operations**
```go
// WRONG - Ignores cancellation
func process(ctx context.Context, data T) (T, error) {
    time.Sleep(10 * time.Second) // ❌
    return data, nil
}
```

### ✅ Best Practices

1. **Singleton rate limiters**
```go
// RIGHT - Shared instance
var limiter = pipz.NewRateLimiter(...) // ✅
```

2. **Deep copy in Clone()**
```go
// RIGHT - New memory
func (d Data) Clone() Data {
    items := make([]Item, len(d.Items))
    copy(items, d.Items) // ✅
    return Data{Items: items}
}
```

3. **Respect context**
```go
// RIGHT - Cancellable
func process(ctx context.Context, data T) (T, error) {
    select {
    case <-time.After(10 * time.Second):
        return data, nil
    case <-ctx.Done():
        return data, ctx.Err() // ✅
    }
}
```

## Type Constraints

| Connector | Requires |
|-----------|----------|
| Concurrent | `T` implements `Cloner[T]` |
| Race | `T` implements `Cloner[T]` |
| Contest | `T` implements `Cloner[T]` |
| All others | Any type `T` |

## Performance Tips

- **Sequence**: O(n) - minimize processor count
- **Concurrent**: Overhead from goroutines - use for expensive operations
- **Transform**: Zero allocations - prefer over Apply when possible
- **Effect**: Use for metrics/logging without data copy
- **Switch**: Single branch execution - efficient routing

## Context Patterns

### Add request ID
```go
ctx := context.WithValue(ctx, "request-id", uuid.New())
```

### Set timeout
```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Check cancellation
```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
    // Continue processing
}
```