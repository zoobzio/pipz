# Testing Pipelines

Comprehensive guide to testing pipz pipelines and processors.

## Testing Philosophy

pipz is designed for testability:
- Every processor is independently testable
- Pipelines are composable test units
- Type safety catches many errors at compile time
- No mocking frameworks required

## Unit Testing Processors

### Testing Individual Processors

```go
func TestValidateOrder(t *testing.T) {
    tests := []struct {
        name    string
        input   Order
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid order",
            input: Order{
                ID:    "123",
                Items: []Item{{Name: "Widget", Price: 9.99}},
                Total: 9.99,
            },
            wantErr: false,
        },
        {
            name: "empty order",
            input: Order{
                ID:    "124",
                Items: []Item{},
            },
            wantErr: true,
            errMsg:  "order must have items",
        },
        {
            name: "negative total",
            input: Order{
                ID:    "125",
                Items: []Item{{Name: "Widget", Price: -5}},
                Total: -5,
            },
            wantErr: true,
            errMsg:  "invalid total",
        },
    }
    
    validator := pipz.Apply("validate", validateOrder)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := validator.Process(context.Background(), tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.input, result)
            }
        })
    }
}
```

### Testing Transformations

```go
func TestEnrichOrder(t *testing.T) {
    enricher := pipz.Transform("enrich", enrichOrder)
    
    input := Order{
        ID:    "123",
        Items: []Item{{Name: "Widget"}},
    }
    
    result, err := enricher.Process(context.Background(), input)
    
    assert.NoError(t, err)
    assert.NotZero(t, result.ProcessedAt)
    assert.NotEmpty(t, result.Version)
    assert.Equal(t, "USD", result.Currency) // Default currency
}
```

### Testing Side Effects

```go
func TestEmailNotification(t *testing.T) {
    // Capture side effects
    var sentEmails []Email
    
    sender := pipz.Effect("send_email", func(ctx context.Context, order Order) error {
        sentEmails = append(sentEmails, Email{
            To:      order.CustomerEmail,
            Subject: fmt.Sprintf("Order %s confirmed", order.ID),
        })
        return nil
    })
    
    order := Order{
        ID:            "123",
        CustomerEmail: "test@example.com",
    }
    
    result, err := sender.Process(context.Background(), order)
    
    assert.NoError(t, err)
    assert.Equal(t, order, result) // Effect doesn't modify data
    assert.Len(t, sentEmails, 1)
    assert.Equal(t, "test@example.com", sentEmails[0].To)
}
```

## Testing Pipelines

### Integration Testing

```go
func TestOrderPipeline(t *testing.T) {
    pipeline := pipz.Sequential(
        pipz.Apply("validate", validateOrder),
        pipz.Transform("calculate_tax", calculateTax),
        pipz.Apply("charge_payment", chargePayment),
        pipz.Effect("send_confirmation", sendConfirmation),
    )
    
    t.Run("successful order", func(t *testing.T) {
        order := Order{
            ID:       "123",
            Items:    []Item{{Name: "Widget", Price: 100}},
            Total:    100,
            Customer: Customer{Email: "test@example.com"},
        }
        
        result, err := pipeline.Process(context.Background(), order)
        
        assert.NoError(t, err)
        assert.Equal(t, "completed", result.Status)
        assert.NotZero(t, result.Tax)
        assert.NotEmpty(t, result.TransactionID)
    })
    
    t.Run("invalid order rejected", func(t *testing.T) {
        order := Order{} // Empty order
        
        _, err := pipeline.Process(context.Background(), order)
        
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "validate")
    })
}
```

### Testing Error Scenarios

```go
func TestPipelineErrorHandling(t *testing.T) {
    // Create a processor that fails intermittently
    var attempts int
    flaky := pipz.Apply("flaky", func(ctx context.Context, data string) (string, error) {
        attempts++
        if attempts < 3 {
            return "", errors.New("temporary failure")
        }
        return data + "_processed", nil
    })
    
    t.Run("retry recovers from transient errors", func(t *testing.T) {
        attempts = 0
        pipeline := pipz.Retry(flaky, 3)
        
        result, err := pipeline.Process(context.Background(), "test")
        
        assert.NoError(t, err)
        assert.Equal(t, "test_processed", result)
        assert.Equal(t, 3, attempts)
    })
    
    t.Run("fallback activates on primary failure", func(t *testing.T) {
        primary := pipz.Apply("primary", func(ctx context.Context, data string) (string, error) {
            return "", errors.New("primary failed")
        })
        
        fallback := pipz.Apply("fallback", func(ctx context.Context, data string) (string, error) {
            return data + "_fallback", nil
        })
        
        pipeline := pipz.Fallback(primary, fallback)
        
        result, err := pipeline.Process(context.Background(), "test")
        
        assert.NoError(t, err)
        assert.Equal(t, "test_fallback", result)
    })
}
```

## Testing Concurrent Pipelines

