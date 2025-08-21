# Troubleshooting Guide

This guide helps diagnose and resolve common issues when working with pipz pipelines. Start with the Common Gotchas section for the most frequent mistakes.

## Common Gotchas (Quick Reference)

These are the most common mistakes that cause bugs in pipz pipelines:

### 1. Creating Rate Limiters or Circuit Breakers Per Request

**❌ WRONG:**
```go
func handleRequest(req Request) Response {
    limiter := pipz.NewRateLimiter("api", 100, 10) // New instance!
    return limiter.Process(ctx, req) // Useless!
}
```

**✅ RIGHT:**
```go
var apiLimiter = pipz.NewRateLimiter("api", 100, 10) // Singleton

func handleRequest(req Request) Response {
    return apiLimiter.Process(ctx, req)
}
```

### 2. Forgetting to Implement Clone() Properly

**❌ WRONG:**
```go
func (d Data) Clone() Data {
    return Data{Items: d.Items} // Shares slice memory!
}
```

**✅ RIGHT:**
```go
func (d Data) Clone() Data {
    items := make([]Item, len(d.Items))
    copy(items, d.Items)
    return Data{Items: items}
}
```

### 3. Using Transform for Operations That Can Fail

**❌ WRONG:**
```go
transform := pipz.Transform("parse", func(ctx context.Context, s string) Data {
    data, _ := json.Unmarshal([]byte(s), &Data{}) // Error ignored!
    return data
})
```

**✅ RIGHT:**
```go
apply := pipz.Apply("parse", func(ctx context.Context, s string) (Data, error) {
    var data Data
    err := json.Unmarshal([]byte(s), &data)
    return data, err
})
```

### 4. Not Respecting Context Cancellation

**❌ WRONG:**
```go
processor := pipz.Apply("slow", func(ctx context.Context, data Data) (Data, error) {
    time.Sleep(10 * time.Second) // Ignores context!
    return data, nil
})
```

**✅ RIGHT:**
```go
processor := pipz.Apply("slow", func(ctx context.Context, data Data) (Data, error) {
    select {
    case <-time.After(10 * time.Second):
        return data, nil
    case <-ctx.Done():
        return data, ctx.Err()
    }
})
```

### 5. Using Race/Contest for Operations with Side Effects

**❌ WRONG:**
```go
race := pipz.NewRace("payments",
    pipz.Apply("stripe", chargeStripe),     // Charges!
    pipz.Apply("paypal", chargePayPal),     // Also charges!
    pipz.Apply("square", chargeSquare),     // Triple charge!
)
```

**✅ RIGHT:**
```go
race := pipz.NewRace("fetch",
    pipz.Apply("cache", fetchFromCache),
    pipz.Apply("db", fetchFromDB),
    pipz.Apply("api", fetchFromAPI),
)
```

### 6. Expecting Results from Concurrent

**❌ WRONG:**
```go
concurrent := pipz.NewConcurrent("modify",
    pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is still 5, not 10!
```

**✅ RIGHT:**
```go
concurrent := pipz.NewConcurrent("effects",
    pipz.Effect("log", logData),
    pipz.Effect("metrics", updateMetrics),
)
```

### 7. Using Switch Without Default for Unknown Cases

**❌ WRONG:**
```go
switch := pipz.NewSwitch("router",
    func(ctx context.Context, data Data) string {
        return data.Type // Could be anything!
    },
).
AddRoute("A", processA).
AddRoute("B", processB)
// Missing routes will cause runtime errors!
```

**✅ RIGHT:**
```go
switch := pipz.NewSwitch("router",
    func(ctx context.Context, data Data) string {
        return data.Type
    },
).
AddRoute("A", processA).
AddRoute("B", processB).
Default(processUnknown) // Safety net
```

### 8. Ignoring Pipeline Errors

**❌ WRONG:**
```go
result, _ := pipeline.Process(ctx, data) // Error ignored!
processResult(result) // May be zero value!
```

