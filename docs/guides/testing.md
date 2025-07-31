# Testing Strategies for pipz

This guide covers best practices and patterns for testing pipz-based applications.

## Table of Contents

1. [Unit Testing Processors](#unit-testing-processors)
2. [Testing Connectors](#testing-connectors)
3. [Integration Testing](#integration-testing)
4. [Table-Driven Tests](#table-driven-tests)
5. [Testing Time-Based Behavior](#testing-time-based-behavior)
6. [Mock Processors](#mock-processors)
7. [Benchmarking Pipelines](#benchmarking-pipelines)
8. [Testing Concurrent Processors](#testing-concurrent-processors)
9. [Property-Based Testing](#property-based-testing)
10. [Testing Pipelines](#testing-pipelines)

## Unit Testing Processors

### Basic Processor Testing

Test individual processors in isolation to ensure they behave correctly:

```go
func TestValidateUser(t *testing.T) {
    validator := pipz.Apply("validate", validateUser)
    
    tests := []struct {
        name    string
        input   User
        want    User
        wantErr bool
    }{
        {
            name:    "valid user",
            input:   User{Email: "test@example.com", Age: 25},
            want:    User{Email: "test@example.com", Age: 25},
            wantErr: false,
        },
        {
            name:    "missing email",
            input:   User{Age: 25},
            want:    User{},
            wantErr: true,
        },
        {
            name:    "invalid age",
            input:   User{Email: "test@example.com", Age: -1},
            want:    User{},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := validator.Process(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Process() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Testing Transform Processors

Transform processors should never fail, making them easier to test:

```go
func TestNormalizeEmail(t *testing.T) {
    normalizer := pipz.Transform("normalize", func(_ context.Context, email string) string {
        return strings.ToLower(strings.TrimSpace(email))
    })
    
    tests := []struct {
        input string
        want  string
    }{
        {"  TEST@EXAMPLE.COM  ", "test@example.com"},
        {"already@normalized.com", "already@normalized.com"},
        {"CAPS@DOMAIN.COM", "caps@domain.com"},
    }
    
    for _, tt := range tests {
        got, _ := normalizer.Process(context.Background(), tt.input)
        if got != tt.want {
            t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
        }
    }
}
```

### Testing Effect Processors

Effect processors perform side effects and return the input unchanged:

```go
func TestAuditLogger(t *testing.T) {
    var logged []string
    
    logger := pipz.Effect("audit", func(_ context.Context, action Action) error {
        logged = append(logged, fmt.Sprintf("%s: %s", action.Type, action.ID))
        return nil
    })
    
    action := Action{Type: "CREATE", ID: "123"}
    result, err := logger.Process(context.Background(), action)
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !reflect.DeepEqual(result, action) {
        t.Error("Effect should not modify input")
    }
    if len(logged) != 1 || logged[0] != "CREATE: 123" {
        t.Errorf("expected audit log entry, got %v", logged)
    }
}
```

## Testing Connectors

### Testing Sequences

```go
func TestUserPipeline(t *testing.T) {
    pipeline := pipz.NewSequence[User]("user-pipeline")
    pipeline.Register(
        pipz.Apply("validate", validateUser),
        pipz.Transform("normalize", normalizeUser),
        pipz.Effect("audit", auditUser),
    )
    
    user := User{
        Email: "  John.Doe@Example.COM  ",
        Name:  "John Doe",
        Age:   30,
    }
    
    result, err := pipeline.Process(context.Background(), user)
    if err != nil {
        t.Fatalf("pipeline failed: %v", err)
    }
    
    // Check normalization was applied
    if result.Email != "john.doe@example.com" {
        t.Errorf("email not normalized: %s", result.Email)
    }
}
```

### Testing Error Paths

Use pipz's rich error information to verify error handling:

```go
func TestPipelineErrorHandling(t *testing.T) {
    pipeline := pipz.NewSequence[int]("math-pipeline")
    pipeline.Register(
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
    
    expectedPath := []string{"math-pipeline", "divide"}
    if !reflect.DeepEqual(pipeErr.Path, expectedPath) {
        t.Errorf("error path = %v, want %v", pipeErr.Path, expectedPath)
    }
    
    if pipeErr.InputData != 0 {
        t.Errorf("error input = %v, want 0", pipeErr.InputData)
    }
}
```

## Integration Testing

Test complete pipelines with external dependencies:

```go
func TestPaymentProcessingPipeline(t *testing.T) {
    // Set up test doubles
    mockPaymentGateway := &MockPaymentGateway{
        responses: map[string]error{
            "valid-card":   nil,
            "expired-card": errors.New("card expired"),
        },
    }
    
    pipeline := createPaymentPipeline(mockPaymentGateway)
    
    tests := []struct {
        name    string
        payment Payment
        wantErr bool
        wantLog string
    }{
        {
            name: "successful payment",
            payment: Payment{
                CardToken: "valid-card",
                Amount:    100.00,
            },
            wantErr: false,
            wantLog: "payment processed",
        },
        {
            name: "expired card",
            payment: Payment{
                CardToken: "expired-card",
                Amount:    50.00,
            },
            wantErr: true,
            wantLog: "payment failed: card expired",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := pipeline.Process(context.Background(), tt.payment)
            if (err != nil) != tt.wantErr {
                t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
            }
            // Verify expected side effects
            if !strings.Contains(mockPaymentGateway.lastLog, tt.wantLog) {
                t.Errorf("expected log %q, got %q", tt.wantLog, mockPaymentGateway.lastLog)
            }
        })
    }
}
```

## Table-Driven Tests

Structure complex test scenarios with table-driven tests:

```go
func TestDataTransformationPipeline(t *testing.T) {
    pipeline := createDataPipeline()
    
    tests := []struct {
        name      string
        input     Record
        want      Record
        wantErr   bool
        errPath   []string
        errMsg    string
    }{
        {
            name: "valid record",
            input: Record{
                ID:   "123",
                Data: map[string]any{"price": 100.0},
            },
            want: Record{
                ID:   "123",
                Data: map[string]any{"price": 100.0, "tax": 10.0},
            },
            wantErr: false,
        },
        {
            name: "missing required field",
            input: Record{
                ID:   "456",
                Data: map[string]any{},
            },
            wantErr: true,
            errPath: []string{"data-pipeline", "validate"},
            errMsg:  "missing required field: price",
        },
        {
            name: "negative price",
            input: Record{
                ID:   "789",
                Data: map[string]any{"price": -50.0},
            },
            wantErr: true,
            errPath: []string{"data-pipeline", "validate"},
            errMsg:  "price must be positive",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := pipeline.Process(context.Background(), tt.input)
            
            if (err != nil) != tt.wantErr {
                t.Fatalf("Process() error = %v, wantErr %v", err, tt.wantErr)
            }
            
            if err != nil {
                var pipeErr *pipz.Error[Record]
                if errors.As(err, &pipeErr) {
                    if !reflect.DeepEqual(pipeErr.Path, tt.errPath) {
                        t.Errorf("error path = %v, want %v", pipeErr.Path, tt.errPath)
                    }
                    if !strings.Contains(pipeErr.Err.Error(), tt.errMsg) {
                        t.Errorf("error message = %q, want %q", pipeErr.Err.Error(), tt.errMsg)
                    }
                }
            } else {
                if !reflect.DeepEqual(got, tt.want) {
                    t.Errorf("Process() = %v, want %v", got, tt.want)
                }
            }
        })
    }
}
```

## Testing Time-Based Behavior

### Testing Timeouts

Use short timeouts and context-aware operations for predictable tests:

```go
func TestTimeoutBehavior(t *testing.T) {
    slowProcessor := pipz.Apply("slow", func(ctx context.Context, v int) (int, error) {
        select {
        case <-time.After(100 * time.Millisecond):
            return v * 2, nil
        case <-ctx.Done():
            return 0, ctx.Err()
        }
    })
    
    timeout := pipz.NewTimeout("test", slowProcessor, 50*time.Millisecond)
    
    start := time.Now()
    _, err := timeout.Process(context.Background(), 42)
    elapsed := time.Since(start)
    
    var pipeErr *pipz.Error[int]
    if !errors.As(err, &pipeErr) || !pipeErr.Timeout {
        t.Error("expected timeout error")
    }
    
    // Verify it actually timed out around 50ms
    if elapsed < 40*time.Millisecond || elapsed > 70*time.Millisecond {
        t.Errorf("timeout took %v, expected ~50ms", elapsed)
    }
}
```

### Testing Retry Logic

```go
func TestRetryBehavior(t *testing.T) {
    attempts := 0
    processor := pipz.Apply("flaky", func(_ context.Context, v int) (int, error) {
        attempts++
        if attempts < 3 {
            return 0, errors.New("temporary failure")
        }
        return v * 2, nil
    })
    
    retry := pipz.NewRetry("test", processor, 3)
    
    result, err := retry.Process(context.Background(), 42)
    if err != nil {
        t.Fatalf("unexpected error after retries: %v", err)
    }
    
    if result != 84 {
        t.Errorf("got %d, want 84", result)
    }
    
    if attempts != 3 {
        t.Errorf("expected 3 attempts, got %d", attempts)
    }
}
```

### Testing Backoff

```go
func TestBackoffTiming(t *testing.T) {
    var timestamps []time.Time
    
    processor := pipz.Apply("track", func(_ context.Context, v int) (int, error) {
        timestamps = append(timestamps, time.Now())
        if len(timestamps) < 3 {
            return 0, errors.New("retry me")
        }
        return v, nil
    })
    
    backoff := pipz.NewBackoff("test", processor, 3, 10*time.Millisecond)
    
    _, err := backoff.Process(context.Background(), 42)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify exponential backoff timing
    if len(timestamps) != 3 {
        t.Fatalf("expected 3 attempts, got %d", len(timestamps))
    }
    
    // First retry after ~10ms
    gap1 := timestamps[1].Sub(timestamps[0])
    if gap1 < 8*time.Millisecond || gap1 > 15*time.Millisecond {
        t.Errorf("first backoff = %v, expected ~10ms", gap1)
    }
    
    // Second retry after ~20ms (exponential)
    gap2 := timestamps[2].Sub(timestamps[1])
    if gap2 < 18*time.Millisecond || gap2 > 25*time.Millisecond {
        t.Errorf("second backoff = %v, expected ~20ms", gap2)
    }
}
```

## Mock Processors

Create test doubles for external dependencies:

```go
type MockProcessor[T any] struct {
    mu       sync.Mutex
    calls    []T
    results  []T
    errors   []error
    callIdx  int
}

func (m *MockProcessor[T]) Process(ctx context.Context, v T) (T, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.calls = append(m.calls, v)
    
    if m.callIdx < len(m.results) {
        result := m.results[m.callIdx]
        var err error
        if m.callIdx < len(m.errors) {
            err = m.errors[m.callIdx]
        }
        m.callIdx++
        return result, err
    }
    
    var zero T
    return zero, errors.New("no more mock results")
}

func (m *MockProcessor[T]) Name() pipz.Name {
    return "mock"
}

func (m *MockProcessor[T]) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.calls)
}

func (m *MockProcessor[T]) CalledWith(idx int) T {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.calls[idx]
}

// Usage in tests
func TestWithMockProcessor(t *testing.T) {
    mock := &MockProcessor[Order]{
        results: []Order{
            {ID: "123", Total: 100.0},
            {ID: "456", Total: 200.0},
        },
        errors: []error{nil, nil},
    }
    
    pipeline := pipz.NewSequence[Order]("test")
    pipeline.Register(
        pipz.Transform("prepare", prepareOrder),
        mock, // Use mock as a Chainable
        pipz.Effect("notify", sendNotification),
    )
    
    _, err := pipeline.Process(context.Background(), Order{ID: "123"})
    if err != nil {
        t.Fatal(err)
    }
    
    if mock.CallCount() != 1 {
        t.Errorf("expected 1 call, got %d", mock.CallCount())
    }
}
```

## Benchmarking Pipelines

Measure pipeline performance:

```go
func BenchmarkUserValidationPipeline(b *testing.B) {
    pipeline := pipz.NewSequence[User]("user-validation",
        pipz.Apply("parse", parseUser),
        pipz.Effect("validate", validateUser),
        pipz.Transform("normalize", normalizeUser),
        pipz.Apply("enrich", enrichUser),
    )
    
    user := User{
        Email: "test@example.com",
        Name:  "Test User",
        Age:   25,
    }
    
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := pipeline.Process(ctx, user)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkPipelineScaling(b *testing.B) {
    sizes := []int{1, 5, 10, 20, 50}
    
    for _, size := range sizes {
        b.Run(fmt.Sprintf("processors_%d", size), func(b *testing.B) {
            seq := pipz.NewSequence[int]("bench")
            
            for i := 0; i < size; i++ {
                seq.Register(pipz.Transform(fmt.Sprintf("step%d", i), 
                    func(_ context.Context, n int) int {
                        return n + 1
                    }))
            }
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, err := seq.Process(context.Background(), 0)
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

## Testing Concurrent Processors

Test types that implement `Cloner[T]`:

```go
type Report struct {
    ID      string
    Data    []int
    Results map[string]float64
    mu      sync.Mutex
}

func (r Report) Clone() Report {
    data := make([]int, len(r.Data))
    copy(data, r.Data)
    
    results := make(map[string]float64, len(r.Results))
    for k, v := range r.Results {
        results[k] = v
    }
    
    return Report{
        ID:      r.ID,
        Data:    data,
        Results: results,
    }
}

func TestConcurrentProcessing(t *testing.T) {
    // Track which processor ran
    var callOrder sync.Map
    
    makeProcessor := func(name pipz.Name, delay time.Duration) pipz.Chainable[Report] {
        return pipz.Apply(name, func(_ context.Context, r Report) (Report, error) {
            time.Sleep(delay)
            callOrder.Store(name, time.Now())
            r.Results[name] = float64(len(r.Data))
            return r, nil
        })
    }
    
    concurrent := pipz.NewConcurrent[Report]("parallel",
        makeProcessor("fast", 10*time.Millisecond),
        makeProcessor("medium", 50*time.Millisecond),
        makeProcessor("slow", 100*time.Millisecond),
    )
    
    report := Report{
        ID:      "test",
        Data:    []int{1, 2, 3, 4, 5},
        Results: make(map[string]float64),
    }
    
    start := time.Now()
    result, err := concurrent.Process(context.Background(), report)
    elapsed := time.Since(start)
    
    if err != nil {
        t.Fatal(err)
    }
    
    // Should complete in ~100ms (slowest processor)
    if elapsed > 120*time.Millisecond {
        t.Errorf("took too long: %v", elapsed)
    }
    
    // Verify all processors ran
    for _, name := range []string{"fast", "medium", "slow"} {
        if _, ok := callOrder.Load(name); !ok {
            t.Errorf("processor %s didn't run", name)
        }
    }
    
    // Original data should be unchanged
    if !reflect.DeepEqual(result.Data, report.Data) {
        t.Error("original data was modified")
    }
}
```

## Property-Based Testing

Use property-based testing for invariants:

```go
func TestPipelineProperties(t *testing.T) {
    // Property: Transform processors preserve structure
    pipeline := pipz.NewSequence[int]("math-pipeline",
        pipz.Transform("double", func(_ context.Context, n int) int {
            return n * 2
        }),
        pipz.Transform("increment", func(_ context.Context, n int) int {
            return n + 1
        }),
    )
    
    // Test with random inputs
    for i := 0; i < 100; i++ {
        input := rand.Intn(1000)
        result, err := pipeline.Process(context.Background(), input)
        
        if err != nil {
            t.Fatal(err)
        }
        
        // Verify the mathematical property
        expected := (input * 2) + 1
        if result != expected {
            t.Errorf("f(%d) = %d, expected %d", input, result, expected)
        }
    }
}

func TestErrorPropagation(t *testing.T) {
    // Property: First error stops execution
    callCount := 0
    
    pipeline := pipz.NewSequence[int]("error-test",
        pipz.Effect("count1", func(_ context.Context, _ int) error {
            callCount++
            return nil
        }),
        pipz.Apply("fail", func(_ context.Context, n int) (int, error) {
            return 0, errors.New("intentional error")
        }),
        pipz.Effect("count2", func(_ context.Context, _ int) error {
            callCount++
            return nil
        }),
    )
    
    _, err := pipeline.Process(context.Background(), 42)
    
    if err == nil {
        t.Fatal("expected error")
    }
    
    if callCount != 1 {
        t.Errorf("expected 1 processor to run, but %d ran", callCount)
    }
}
```

## Testing Pipelines

The beauty of pipz is that testing complex pipeline logic is straightforward because processors are just functions doing one thing well. You can test individual processors in isolation, then test pipeline composition with mock processors.

### Testing Individual Processors

Since processors are pure functions, they're easy to test:

```go
func TestValidateEmail(t *testing.T) {
    validator := pipz.Apply("validate_email", func(ctx context.Context, u User) (User, error) {
        if !strings.Contains(u.Email, "@") {
            return u, errors.New("invalid email")
        }
        return u, nil
    })
    
    // Test valid case
    user := User{Email: "test@example.com"}
    result, err := validator.Process(context.Background(), user)
    assert.NoError(t, err)
    assert.Equal(t, user, result)
    
    // Test invalid case
    user = User{Email: "invalid"}
    _, err = validator.Process(context.Background(), user)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid email")
}
```

### Testing Pipeline Logic with Mock Processors

For complex pipeline logic, create mock processors that simulate different behaviors:

```go
func TestOrderProcessingPipeline(t *testing.T) {
    // Create mock processors for testing
    var (
        validateCalled = false
        priceCalled = false
        paymentCalled = false
    )
    
    // Mock that tracks calls
    mockValidate := pipz.Apply("mock_validate", func(ctx context.Context, o Order) (Order, error) {
        validateCalled = true
        if o.CustomerID == "" {
            return o, errors.New("missing customer")
        }
        return o, nil
    })
    
    // Mock that modifies data predictably
    mockCalculatePrice := pipz.Transform("mock_price", func(ctx context.Context, o Order) Order {
        priceCalled = true
        o.Total = 100.0 // Predictable value for testing
        return o
    })
    
    // Mock that can simulate failures
    mockPayment := pipz.Apply("mock_payment", func(ctx context.Context, o Order) (Order, error) {
        paymentCalled = true
        if o.Total > 1000 {
            return o, errors.New("payment declined")
        }
        o.PaymentID = "mock-payment-123"
        return o
    })
    
    // Build pipeline with mocks
    pipeline := pipz.NewSequence[Order]("test-pipeline")
    pipeline.Register(mockValidate, mockCalculatePrice, mockPayment)
    
    t.Run("successful order", func(t *testing.T) {
        // Reset call tracking
        validateCalled, priceCalled, paymentCalled = false, false, false
        
        order := Order{CustomerID: "123", Items: []Item{{Name: "test"}}}
        result, err := pipeline.Process(context.Background(), order)
        
        // Verify pipeline succeeded
        assert.NoError(t, err)
        assert.Equal(t, "mock-payment-123", result.PaymentID)
        assert.Equal(t, 100.0, result.Total)
        
        // Verify all processors were called
        assert.True(t, validateCalled, "validate should be called")
        assert.True(t, priceCalled, "price calculation should be called")
        assert.True(t, paymentCalled, "payment should be called")
    })
    
    t.Run("validation failure stops pipeline", func(t *testing.T) {
        validateCalled, priceCalled, paymentCalled = false, false, false
        
        // Invalid order
        order := Order{CustomerID: "", Items: []Item{{Name: "test"}}}
        _, err := pipeline.Process(context.Background(), order)
        
        // Verify error propagation
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "missing customer")
        
        // Verify execution stopped at validation
        assert.True(t, validateCalled, "validate should be called")
        assert.False(t, priceCalled, "price should not be called after validation fails")
        assert.False(t, paymentCalled, "payment should not be called after validation fails")
    })
}
```

### Testing Pipeline Composition Patterns

Test different pipeline compositions to verify routing and conditional logic:

```go
func TestOrderTypeRouting(t *testing.T) {
    // Mock processors for different order types
    standardProcessor := pipz.Transform("standard", func(ctx context.Context, o Order) Order {
        o.ProcessingType = "standard"
        return o
    })
    
    expressProcessor := pipz.Transform("express", func(ctx context.Context, o Order) Order {
        o.ProcessingType = "express"
        o.ShippingCost = 25.0
        return o
    })
    
    // Create router
    router := pipz.NewSwitch("order-type-router",
        func(ctx context.Context, o Order) string {
            if o.ExpressShipping {
                return "express"
            }
            return "standard"
        },
    )
    router.AddRoute("standard", standardProcessor)
    router.AddRoute("express", expressProcessor)
    
    t.Run("standard order routing", func(t *testing.T) {
        order := Order{ExpressShipping: false}
        result, err := router.Process(context.Background(), order)
        
        assert.NoError(t, err)
        assert.Equal(t, "standard", result.ProcessingType)
        assert.Equal(t, 0.0, result.ShippingCost)
    })
    
    t.Run("express order routing", func(t *testing.T) {
        order := Order{ExpressShipping: true}
        result, err := router.Process(context.Background(), order)
        
        assert.NoError(t, err)
        assert.Equal(t, "express", result.ProcessingType)
        assert.Equal(t, 25.0, result.ShippingCost)
    })
}
```

### Testing Error Recovery Pipelines

You can even test error recovery pipelines in isolation:

```go
func TestPaymentErrorRecovery(t *testing.T) {
    var (
        errorLogged = false
        customerNotified = false
        retryScheduled = false
    )
    
    // Mock error recovery pipeline
    errorPipeline := pipz.NewSequence[*pipz.Error[Order]]("error-recovery",
        // Mock error logging
        pipz.Effect("log_error", func(ctx context.Context, err *pipz.Error[Order]) error {
            errorLogged = true
            return nil
        }),
        
        // Mock customer notification
        pipz.Effect("notify_customer", func(ctx context.Context, err *pipz.Error[Order]) error {
            customerNotified = true
            return nil
        }),
        
        // Mock retry scheduling for timeouts
        pipz.Apply("schedule_retry", func(ctx context.Context, err *pipz.Error[Order]) (*pipz.Error[Order], error) {
            if err.Timeout {
                retryScheduled = true
            }
            return err, nil
        }),
    )
    
    t.Run("timeout error triggers retry", func(t *testing.T) {
        errorLogged, customerNotified, retryScheduled = false, false, false
        
        // Create mock error
        testError := &pipz.Error[Order]{
            InputData: Order{ID: "123", CustomerID: "customer-456"},
            Err:       errors.New("connection timeout"),
            Timeout:   true,
            Path:      []string{"payment-pipeline", "process-payment"},
        }
        
        result, err := errorPipeline.Process(context.Background(), testError)
        
        assert.NoError(t, err)
        assert.True(t, errorLogged, "error should be logged")
        assert.True(t, customerNotified, "customer should be notified")
        assert.True(t, retryScheduled, "retry should be scheduled for timeout")
        assert.Equal(t, "123", result.InputData.ID)
    })
    
    t.Run("non-timeout error skips retry", func(t *testing.T) {
        errorLogged, customerNotified, retryScheduled = false, false, false
        
        testError := &pipz.Error[Order]{
            InputData: Order{ID: "123"},
            Err:       errors.New("payment declined"),
            Timeout:   false,
        }
        
        _, err := errorPipeline.Process(context.Background(), testError)
        
        assert.NoError(t, err)
        assert.True(t, errorLogged, "error should be logged")
        assert.True(t, customerNotified, "customer should be notified")
        assert.False(t, retryScheduled, "retry should not be scheduled for non-timeout")
    })
}
```

### Key Testing Insights

1. **Processors are just functions** - Easy to test in isolation with clear inputs/outputs
2. **Mock processors for pipeline logic** - Test routing, conditional behavior, and execution flow
3. **Error pipelines are testable too** - Verify recovery logic with mock errors
4. **Composition is testable** - Test that different pipeline configurations work correctly
5. **Call tracking reveals execution flow** - Verify which processors ran and in what order

This approach makes even complex pipeline logic straightforward to test and verify.

## Best Practices

1. **Test processors in isolation** - Each processor should have its own unit tests
2. **Use table-driven tests** - Makes it easy to add test cases
3. **Test error paths explicitly** - Ensure error handling works correctly
4. **Mock external dependencies** - Keep tests fast and deterministic
5. **Use meaningful test names** - Describe what the test verifies
6. **Test with context cancellation** - Ensure processors respect context
7. **Benchmark critical paths** - Monitor performance regressions
8. **Test concurrent behavior** - Verify thread safety and race conditions
9. **Use property-based testing** - Verify invariants hold for all inputs
10. **Keep tests focused** - Each test should verify one behavior

## Testing Checklist

When testing pipz pipelines, ensure you cover:

- [ ] Happy path execution
- [ ] Error handling and propagation
- [ ] Context cancellation
- [ ] Context timeout
- [ ] Empty/nil input handling
- [ ] Concurrent access (for stateful processors)
- [ ] Performance characteristics
- [ ] Memory usage (no leaks)
- [ ] Panic recovery (if applicable)
- [ ] Integration with external systems