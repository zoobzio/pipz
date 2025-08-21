# Testing pipz Pipelines

Quick solutions for common testing scenarios.

## Testing Problems & Solutions

### You need to: Verify where errors occur in your pipeline

**Solution:** Use pipz.Error[T] path tracking

```go
func TestErrorLocation(t *testing.T) {
    pipeline := pipz.NewSequence[int]("math",
        pipz.Transform("double", func(_ context.Context, n int) int {
            return n * 2
        }),
        pipz.Apply("divide", func(_ context.Context, n int) (int, error) {
            if n == 0 {
                return 0, errors.New("division by zero")
            }
            return 100 / n, nil
        }),
    )
    
    _, err := pipeline.Process(context.Background(), 0)
    
    var pipeErr *pipz.Error[int]
    if !errors.As(err, &pipeErr) {
        t.Fatal("expected pipz.Error")
    }
    
    // Verify exact failure location
    if pipeErr.Stage != "divide" {
        t.Errorf("error at wrong stage: %s", pipeErr.Stage)
    }
}
```

**Why this matters:** Know exactly which processor failed without debugging

---

### You need to: Test pipeline modifications at runtime

**Solution:** Test Sequence mutations

```go
func TestDynamicPipeline(t *testing.T) {
    seq := pipz.NewSequence[string]("dynamic")
    
    // Start with one processor
    seq.Register(pipz.Transform("upper", strings.ToUpper))
    result, _ := seq.Process(context.Background(), "hello")
    assert.Equal(t, "HELLO", result)
    
    // Add processor at runtime
    seq.PushTail(pipz.Transform("exclaim", func(_ context.Context, s string) string {
        return s + "!"
    }))
    result, _ = seq.Process(context.Background(), "hello")
    assert.Equal(t, "HELLO!", result)
}
```

**Why this matters:** Sequences can be modified - test that behavior

---

### You need to: Ensure concurrent processors don't share state

**Solution:** Verify Clone() implementation

```go
func TestCloneSafety(t *testing.T) {
    type Data struct {
        Values []int
    }
    
    // Implement Clone
    func (d Data) Clone() Data {
        newValues := make([]int, len(d.Values))
        copy(newValues, d.Values)
        return Data{Values: newValues}
    }
    
    concurrent := pipz.NewConcurrent[Data]("parallel",
        pipz.Apply("mod1", func(_ context.Context, d Data) (Data, error) {
            d.Values[0] = 999
            return d, nil
        }),
        pipz.Apply("mod2", func(_ context.Context, d Data) (Data, error) {
            d.Values[1] = 888
            return d, nil
        }),
    )
    
    original := Data{Values: []int{1, 2, 3}}
    originalCopy := Data{Values: []int{1, 2, 3}}
    
    concurrent.Process(context.Background(), original)
    
    // Original MUST be unchanged
    if !reflect.DeepEqual(original, originalCopy) {
        t.Error("concurrent processing modified original!")
    }
}
```

**Why this matters:** Concurrent processors run in parallel - data corruption is silent

---

### You need to: Test processors respect context cancellation

**Solution:** Use context with timeout

```go
func TestContextRespect(t *testing.T) {
    slowProcessor := pipz.Apply("slow", func(ctx context.Context, v int) (int, error) {
        select {
        case <-time.After(1 * time.Second):
            return v * 2, nil
        case <-ctx.Done():
            return 0, ctx.Err()
        }
    })
    
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    
    _, err := slowProcessor.Process(ctx, 42)
    
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Error("processor didn't respect context cancellation")
    }
}
```

**Why this matters:** Prevents hanging pipelines in production

---

### You need to: Test circuit breaker opens after failures

**Solution:** Verify state changes

```go
func TestCircuitBreakerState(t *testing.T) {
    callCount := 0
    failing := pipz.Apply("fail", func(_ context.Context, v int) (int, error) {
        callCount++
        return 0, errors.New("service down")
    })
    
    cb := pipz.NewCircuitBreaker("test", failing,
        pipz.WithCircuitBreakerThreshold(3),
    )
    
    // First 3 calls hit the processor
    for i := 0; i < 3; i++ {
        cb.Process(context.Background(), i)
    }
    assert.Equal(t, 3, callCount)
    
    // 4th call should fail fast (circuit open)
    cb.Process(context.Background(), 4)
    assert.Equal(t, 3, callCount) // No new call
}
```