**✅ RIGHT:**
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[Data]
    if errors.As(err, &pipeErr) {
        log.Printf("Failed at %s: %v", pipeErr.Stage, pipeErr.Cause)
    }
    return handleError(err)
}
processResult(result)
```

### 9. Nested Timeouts with Wrong Duration

**❌ WRONG:**
```go
timeout := pipz.NewTimeout("outer",
    pipz.NewTimeout("inner", processor, 10*time.Second), // Longer!
    5*time.Second, // Shorter - inner never gets full time
)
```

**✅ RIGHT:**
```go
timeout := pipz.NewTimeout("outer",
    pipz.NewSequence("steps",
        pipz.NewTimeout("step1", step1, 5*time.Second),
        pipz.NewTimeout("step2", step2, 5*time.Second),
    ),
    12*time.Second, // Accommodates both with buffer
)
```

### 10. Using Enrich for Required Operations

**❌ WRONG:**
```go
enrich := pipz.Enrich("validate", func(ctx context.Context, user User) (User, error) {
    if !isValid(user) {
        return user, errors.New("invalid") // Error is swallowed!
    }
    return user, nil
})
```

**✅ RIGHT:**
```go
apply := pipz.Apply("validate", func(ctx context.Context, user User) (User, error) {
    if !isValid(user) {
        return user, errors.New("invalid") // Fails the pipeline
    }
    return user, nil
})
```

## Common Issues and Solutions

### 1. Pipeline Execution Issues

#### Pipeline Stops Unexpectedly

**Symptom**: Pipeline execution halts without clear error message.

**Possible Causes**:
- Context cancellation or timeout
- Panic in a processor function
- Deadlock in concurrent operations

**Solutions**:

```go
// Check for context issues
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := pipeline.Process(ctx, data)
if err != nil {
    // Check if it's a context error
    if errors.Is(err.Cause, context.DeadlineExceeded) {
        log.Println("Pipeline timed out")
    } else if errors.Is(err.Cause, context.Canceled) {
        log.Println("Pipeline was cancelled")
    }
}

// Add panic recovery
safeProcessor := pipz.Apply("safe", func(ctx context.Context, data T) (T, error) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic recovered: %v", r)
        }
    }()
    return riskyOperation(data)
})
```

#### Pipeline Returns Unexpected Results

**Symptom**: Output doesn't match expected transformation.

**Possible Causes**:
- Processors registered in wrong order
- Mutation of shared state
- Missing Clone() implementation for concurrent processing

**Solutions**:

```go
// Verify processor order
seq := pipz.NewSequence[Data]("pipeline")
seq.Iterate(func(p pipz.Chainable[Data]) bool {
    fmt.Printf("Processor: %s\n", p.Name())
    return true // continue iteration
})

// Ensure proper cloning for concurrent operations
type Data struct {
    Values []int
}

func (d Data) Clone() Data {
    // Deep copy slice to prevent shared state
    newValues := make([]int, len(d.Values))
    copy(newValues, d.Values)
    return Data{Values: newValues}
}
```

### 2. Memory and Performance Issues

#### High Memory Usage

**Symptom**: Memory consumption grows unexpectedly.

**Possible Causes**:
- Large data structures not being garbage collected
- Goroutine leaks in concurrent processors
- Buffered channels not being drained

**Solutions**:

```go
// Use streaming for large datasets
func processLargeDataset(ctx context.Context, reader io.Reader) error {
    scanner := bufio.NewScanner(reader)
    pipeline := createPipeline()
    
    for scanner.Scan() {
        data := parseData(scanner.Text())
        result, err := pipeline.Process(ctx, data)
        if err != nil {
            return err
        }
        // Process result immediately, don't accumulate
        handleResult(result)
    }
    return scanner.Err()
}

// Ensure goroutines are properly cleaned up
concurrent := pipz.NewConcurrent("parallel", processors...)
// Always use context with timeout/cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
result, err := concurrent.Process(ctx, data)
```

#### Slow Pipeline Execution

**Symptom**: Pipeline takes longer than expected to complete.

**Possible Causes**:
- Sequential processing of independent operations
- Inefficient processor implementations
- Excessive context switching in concurrent operations

**Solutions**:

```go
// Profile to identify bottlenecks
import _ "net/http/pprof"

// In your main:
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Use concurrent processing for independent operations
// Instead of:
sequential := pipz.NewSequence("slow",
    fetchUserData,    // 100ms
    fetchOrderData,   // 150ms
    fetchInventory,   // 200ms
)

// Use:
concurrent := pipz.NewConcurrent("fast",
    fetchUserData,
    fetchOrderData,
    fetchInventory,
)
// Total time: ~200ms instead of 450ms
```

### 3. Error Handling Issues

#### Errors Not Being Caught

**Symptom**: Errors pass through without being handled.

**Possible Causes**:
- Using Transform instead of Apply for fallible operations
- Not checking error returns
- Enrich processor swallowing errors

**Solutions**:

```go
// Use Apply for operations that can fail
// Wrong:
processor := pipz.Transform("parse", func(ctx context.Context, s string) Data {
    var d Data
    json.Unmarshal([]byte(s), &d) // Error ignored!
    return d
})