```go
func TestConcurrentProcessing(t *testing.T) {
    // Track which processors ran
    var mu sync.Mutex
    executed := make(map[string]bool)
    
    makeTracker := func(name string) pipz.Chainable[TestData] {
        return pipz.Effect(name, func(ctx context.Context, data TestData) error {
            mu.Lock()
            executed[name] = true
            mu.Unlock()
            
            // Simulate work
            time.Sleep(10 * time.Millisecond)
            return nil
        })
    }
    
    pipeline := pipz.Concurrent(
        makeTracker("email"),
        makeTracker("sms"),
        makeTracker("push"),
    )
    
    // TestData must implement Cloner
    data := TestData{ID: "123"}
    
    start := time.Now()
    result, err := pipeline.Process(context.Background(), data)
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Equal(t, data, result) // Unchanged
    
    // All processors should have executed
    assert.True(t, executed["email"])
    assert.True(t, executed["sms"])
    assert.True(t, executed["push"])
    
    // Should run in parallel (faster than sequential)
    assert.Less(t, duration, 25*time.Millisecond)
}
```

## Testing with Context

### Timeout Testing

```go
func TestPipelineTimeout(t *testing.T) {
    slowProcessor := pipz.Apply("slow", func(ctx context.Context, data string) (string, error) {
        select {
        case <-time.After(100 * time.Millisecond):
            return data, nil
        case <-ctx.Done():
            return "", ctx.Err()
        }
    })
    
    pipeline := pipz.Timeout(slowProcessor, 50*time.Millisecond)
    
    _, err := pipeline.Process(context.Background(), "test")
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "timeout")
}
```

### Cancellation Testing

```go
func TestPipelineCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    pipeline := pipz.Apply("cancellable", func(ctx context.Context, data string) (string, error) {
        select {
        case <-time.After(100 * time.Millisecond):
            return data, nil
        case <-ctx.Done():
            return "", ctx.Err()
        }
    })
    
    // Cancel after 50ms
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()
    
    _, err := pipeline.Process(ctx, "test")
    
    assert.Error(t, err)
    assert.ErrorIs(t, err, context.Canceled)
}
```

## Table-Driven Tests

```go
func TestPaymentRouting(t *testing.T) {
    router := createPaymentRouter()
    
    tests := []struct {
        name           string
        payment        Payment
        expectedRoute  string
        expectedError  bool
    }{
        {
            name: "credit card to stripe",
            payment: Payment{
                Method: "credit_card",
                Amount: 50.00,
            },
            expectedRoute: "stripe",
        },
        {
            name: "high value to manual review",
            payment: Payment{
                Method: "wire",
                Amount: 100000,
            },
            expectedRoute: "manual_review",
        },
        {
            name: "crypto to coinbase",
            payment: Payment{
                Method: "bitcoin",
                Amount: 500,
            },
            expectedRoute: "coinbase",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := router.Process(context.Background(), tt.payment)
            
            if tt.expectedError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedRoute, result.ProcessedBy)
            }
        })
    }
}
```

## Benchmarking

```go
func BenchmarkPipelineTypes(b *testing.B) {
    order := Order{
        ID:    "bench-123",
        Items: []Item{{Name: "Widget", Price: 99.99}},
    }
    
    b.Run("Sequential", func(b *testing.B) {
        pipeline := pipz.Sequential(
            transform1,
            transform2,
            transform3,
        )
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            pipeline.Process(context.Background(), order)
        }
    })
    
    b.Run("WithRetry", func(b *testing.B) {
        pipeline := pipz.Retry(
            pipz.Sequential(transform1, transform2, transform3),
            3,
        )
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            pipeline.Process(context.Background(), order)
        }
    })
}
```

## Test Helpers

### Mock Processors

```go
func mockProcessor[T any](name string, err error) pipz.Chainable[T] {
    return pipz.Apply(name, func(ctx context.Context, data T) (T, error) {
        return data, err
    })
}

func mockTransform[T any](name string, transform func(T) T) pipz.Chainable[T] {
    return pipz.Transform(name, func(ctx context.Context, data T) T {
        return transform(data)
    })
}
```

### Test Fixtures

```go
func newTestOrder() Order {
    return Order{
        ID:       uuid.New().String(),
        Items:    []Item{{Name: "Test Item", Price: 10.00}},
        Customer: Customer{Email: "test@example.com"},
        Status:   "pending",
    }
}

func newTestPipeline() pipz.Chainable[Order] {
    return pipz.Sequential(
        mockProcessor[Order]("step1", nil),
        mockProcessor[Order]("step2", nil),
        mockProcessor[Order]("step3", nil),
    )
}
```

## Testing Best Practices

1. **Test at Multiple Levels**
   - Unit test individual processors
   - Integration test complete pipelines
   - End-to-end test with real dependencies

2. **Test Error Paths**
   - Validation failures
   - Network errors
   - Timeout scenarios
   - Cancellation handling

3. **Use Table-Driven Tests**
   - Clear test cases
   - Easy to add new scenarios
   - Better test coverage

4. **Benchmark Critical Paths**
   - Measure performance regularly
   - Compare optimization attempts
   - Track allocations

5. **Test Concurrency**
   - Race conditions
   - Proper cloning
   - Context propagation

## Next Steps

- [Best Practices](./best-practices.md) - Testing in production
- [Error Recovery](./error-recovery.md) - Testing error scenarios
- [Performance](./performance.md) - Performance testing