**Why this matters:** Circuit breakers are stateful - must verify they actually protect

---

### You need to: Test rate limiter throttling

**Solution:** Measure timing

```go
func TestRateLimiting(t *testing.T) {
    fast := pipz.Apply("fast", func(_ context.Context, v int) (int, error) {
        return v, nil
    })
    
    // 2 requests per 100ms
    limiter := pipz.NewRateLimiter("test", fast,
        pipz.WithRateLimiterRate(2),
        pipz.WithRateLimiterPeriod(100*time.Millisecond),
    )
    
    start := time.Now()
    
    // Process 4 items (should take ~100ms)
    for i := 0; i < 4; i++ {
        limiter.Process(context.Background(), i)
    }
    
    elapsed := time.Since(start)
    if elapsed < 100*time.Millisecond {
        t.Error("rate limiter not throttling")
    }
}
```

**Why this matters:** Rate limiters must be singletons to work

---

### You need to: Access data state at error time

**Solution:** Use Error[T].State

```go
func TestErrorState(t *testing.T) {
    pipeline := pipz.NewSequence[User]("process",
        pipz.Transform("enrich", func(_ context.Context, u User) User {
            u.ProcessedAt = time.Now()
            return u
        }),
        pipz.Apply("validate", func(_ context.Context, u User) (User, error) {
            if u.Age < 18 {
                return u, errors.New("underage")
            }
            return u, nil
        }),
    )
    
    user := User{Name: "Alice", Age: 16}
    _, err := pipeline.Process(context.Background(), user)
    
    var pipeErr *pipz.Error[User]
    if errors.As(err, &pipeErr) {
        // Access the exact state when error occurred
        assert.Equal(t, "Alice", pipeErr.State.Name)
        assert.NotZero(t, pipeErr.State.ProcessedAt) // Was enriched
    }
}
```

**Why this matters:** Debug with complete context

## Testing Gotchas

### ❌ Creating rate limiters per test
```go
// WRONG - Each test gets new limiter
func process(data int) {
    limiter := pipz.NewRateLimiter(...) // New instance!
    limiter.Process(ctx, data)
}
```

### ✅ Share rate limiter instance
```go
// RIGHT - Singleton
var limiter = pipz.NewRateLimiter(...)

func process(data int) {
    limiter.Process(ctx, data)
}
```

### ❌ Not deep copying in Clone()
```go
// WRONG - Shares slice
func (d Data) Clone() Data {
    return Data{Items: d.Items} // Shallow copy!
}
```

### ✅ Deep copy slices and maps
```go
// RIGHT - Deep copy
func (d Data) Clone() Data {
    items := make([]Item, len(d.Items))
    copy(items, d.Items)
    return Data{Items: items}
}
```

## Quick Test Patterns

### Mock Processor
```go
type Mock[T any] struct {
    CallCount int
    Returns   T
    Error     error
}

func (m *Mock[T]) Process(ctx context.Context, data T) (T, error) {
    m.CallCount++
    return m.Returns, m.Error
}

func (m *Mock[T]) Name() string { return "mock" }
```

### Test Helper for Errors
```go
func assertPipelineError(t *testing.T, err error, stage string) {
    var pipeErr *pipz.Error[any]
    if !errors.As(err, &pipeErr) {
        t.Fatal("expected pipz.Error")
    }
    if pipeErr.Stage != stage {
        t.Errorf("error at %s, expected %s", pipeErr.Stage, stage)
    }
}
```

### Timeout Helper
```go
func withTimeout(t *testing.T, d time.Duration, fn func()) {
    done := make(chan struct{})
    go func() {
        fn()
        close(done)
    }()
    
    select {
    case <-done:
        // Success
    case <-time.After(d):
        t.Fatal("test timed out")
    }
}
```