// Correct:
processor := pipz.Apply("parse", func(ctx context.Context, s string) (Data, error) {
    var d Data
    err := json.Unmarshal([]byte(s), &d)
    return d, err
})

// Always check pipeline errors
result, err := pipeline.Process(ctx, input)
if err != nil {
    log.Printf("Pipeline failed at stage '%s': %v", err.Stage, err.Cause)
    // Access the data state at failure
    log.Printf("Data state at failure: %+v", err.State)
}
```

#### Error Recovery Not Working

**Symptom**: Fallback processors not executing on error.

**Possible Causes**:
- Incorrect Fallback configuration
- Error occurring after fallback point
- Context already cancelled

**Solutions**:

```go
// Ensure fallback is properly configured
fallback := pipz.NewFallback("recover",
    primaryPipeline,
    pipz.Transform("default", func(ctx context.Context, data T) T {
        return getDefaultValue()
    }),
)

// Check that context is still valid
if ctx.Err() != nil {
    log.Printf("Context already cancelled: %v", ctx.Err())
}
```

### 4. Type Safety Issues

#### Compilation Errors with Generics

**Symptom**: "cannot infer T" or type mismatch errors.

**Possible Causes**:
- Type inference limitations
- Incompatible processor types in sequence
- Missing type parameters

**Solutions**:

```go
// Explicitly specify type parameters when needed
// Instead of:
seq := pipz.NewSequence("pipeline") // Error: cannot infer T

// Use:
seq := pipz.NewSequence[MyDataType]("pipeline")

// Ensure all processors handle the same type
type User struct { /* fields */ }
type Order struct { /* fields */ }

// This won't compile:
pipeline := pipz.NewSequence("mixed",
    processUser,  // Chainable[User]
    processOrder, // Chainable[Order] - Type mismatch!
)

// Use transformation to change types:
pipeline := pipz.NewSequence("correct",
    processUser,
    pipz.Apply("convert", func(ctx context.Context, u User) (Order, error) {
        return convertUserToOrder(u)
    }),
    processOrder,
)
```

### 5. Concurrency Issues

#### Race Conditions

**Symptom**: Inconsistent results, data corruption, panics.

**Possible Causes**:
- Shared mutable state
- Missing Clone() implementation
- Unsafe concurrent access

**Solutions**:

```go
// Implement proper cloning
type Data struct {
    mu     sync.Mutex // Don't include mutex in clone!
    values map[string]int
}

func (d Data) Clone() Data {
    newValues := make(map[string]int, len(d.values))
    for k, v := range d.values {
        newValues[k] = v
    }
    return Data{values: newValues}
}

// Use synchronization for shared resources
var (
    counter int64
)

processor := pipz.Effect("count", func(ctx context.Context, data T) error {
    atomic.AddInt64(&counter, 1)
    return nil
})
```

#### Deadlocks

**Symptom**: Pipeline hangs indefinitely.

**Possible Causes**:
- Circular dependencies
- Channel deadlocks
- Mutex ordering issues

**Solutions**:

```go
// Always use timeouts
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Detect deadlocks with runtime checks
import _ "runtime/debug"

// In development:
runtime.SetBlockProfileRate(1)

// Avoid circular dependencies
// Bad:
proc1 := createProcessor(proc2) // proc1 depends on proc2
proc2 := createProcessor(proc1) // proc2 depends on proc1

// Good:
proc1 := createProcessor(nil)
proc2 := createProcessor(nil)
// Configure dependencies after creation if needed
```

### 6. Circuit Breaker Issues

#### Circuit Breaker Not Opening

**Symptom**: Failed requests continue despite threshold.

**Possible Causes**:
- Threshold too high
- Window size too large
- Errors not properly propagated

**Solutions**:

```go
cb := pipz.NewCircuitBreaker("api", 
    apiProcessor,
    pipz.WithCircuitBreakerThreshold(5),        // Open after 5 failures
    pipz.WithCircuitBreakerWindow(time.Minute), // Within 1 minute
    pipz.WithCircuitBreakerCooldown(30*time.Second),
)

