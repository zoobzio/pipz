# Testing pipz Pipelines

Comprehensive guide to testing pipelines with the pipz testing utilities, best practices, and three-tier testing strategy.

## Table of Contents
- [Testing Package Overview](#testing-package-overview)
- [MockProcessor - Testing Pipeline Behavior](#mockprocessor---testing-pipeline-behavior)
- [ChaosProcessor - Resilience Testing](#chaosprocessor---resilience-testing)
- [Assertion Helpers](#assertion-helpers)
- [Test Organization Strategy](#test-organization-strategy)
- [Testing Best Practices](#testing-best-practices)
- [Common Testing Patterns](#common-testing-patterns)
- [Testing Gotchas](#testing-gotchas)

## Testing Package Overview

The `github.com/zoobzio/pipz/testing` package provides comprehensive utilities for testing pipz pipelines:

```go
import pipztesting "github.com/zoobzio/pipz/testing"
```

### Core Testing Components

- **MockProcessor**: Configurable mock implementation for testing pipeline behavior
- **ChaosProcessor**: Chaos engineering tool for resilience testing
- **Assertion Helpers**: Utilities for verifying processor calls and behaviors
- **Helper Functions**: Timing, parallelization, and synchronization utilities

## MockProcessor - Testing Pipeline Behavior

MockProcessor provides a fully configurable mock implementation of `pipz.Chainable[T]` with call tracking, configurable return values, delays, and panic simulation.

### Basic Mock Usage

```go
func TestPipelineWithMock(t *testing.T) {
    // Create a mock processor
    mock := pipztesting.NewMockProcessor[string](t, "data-processor")
    
    // Configure return values
    mock.WithReturn("processed", nil)
    
    // Build pipeline with mock
    pipeline := pipz.NewSequence[string]("test-pipeline",
        pipz.Transform("prepare", strings.ToUpper),
        mock,
        pipz.Transform("finalize", strings.TrimSpace),
    )
    
    // Process data
    result, err := pipeline.Process(context.Background(), "  input  ")
    
    // Verify results
    require.NoError(t, err)
    assert.Equal(t, "processed", result)
    
    // Verify mock was called
    pipztesting.AssertProcessed(t, mock, 1)
    pipztesting.AssertProcessedWith(t, mock, "INPUT")
}
```

### Testing Error Paths

```go
func TestErrorHandling(t *testing.T) {
    // Create mock that returns error
    mock := pipztesting.NewMockProcessor[Order](t, "payment-processor")
    mock.WithReturn(Order{}, errors.New("payment declined"))
    
    // Build pipeline with error handling
    pipeline := pipz.NewHandle[Order]("order-pipeline",
        pipz.NewSequence[Order]("process",
            validateOrder,
            mock, // Will fail here
            shipOrder,
        ),
        pipz.Transform("error-recovery", func(ctx context.Context, err *pipz.Error[Order]) *pipz.Error[Order] {
            // Log error and mark order as failed
            err.State.Status = "payment_failed"
            return err
        }),
    )
    
    order := Order{ID: "123", Amount: 99.99}
    _, err := pipeline.Process(context.Background(), order)
    
    // Verify error occurred
    require.Error(t, err)
    
    // Verify shipOrder was never called (pipeline stopped at error)
    var pipeErr *pipz.Error[Order]
    require.True(t, errors.As(err, &pipeErr))
    assert.Equal(t, "payment-processor", string(pipeErr.Stage))
    assert.Equal(t, "payment_failed", pipeErr.State.Status)
}
```

### Testing Delays and Timeouts

```go
func TestTimeoutBehavior(t *testing.T) {
    // Create mock with 200ms delay
    slowMock := pipztesting.NewMockProcessor[string](t, "slow-service")
    slowMock.WithReturn("result", nil).WithDelay(200 * time.Millisecond)
    
    // Wrap with timeout
    pipeline := pipz.NewTimeout[string]("fast-timeout",
        slowMock,
        100*time.Millisecond, // Timeout before mock completes
    )
    
    // Process should timeout
    _, err := pipeline.Process(context.Background(), "data")
    
    // Verify timeout occurred
    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
    
    // Mock should still have been called
    pipztesting.AssertProcessed(t, slowMock, 1)
}
```

### Testing Panic Recovery

```go
func TestPanicRecovery(t *testing.T) {
    // Create mock that panics
    panicMock := pipztesting.NewMockProcessor[string](t, "unstable-service")
    panicMock.WithPanic("database connection lost")
    
    // Build pipeline with panic recovery
    pipeline := pipz.NewHandle[string]("safe-pipeline",
        panicMock,
        pipz.Transform("recover", func(ctx context.Context, err *pipz.Error[string]) *pipz.Error[string] {
            if err.Panic {
                // Convert panic to error
                err.Wrapped = fmt.Errorf("recovered from panic: %v", err.Error())
                err.Panic = false
            }
            return err
        }),
    )
    
    // Should recover from panic
    _, err := pipeline.Process(context.Background(), "test")
    
    // Verify error but not panic
    require.Error(t, err)
    assert.Contains(t, err.Error(), "recovered from panic")
}
```

### Call History Tracking

```go
func TestCallHistory(t *testing.T) {
    mock := pipztesting.NewMockProcessor[int](t, "accumulator")
    mock.WithReturn(0, nil).WithHistorySize(10) // Keep last 10 calls
    
    // Process multiple values
    for i := 0; i < 5; i++ {
        mock.Process(context.Background(), i)
    }
    
    // Examine call history
    history := mock.CallHistory()
    require.Len(t, history, 5)
    
    // Verify call order and timing
    for i, call := range history {
        assert.Equal(t, i, call.Input)
        if i > 0 {
            assert.True(t, call.Timestamp.After(history[i-1].Timestamp))
        }
    }
}
```

## ChaosProcessor - Resilience Testing

ChaosProcessor enables chaos engineering for pipelines by randomly injecting failures, delays, timeouts, and panics. This helps verify resilience patterns work correctly under adverse conditions.

### Basic Chaos Testing

```go
func TestPipelineResilience(t *testing.T) {
    // Wrap a normal processor with chaos
    normalProcessor := pipz.Transform("process", func(ctx context.Context, data string) string {
        return data + "_processed"
    })
    
    // Configure chaos
    chaosConfig := pipztesting.ChaosConfig{
        FailureRate: 0.2,              // 20% failure rate
        LatencyMin:  10 * time.Millisecond,
        LatencyMax:  50 * time.Millisecond,
        TimeoutRate: 0.1,               // 10% timeout rate
        PanicRate:   0.05,              // 5% panic rate
        Seed:        42,                // Reproducible chaos
    }
    
    chaos := pipztesting.NewChaosProcessor("chaos-test", normalProcessor, chaosConfig)
    
    // Build resilient pipeline
    pipeline := pipz.NewRetry[string]("resilient",
        pipz.NewHandle[string]("recover",
            chaos,
            pipz.Transform("handle-error", func(ctx context.Context, err *pipz.Error[string]) *pipz.Error[string] {
                // Log and recover from errors
                return err
            }),
        ),
        3, // Retry up to 3 times
    )
    
    // Run many iterations to trigger chaos
    successCount := 0
    failureCount := 0
    
    for i := 0; i < 100; i++ {
        result, err := pipeline.Process(context.Background(), fmt.Sprintf("request_%d", i))
        if err == nil {
            successCount++
            assert.Contains(t, result, "_processed")
        } else {
            failureCount++
        }
    }
    
    // With retries, success rate should be higher than failure rate
    stats := chaos.Stats()
    t.Logf("Chaos Stats: %s", stats)
    t.Logf("Pipeline Success Rate: %.1f%%", float64(successCount)/100*100)
    
    // Verify chaos was actually injected
    assert.Greater(t, stats.FailedCalls+stats.TimeoutCalls+stats.PanicCalls, int64(0))
}
```

### Testing Circuit Breaker with Chaos

```go
func TestCircuitBreakerUnderChaos(t *testing.T) {
    // Create service with intermittent failures
    service := pipz.Apply("external-api", func(ctx context.Context, req Request) (Response, error) {
        // Actual API call
        return callAPI(req)
    })
    
    // Add chaos to simulate network issues
    chaosService := pipztesting.NewChaosProcessor("chaos-api", service,
        pipztesting.ChaosConfig{
            FailureRate: 0.3,  // 30% failures
            TimeoutRate: 0.2,  // 20% timeouts
            LatencyMin:  50 * time.Millisecond,
            LatencyMax:  200 * time.Millisecond,
        },
    )
    
    // Wrap with circuit breaker
    circuitBreaker := pipz.NewCircuitBreaker[Request]("api-breaker",
        chaosService,
        10,                     // Open after 10 failures
        30*time.Second,         // Recovery time
    )
    
    // Test that circuit breaker opens under chaos
    var openedAt time.Time
    failuresSinceOpen := 0
    
    for i := 0; i < 50; i++ {
        _, err := circuitBreaker.Process(context.Background(), Request{ID: i})
        
        var pipeErr *pipz.Error[Request]
        if errors.As(err, &pipeErr) && pipeErr.Stage == "api-breaker" {
            if openedAt.IsZero() {
                openedAt = time.Now()
                t.Logf("Circuit opened after %d requests", i)
            }
            failuresSinceOpen++
        }
    }
    
    // Verify circuit breaker opened
    assert.False(t, openedAt.IsZero(), "Circuit breaker should have opened")
    
    // Verify fast failures after opening
    assert.Greater(t, failuresSinceOpen, 10, "Should fail fast when open")
    
    // Check chaos statistics
    stats := chaosService.Stats()
    t.Logf("Chaos injected: %d failures, %d timeouts out of %d calls",
        stats.FailedCalls, stats.TimeoutCalls, stats.TotalCalls)
}
```

### Load Testing with Chaos

```go
func TestLoadWithChaos(t *testing.T) {
    // Create processor that tracks throughput
    var processed atomic.Int64
    processor := pipz.Effect("counter", func(ctx context.Context, data int) error {
        processed.Add(1)
        return nil
    })
    
    // Add variable chaos
    chaos := pipztesting.NewChaosProcessor("variable-chaos", processor,
        pipztesting.ChaosConfig{
            FailureRate: 0.1,
            LatencyMin:  1 * time.Millisecond,
            LatencyMax:  10 * time.Millisecond,
            TimeoutRate: 0.05,
        },
    )
    
    // Build pipeline with rate limiting and retries
    pipeline := pipz.NewSequence[int]("load-test",
        pipz.NewRateLimiter[int]("throttle", 100, 10), // 100 req/s, burst 10
        pipz.NewRetry[int]("retry", chaos, 2),
    )
    
    // Generate load
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ { // 10 concurrent workers
        wg.Add(1)
        go func(worker int) {
            defer wg.Done()
            for j := 0; ctx.Err() == nil; j++ {
                pipeline.Process(ctx, worker*1000+j)
            }
        }(i)
    }
    
    wg.Wait()
    
    // Analyze results
    totalProcessed := processed.Load()
    stats := chaos.Stats()
    
    t.Logf("Processed: %d requests in 5 seconds", totalProcessed)
    t.Logf("Chaos Stats: %s", stats)
    t.Logf("Effective throughput: %.1f req/s", float64(totalProcessed)/5)
    
    // Verify system remained stable under chaos
    assert.Greater(t, totalProcessed, int64(100), "Should process reasonable amount despite chaos")
}
```

## Assertion Helpers

The testing package provides specialized assertions for verifying processor behavior:

### Basic Assertions

```go
func TestProcessorAssertions(t *testing.T) {
    mock := pipztesting.NewMockProcessor[string](t, "test-processor")
    mock.WithReturn("result", nil)
    
    pipeline := pipz.NewSequence[string]("pipeline", mock)
    
    // Process some data
    pipeline.Process(context.Background(), "input1")
    pipeline.Process(context.Background(), "input2")
    
    // Assert exact call count
    pipztesting.AssertProcessed(t, mock, 2)
    
    // Assert last input
    pipztesting.AssertProcessedWith(t, mock, "input2")
    
    // Assert call count range
    pipztesting.AssertProcessedBetween(t, mock, 1, 3)
    
    // Reset and verify no calls
    mock.Reset()
    pipztesting.AssertNotProcessed(t, mock)
}
```

### Waiting for Async Operations

```go
func TestAsyncProcessing(t *testing.T) {
    mock := pipztesting.NewMockProcessor[string](t, "async-processor")
    mock.WithReturn("done", nil)
    
    // Process asynchronously
    go func() {
        time.Sleep(100 * time.Millisecond)
        mock.Process(context.Background(), "async-data")
    }()
    
    // Wait for processor to be called
    success := pipztesting.WaitForCalls(mock, 1, 500*time.Millisecond)
    require.True(t, success, "Mock should have been called within timeout")
    
    // Verify the call
    pipztesting.AssertProcessedWith(t, mock, "async-data")
}
```

### Parallel Testing Helper

```go
func TestConcurrentSafety(t *testing.T) {
    var counter atomic.Int64
    processor := pipz.Effect("counter", func(ctx context.Context, n int) error {
        counter.Add(1)
        return nil
    })
    
    pipeline := pipz.NewConcurrent[int]("parallel",
        processor,
        processor,
        processor,
    )
    
    // Run parallel test
    pipztesting.ParallelTest(t, 10, func(workerID int) {
        for i := 0; i < 100; i++ {
            pipeline.Process(context.Background(), workerID*1000+i)
        }
    })
    
    // Each of 10 workers processed 100 items through 3 processors
    expected := int64(10 * 100 * 3)
    assert.Equal(t, expected, counter.Load())
}
```

### Latency Measurement

```go
func TestProcessorPerformance(t *testing.T) {
    processor := pipz.Transform("slow", func(ctx context.Context, n int) int {
        time.Sleep(10 * time.Millisecond)
        return n * 2
    })
    
    // Measure latency
    latency := pipztesting.MeasureLatency(func() {
        processor.Process(context.Background(), 42)
    })
    
    assert.GreaterOrEqual(t, latency, 10*time.Millisecond)
    assert.Less(t, latency, 20*time.Millisecond)
    
    // Measure with result
    result, duration := pipztesting.MeasureLatencyWithResult(func() int {
        res, _ := processor.Process(context.Background(), 21)
        return res
    })
    
    assert.Equal(t, 42, result)
    assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
}
```

## Test Organization Strategy

Pipz follows a three-tier testing strategy that separates concerns and enables comprehensive validation:

### 1. Unit Tests (Package-Level)

Located alongside source code, testing individual processors and connectors in isolation.

```
pipz/
├── processor_test.go      # Tests individual processors
├── connector_test.go      # Tests individual connectors
└── error_test.go          # Tests error handling
```

**Example Unit Test:**
```go
// processor_test.go
func TestTransformProcessor(t *testing.T) {
    processor := Transform("double", func(ctx context.Context, n int) int {
        return n * 2
    })
    
    result, err := processor.Process(context.Background(), 21)
    require.NoError(t, err)
    assert.Equal(t, 42, result)
}
```

### 2. Integration Tests

Located in `testing/integration/`, testing complete pipelines and real-world scenarios.

```
testing/integration/
├── pipeline_flows_test.go       # End-to-end pipeline tests
├── resilience_patterns_test.go  # Circuit breakers, retries, fallbacks
└── real_world_test.go           # Business scenario tests
```

**Example Integration Test:**
```go
// testing/integration/resilience_patterns_test.go
func TestCircuitBreakerWithRetry(t *testing.T) {
    var callCount int64
    
    // Flaky service that fails initially
    flakyService := pipz.Apply("flaky", func(ctx context.Context, data string) (string, error) {
        count := atomic.AddInt64(&callCount, 1)
        if count <= 3 {
            return "", errors.New("service unavailable")
        }
        return data + "_processed", nil
    })
    
    // Build resilient pipeline
    pipeline := pipz.NewCircuitBreaker[string]("breaker",
        pipz.NewRetry[string]("retry", flakyService, 3),
        5,                      // Threshold
        time.Second,           // Recovery
    )
    
    // Should succeed after retries
    result, err := pipeline.Process(context.Background(), "test")
    require.NoError(t, err)
    assert.Equal(t, "test_processed", result)
    assert.Equal(t, int64(4), callCount) // 3 failures + 1 success
}
```

### 3. Benchmarks

Located in `testing/benchmarks/`, measuring performance and comparing implementations.

```
testing/benchmarks/
├── core_performance_test.go      # Individual processor benchmarks
├── composition_performance_test.go # Pipeline composition benchmarks
└── comparison_test.go            # Comparative benchmarks
```

**Example Benchmark:**
```go
// testing/benchmarks/core_performance_test.go
func BenchmarkTransformProcessor(b *testing.B) {
    processor := pipz.Transform("double", func(_ context.Context, n int) int {
        return n * 2
    })
    
    ctx := context.Background()
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        result, _ := processor.Process(ctx, 42)
        _ = result // Prevent optimization
    }
}
```

## Testing Best Practices

### 1. Test Data Isolation with Clone()

Always implement proper `Clone()` for concurrent testing:

```go
type TestData struct {
    ID     string
    Values []int
    Meta   map[string]any
}

// Proper deep clone implementation
func (d TestData) Clone() TestData {
    values := make([]int, len(d.Values))
    copy(values, d.Values)
    
    meta := make(map[string]any, len(d.Meta))
    for k, v := range d.Meta {
        meta[k] = v
    }
    
    return TestData{
        ID:     d.ID,
        Values: values,
        Meta:   meta,
    }
}

func TestConcurrentIsolation(t *testing.T) {
    data := TestData{
        ID:     "test",
        Values: []int{1, 2, 3},
        Meta:   map[string]any{"key": "value"},
    }
    
    // Concurrent processors should not affect each other
    concurrent := pipz.NewConcurrent[TestData]("parallel",
        pipz.Mutate("modify1", func(ctx context.Context, d *TestData) error {
            d.Values[0] = 999
            return nil
        }),
        pipz.Mutate("modify2", func(ctx context.Context, d *TestData) error {
            d.Meta["new"] = "data"
            return nil
        }),
    )
    
    original := data.Clone()
    concurrent.Process(context.Background(), data)
    
    // Original must be unchanged
    assert.Equal(t, original, data)
}
```

### 2. Stateful Connector Testing

Stateful connectors (RateLimiter, CircuitBreaker) must be singletons:

```go
func TestStatefulConnectorSharing(t *testing.T) {
    // CORRECT: Shared instance maintains state
    rateLimiter := pipz.NewRateLimiter[string]("api", 2, 1) // 2 req/s
    
    // Multiple goroutines share the same limiter
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            rateLimiter.Process(context.Background(), fmt.Sprintf("req_%d", id))
        }(i)
    }
    
    start := time.Now()
    wg.Wait()
    elapsed := time.Since(start)
    
    // Should take ~2 seconds for 5 requests at 2 req/s
    assert.GreaterOrEqual(t, elapsed, 2*time.Second)
}
```

### 3. Error Path Testing

Test both success and failure paths:

```go
func TestCompleteErrorCoverage(t *testing.T) {
    tests := []struct {
        name        string
        input       Order
        shouldFail  bool
        failureStage string
    }{
        {
            name:       "valid_order",
            input:      Order{ID: "123", Amount: 99.99},
            shouldFail: false,
        },
        {
            name:        "invalid_amount",
            input:       Order{ID: "456", Amount: -10},
            shouldFail:  true,
            failureStage: "validate",
        },
        {
            name:        "payment_failure",
            input:       Order{ID: "789", Amount: 99999}, // Triggers payment failure
            shouldFail:  true,
            failureStage: "payment",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pipeline := buildOrderPipeline()
            _, err := pipeline.Process(context.Background(), tt.input)
            
            if tt.shouldFail {
                require.Error(t, err)
                var pipeErr *pipz.Error[Order]
                require.True(t, errors.As(err, &pipeErr))
                assert.Equal(t, tt.failureStage, string(pipeErr.Stage))
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### 4. Context Cancellation Testing

Ensure processors respect context:

```go
func TestContextPropagation(t *testing.T) {
    // Create a processor that blocks until context is cancelled
    blockingProcessor := pipz.Apply("blocker", func(ctx context.Context, data string) (string, error) {
        <-ctx.Done()
        return "", ctx.Err()
    })
    
    ctx, cancel := context.WithCancel(context.Background())
    
    // Start processing in background
    done := make(chan error, 1)
    go func() {
        _, err := blockingProcessor.Process(ctx, "test")
        done <- err
    }()
    
    // Give it time to start
    time.Sleep(10 * time.Millisecond)
    
    // Cancel context
    cancel()
    
    // Should complete quickly with cancellation error
    select {
    case err := <-done:
        assert.ErrorIs(t, err, context.Canceled)
    case <-time.After(100 * time.Millisecond):
        t.Fatal("Processor did not respect context cancellation")
    }
}
```

### 5. Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestPipelineVariations(t *testing.T) {
    tests := []struct {
        name     string
        pipeline func() pipz.Chainable[int]
        input    int
        expected int
        wantErr  bool
    }{
        {
            name: "simple_transform",
            pipeline: func() pipz.Chainable[int] {
                return pipz.Transform("double", func(_ context.Context, n int) int {
                    return n * 2
                })
            },
            input:    5,
            expected: 10,
        },
        {
            name: "sequence_of_transforms",
            pipeline: func() pipz.Chainable[int] {
                return pipz.NewSequence[int]("math",
                    pipz.Transform("double", func(_ context.Context, n int) int { return n * 2 }),
                    pipz.Transform("add10", func(_ context.Context, n int) int { return n + 10 }),
                )
            },
            input:    5,
            expected: 20, // (5 * 2) + 10
        },
        {
            name: "with_validation",
            pipeline: func() pipz.Chainable[int] {
                return pipz.Apply("validate", func(_ context.Context, n int) (int, error) {
                    if n < 0 {
                        return 0, errors.New("negative not allowed")
                    }
                    return n, nil
                })
            },
            input:   -5,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pipeline := tt.pipeline()
            result, err := pipeline.Process(context.Background(), tt.input)
            
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

## Common Testing Patterns

### Testing Dynamic Pipeline Modification

```go
func TestDynamicPipelineModification(t *testing.T) {
    // Start with basic pipeline
    seq := pipz.NewSequence[string]("dynamic")
    seq.Register(
        pipz.Transform("step1", strings.ToUpper),
    )
    
    // Test initial configuration
    result, _ := seq.Process(context.Background(), "hello")
    assert.Equal(t, "HELLO", result)
    
    // Add processor at runtime
    seq.PushTail(pipz.Transform("step2", func(_ context.Context, s string) string {
        return s + "!"
    }))
    
    // Test modified pipeline
    result, _ = seq.Process(context.Background(), "hello")
    assert.Equal(t, "HELLO!", result)
    
    // Insert processor in middle
    seq.InsertAt(1, pipz.Transform("step1.5", func(_ context.Context, s string) string {
        return "[" + s + "]"
    }))
    
    // Test final configuration
    result, _ = seq.Process(context.Background(), "hello")
    assert.Equal(t, "[HELLO]!", result)
}
```

### Testing Pipeline Composition

```go
func TestPipelineComposition(t *testing.T) {
    // Build reusable sub-pipelines
    validation := pipz.NewSequence[User]("validation",
        pipz.Apply("validate-email", validateEmail),
        pipz.Apply("validate-age", validateAge),
    )
    
    enrichment := pipz.NewSequence[User]("enrichment",
        pipz.Transform("add-metadata", addMetadata),
        pipz.Transform("calculate-score", calculateScore),
    )
    
    // Compose into larger pipeline
    fullPipeline := pipz.NewSequence[User]("user-processing",
        validation,
        enrichment,
        pipz.Effect("save", saveUser),
    )
    
    // Test composed pipeline
    user := User{Email: "test@example.com", Age: 25}
    result, err := fullPipeline.Process(context.Background(), user)
    
    require.NoError(t, err)
    assert.NotEmpty(t, result.Metadata)
    assert.Greater(t, result.Score, 0)
}
```

### Testing Switch Routing

```go
func TestSwitchRouting(t *testing.T) {
    // Create router that processes based on type
    router := pipz.NewSwitch[Request]("request-router",
        func(_ context.Context, req Request) string {
            return req.Type
        },
    ).
    AddRoute("query", pipz.Transform("handle-query", handleQuery)).
    AddRoute("command", pipz.Apply("handle-command", handleCommand)).
    AddRoute("event", pipz.Effect("handle-event", handleEvent)).
    Default(pipz.Transform("handle-unknown", handleUnknown))
    
    tests := []struct {
        reqType  string
        expected string
    }{
        {"query", "query_result"},
        {"command", "command_result"},
        {"event", "event_result"},
        {"unknown", "unknown_result"},
    }
    
    for _, tt := range tests {
        t.Run(tt.reqType, func(t *testing.T) {
            req := Request{Type: tt.reqType, Data: "test"}
            result, err := router.Process(context.Background(), req)
            require.NoError(t, err)
            assert.Contains(t, result.Response, tt.expected)
        })
    }
}
```

## Testing Gotchas

### ❌ Creating Connectors Per Request
```go
// WRONG - New rate limiter per request (useless!)
func processRequest(req Request) Response {
    limiter := pipz.NewRateLimiter("api", 10, 1) // New instance
    return limiter.Process(ctx, req)
}
```

### ✅ Singleton Connectors
```go
// RIGHT - Shared instance maintains state
var apiLimiter = pipz.NewRateLimiter("api", 10, 1)

func processRequest(req Request) Response {
    return apiLimiter.Process(ctx, req)
}
```

### ❌ Shallow Clone Implementation
```go
// WRONG - Shares memory between concurrent processors
func (d Data) Clone() Data {
    return Data{
        ID:    d.ID,
        Items: d.Items, // SHARES SLICE!
        Meta:  d.Meta,  // SHARES MAP!
    }
}
```

### ✅ Deep Clone Implementation
```go
// RIGHT - Complete isolation
func (d Data) Clone() Data {
    items := make([]Item, len(d.Items))
    copy(items, d.Items)
    
    meta := make(map[string]any, len(d.Meta))
    for k, v := range d.Meta {
        meta[k] = v
    }
    
    return Data{
        ID:    d.ID,
        Items: items,
        Meta:  meta,
    }
}
```

### ❌ Not Testing Error Paths
```go
// WRONG - Only tests happy path
func TestPipeline(t *testing.T) {
    pipeline := buildPipeline()
    result, _ := pipeline.Process(ctx, validData)
    assert.Equal(t, expected, result)
}
```

### ✅ Complete Path Coverage
```go
// RIGHT - Tests success and failure
func TestPipeline(t *testing.T) {
    pipeline := buildPipeline()
    
    // Test success
    result, err := pipeline.Process(ctx, validData)
    require.NoError(t, err)
    assert.Equal(t, expected, result)
    
    // Test failure
    _, err = pipeline.Process(ctx, invalidData)
    require.Error(t, err)
    
    var pipeErr *pipz.Error[Data]
    require.True(t, errors.As(err, &pipeErr))
    assert.Equal(t, "validation", string(pipeErr.Stage))
}
```

### ❌ Ignoring Context Cancellation
```go
// WRONG - Doesn't respect context
func (p *SlowProcessor) Process(ctx context.Context, data Data) (Data, error) {
    time.Sleep(5 * time.Second) // Blocks regardless of context
    return process(data)
}
```

### ✅ Context-Aware Processing
```go
// RIGHT - Respects cancellation
func (p *SlowProcessor) Process(ctx context.Context, data Data) (Data, error) {
    select {
    case <-time.After(5 * time.Second):
        return process(data)
    case <-ctx.Done():
        return data, ctx.Err()
    }
}
```

## Summary

The pipz testing package provides comprehensive tools for validating pipeline behavior:

1. **MockProcessor** for controlled testing with configurable behavior
2. **ChaosProcessor** for resilience and fault tolerance testing
3. **Assertion helpers** for verifying processor interactions
4. **Three-tier testing strategy** separating unit, integration, and performance tests
5. **Best practices** for avoiding common pitfalls

Effective testing ensures your pipelines are robust, performant, and handle edge cases gracefully. Use mocks for isolation, chaos for resilience validation, and follow the testing patterns to build reliable data processing systems.