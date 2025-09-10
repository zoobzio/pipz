# Performance Optimization Guide

Learn how to build high-performance pipelines with pipz.

## Performance Principles

pipz is designed for performance:
- Minimal allocations in core operations
- No reflection or runtime type assertions
- Low interface call overhead
- Efficient error propagation without excessive wrapping

## Benchmarking Pipelines

Always measure before optimizing:

```go
func BenchmarkPipeline(b *testing.B) {
    pipeline := pipz.NewSequence[Order]("benchmark-pipeline",
        validateOrder,
        calculateTax,
        applyDiscount,
    )
    
    order := Order{
        Items: []Item{{Price: 99.99}},
        Country: "US",
    }
    
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := pipeline.Process(ctx, order)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

Run with:
```bash
go test -bench=BenchmarkPipeline -benchmem
```

## Optimization Strategies

### 1. Minimize Allocations

Reuse objects where possible:

```go
// Bad: Allocates on every call
func enrichOrder(ctx context.Context, order Order) Order {
    order.Metadata = make(map[string]string) // New allocation
    order.Metadata["processed"] = "true"
    return order
}

// Good: Reuse existing map
func enrichOrder(ctx context.Context, order Order) Order {
    if order.Metadata == nil {
        order.Metadata = make(map[string]string, 1)
    }
    order.Metadata["processed"] = "true"
    return order
}

// Better: Pre-size collections
func processOrders(orders []Order) []Order {
    result := make([]Order, 0, len(orders)) // Pre-sized
    for _, order := range orders {
        if order.Valid {
            result = append(result, order)
        }
    }
    return result
}
```

### 2. Efficient Cloning

For concurrent processing, optimize your Clone implementation:

```go
// Inefficient clone
func (o Order) Clone() Order {
    var clone Order
    json.Unmarshal(json.Marshal(o)) // Slow!
    return clone
}

// Efficient clone
func (o Order) Clone() Order {
    // Copy slice with pre-allocation
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    
    // Copy map if needed
    var metadata map[string]string
    if o.Metadata != nil {
        metadata = make(map[string]string, len(o.Metadata))
        for k, v := range o.Metadata {
            metadata[k] = v
        }
    }
    
    return Order{
        ID:       o.ID,
        Customer: o.Customer,
        Items:    items,
        Total:    o.Total,
        Metadata: metadata,
    }
}
```

### 3. Batching Operations

Process multiple items together:

```go
// Individual processing (slow)
for _, order := range orders {
    enriched, _ := enrichOrder(ctx, order)
    saved, _ := saveOrder(ctx, enriched)
    results = append(results, saved)
}

// Batch processing (fast)
type OrderBatch []Order

func processBatch(ctx context.Context, batch OrderBatch) (OrderBatch, error) {
    // Enrich all at once
    for i := range batch {
        batch[i] = enrichOrder(ctx, batch[i])
    }
    
    // Save in bulk
    if err := saveOrdersBulk(ctx, batch); err != nil {
        return batch, err
    }
    
    return batch, nil
}
```

### 4. Parallel Processing

Use concurrent processing wisely:

```go
// Process orders in parallel with controlled concurrency
func processOrdersConcurrent(orders []Order) []Order {
    // Limit concurrency to CPU count
    workers := runtime.NumCPU()
    
    // Create work channel
    work := make(chan Order, len(orders))
    results := make(chan Order, len(orders))
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            pipeline := createOrderPipeline()
            ctx := context.Background()
            
            for order := range work {
                result, err := pipeline.Process(ctx, order)
                if err != nil {
                    log.Printf("Failed to process order %s: %v", order.ID, err)
                    continue
                }
                results <- result
            }
        }()
    }
    
    // Send work
    for _, order := range orders {
        work <- order
    }
    close(work)
    
    // Wait and collect
    wg.Wait()
    close(results)
    
    // Collect results
    processed := make([]Order, 0, len(orders))
    for result := range results {
        processed = append(processed, result)
    }
    
    return processed
}
```

### 5. Caching

Cache expensive operations:

```go
type CachedProcessor[T any, K comparable] struct {
    processor pipz.Chainable[T]
    keyFunc   func(T) K
    cache     sync.Map
    ttl       time.Duration
}

type cacheEntry[T any] struct {
    value     T
    timestamp time.Time
}