// Ensure errors are returned, not swallowed
apiProcessor := pipz.Apply("api", func(ctx context.Context, data T) (T, error) {
    resp, err := callAPI(data)
    if err != nil {
        return data, err // Must return error for circuit breaker to count
    }
    if resp.StatusCode >= 500 {
        return data, fmt.Errorf("server error: %d", resp.StatusCode)
    }
    return processResponse(resp)
})
```

### 7. Rate Limiter Issues

#### Rate Limiting Not Working

**Symptom**: Requests exceed configured rate.

**Possible Causes**:
- Incorrect rate configuration
- Multiple rate limiter instances
- Time synchronization issues

**Solutions**:

```go
// Use a single rate limiter instance
var rateLimiter = pipz.NewRateLimiter("api",
    apiProcessor,
    pipz.WithRateLimiterRate(100),           // 100 requests
    pipz.WithRateLimiterPeriod(time.Second), // per second
)

// Don't create new instances per request
// Wrong:
func handleRequest(data T) (T, error) {
    rl := pipz.NewRateLimiter(...) // New instance each time!
    return rl.Process(ctx, data)
}

// Correct:
func handleRequest(data T) (T, error) {
    return rateLimiter.Process(ctx, data) // Reuse single instance
}
```

## Debugging Techniques

### 1. Pipeline Inspection

```go
// Inspect pipeline structure
func inspectPipeline[T any](seq *pipz.Sequence[T]) {
    fmt.Printf("Pipeline: %s\n", seq.Name())
    seq.Iterate(func(p pipz.Chainable[T]) bool {
        fmt.Printf("  - %s\n", p.Name())
        return true
    })
}

// Add debug logging
debugPipeline := pipz.NewSequence("debug",
    pipz.Effect("log-input", func(ctx context.Context, data T) error {
        log.Printf("Input: %+v", data)
        return nil
    }),
    actualProcessor,
    pipz.Effect("log-output", func(ctx context.Context, data T) error {
        log.Printf("Output: %+v", data)
        return nil
    }),
)
```

### 2. Error Analysis

```go
// Detailed error logging
result, err := pipeline.Process(ctx, data)
if err != nil {
    pipeErr := err.(*pipz.Error[T])
    log.Printf("Pipeline failed:")
    log.Printf("  Stage: %s", pipeErr.Stage)
    log.Printf("  Cause: %v", pipeErr.Cause)
    log.Printf("  Type: %T", pipeErr.Cause)
    log.Printf("  State: %+v", pipeErr.State)
    
    // Check for specific error types
    var validationErr *ValidationError
    if errors.As(pipeErr.Cause, &validationErr) {
        log.Printf("  Validation failed: %s", validationErr.Field)
    }
}
```

### 3. Performance Profiling

```go
// Time individual processors
func timeProcessor[T any](name pipz.Name, p pipz.Chainable[T]) pipz.Chainable[T] {
    return pipz.Apply(name, func(ctx context.Context, data T) (T, error) {
        start := time.Now()
        result, err := p.Process(ctx, data)
        duration := time.Since(start)
        log.Printf("Processor %s took %v", p.Name(), duration)
        return result, err
    })
}

// Memory profiling
import (
    "runtime"
    "runtime/pprof"
)

func profileMemory() {
    f, _ := os.Create("mem.prof")
    defer f.Close()
    runtime.GC()
    pprof.WriteHeapProfile(f)
}
```

## Getting Help

If you're still experiencing issues:

1. **Check the Examples**: Review the `/examples` directory for working implementations
2. **Read the Tests**: Test files often demonstrate edge cases and proper usage
3. **Enable Debug Logging**: Set up detailed logging to trace execution
4. **Create a Minimal Reproduction**: Isolate the issue in a small, reproducible example
5. **File an Issue**: Report bugs at https://github.com/zoobzio/pipz/issues with:
   - Go version
   - pipz version
   - Minimal code to reproduce
   - Expected vs actual behavior
   - Any error messages or stack traces

## Common Patterns for Resilience

### Defensive Pipeline Construction

```go
// Combine multiple resilience patterns
resilientPipeline := pipz.NewSequence("resilient",
    // Input validation
    pipz.Apply("validate", validateInput),
    
    // Rate limiting
    pipz.NewRateLimiter("rate", 
        // Circuit breaker
        pipz.NewCircuitBreaker("circuit",
            // Timeout protection
            pipz.NewTimeout("timeout",
                // Retry logic
                pipz.NewRetry("retry",
                    actualProcessor,
                    pipz.WithRetryAttempts(3),
                ),
                pipz.WithTimeoutDuration(5*time.Second),
            ),
        ),
    ),
    
    // Error recovery
    pipz.NewFallback("recover",
        riskyProcessor,
        safeDefault,
    ),
)
```

This layered approach provides multiple levels of protection against various failure modes.