func (cp *CachedProcessor[T, K]) Process(ctx context.Context, data T) (T, error) {
    key := cp.keyFunc(data)
    
    // Check cache
    if cached, ok := cp.cache.Load(key); ok {
        entry := cached.(cacheEntry[T])
        if time.Since(entry.timestamp) < cp.ttl {
            return entry.value, nil
        }
        cp.cache.Delete(key) // Expired
    }
    
    // Process and cache
    result, err := cp.processor.Process(ctx, data)
    if err != nil {
        return data, err
    }
    
    cp.cache.Store(key, cacheEntry[T]{
        value:     result,
        timestamp: time.Now(),
    })
    
    return result, nil
}
```

### 6. Pipeline Complexity

Simpler pipelines are faster:

```go
// Complex: Many small steps
complex := pipz.NewSequence[Order]("complex-pipeline",
    step1, step2, step3, step4, step5,
    step6, step7, step8, step9, step10,
)

// Simple: Combine related operations
simple := pipz.NewSequence[Order]("simple-pipeline",
    validateAndNormalize,  // Combined steps 1-3
    enrichAndTransform,    // Combined steps 4-7
    saveAndNotify,        // Combined steps 8-10
)
```

## Performance Patterns

### Pattern: Zero-Copy Processing

Modify data in-place when safe:

```go
// For types that don't need cloning
func processInPlace(ctx context.Context, data *LargeData) (*LargeData, error) {
    // Modify the pointer directly
    data.Processed = true
    data.Timestamp = time.Now()
    return data, nil
}
```

### Pattern: Stream Processing

Process data as it arrives:

```go
func streamProcessor[T any](
    input <-chan T,
    pipeline pipz.Chainable[T],
) <-chan T {
    output := make(chan T)
    
    go func() {
        defer close(output)
        ctx := context.Background()
        
        for item := range input {
            result, err := pipeline.Process(ctx, item)
            if err != nil {
                log.Printf("Processing error: %v", err)
                continue
            }
            output <- result
        }
    }()
    
    return output
}
```

### Pattern: Memory Pools

Reuse objects to reduce GC pressure:

```go
var orderPool = sync.Pool{
    New: func() interface{} {
        return &Order{
            Items:    make([]Item, 0, 10),
            Metadata: make(map[string]string),
        }
    },
}

func processWithPool(ctx context.Context, data OrderData) (*Order, error) {
    order := orderPool.Get().(*Order)
    defer func() {
        // Reset order
        order.Items = order.Items[:0]
        order.Metadata = make(map[string]string)
        orderPool.Put(order)
    }()
    
    // Use pooled order
    order.ID = data.ID
    order.Customer = data.Customer
    // ...
    
    return order, nil
}
```

## Profiling Pipelines

Use Go's built-in profiler:

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Run your pipeline
    runPipeline()
}

// Profile CPU:
// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

// Profile Memory:
// go tool pprof http://localhost:6060/debug/pprof/heap
```

## Performance Checklist

- [ ] Benchmark before optimizing
- [ ] Profile to find bottlenecks
- [ ] Minimize allocations
- [ ] Optimize Clone() methods
- [ ] Use batching where possible
- [ ] Limit concurrent operations
- [ ] Cache expensive computations
- [ ] Combine related processors
- [ ] Use memory pools for high-frequency objects
- [ ] Monitor GC pressure

## Common Bottlenecks

1. **Excessive Cloning**: Optimize Clone() or avoid Concurrent when not needed
2. **Small Batches**: Increase batch sizes for better throughput
3. **Unbounded Concurrency**: Limit parallel operations
4. **Missing Caches**: Cache expensive external calls
5. **Interface Overhead**: Combine processors to reduce calls

## Benchmark Results

Example from the payment processing pipeline:

```
BenchmarkSimplePipeline-8        1000000      1053 ns/op       0 B/op       0 allocs/op
BenchmarkWithCloning-8            300000      4127 ns/op     896 B/op      12 allocs/op
BenchmarkConcurrent-8             100000     10382 ns/op    1792 B/op      24 allocs/op
BenchmarkWithCaching-8           5000000       237 ns/op       0 B/op       0 allocs/op
```

## Next Steps

- [Testing Guide](./testing.md) - Performance testing strategies
- [Best Practices](./best-practices.md) - Production optimization
- [Error Recovery](./safety-reliability.md) - Performance impact of error handling