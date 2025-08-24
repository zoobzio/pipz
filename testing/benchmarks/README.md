# Benchmark Suite for pipz

This directory contains comprehensive performance benchmarks for the pipz package. The benchmarks are organized to measure different aspects of pipeline performance and provide comparisons with traditional approaches.

## Benchmark Categories

### Core Performance (`core_performance_test.go`)
Benchmarks fundamental pipz components in isolation:
- **Processor Types**: Transform, Apply, Effect, Enrich, Mutate
- **Individual Connectors**: Sequence, Concurrent, Race, Contest, Fallback
- **Stateful Components**: CircuitBreaker, RateLimiter, Timeout
- **Memory Allocation**: Zero-allocation paths vs error paths

### Composition Performance (`composition_performance_test.go`)
Benchmarks realistic pipeline compositions:
- **Pipeline Length**: Short (2-3 steps) vs Long (10+ steps)
- **Branching Patterns**: Sequential vs Parallel processing
- **Complex Workflows**: Multi-stage with fallbacks and retries
- **Dynamic Pipelines**: Runtime modification performance

### Comparison Benchmarks (`comparison_test.go`)
Compares pipz with traditional Go patterns:
- **Error Handling**: pipz vs traditional if-err chains
- **Retry Logic**: pipz vs manual retry implementations
- **Circuit Breaking**: pipz vs popular libraries
- **Pipeline Patterns**: pipz vs hand-coded compositions

## Running Benchmarks

### All Benchmarks
```bash
# Run all benchmarks
go test -v -bench=. ./testing/benchmarks/...

# Run with memory allocation tracking
go test -v -bench=. -benchmem ./testing/benchmarks/...

# Run multiple times for statistical significance
go test -v -bench=. -count=5 ./testing/benchmarks/...
```

### Specific Categories
```bash
# Core component benchmarks
go test -v -bench=BenchmarkCore ./testing/benchmarks/...

# Composition benchmarks
go test -v -bench=BenchmarkComposition ./testing/benchmarks/...

# Comparison benchmarks
go test -v -bench=BenchmarkComparison ./testing/benchmarks/...
```

### Performance Profiling
```bash
# CPU profiling
go test -bench=BenchmarkSpecific -cpuprofile=cpu.prof ./testing/benchmarks/...
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkSpecific -memprofile=mem.prof ./testing/benchmarks/...
go tool pprof mem.prof

# Block profiling for concurrency
go test -bench=BenchmarkConcurrent -blockprofile=block.prof ./testing/benchmarks/...
go tool pprof block.prof
```

## Benchmark Metrics

### Performance Targets
Based on current benchmarks, pipz aims for:
- **Transform**: < 5ns/op, 0 allocs/op
- **Apply (success)**: < 50ns/op, 0 allocs/op  
- **Apply (error)**: < 500ns/op, < 5 allocs/op
- **Sequence (short)**: < 100ns/op, 0 allocs/op
- **Complex workflows**: < 1µs/op for typical use cases

### Overhead Measurements
- **Pipeline overhead**: ~2-5ns per component
- **Error wrapping**: ~300-400ns with 3 allocations
- **Context propagation**: Essentially free (< 1ns)
- **Concurrent processing**: Scales linearly with goroutine count

## Benchmark Data Analysis

### Interpreting Results
```
BenchmarkTransform-8    500000000    2.7 ns/op    0 B/op    0 allocs/op
```
- **Name**: BenchmarkTransform-8 (8 CPU cores used)
- **Iterations**: 500,000,000 iterations run
- **Time per op**: 2.7 nanoseconds per operation
- **Memory per op**: 0 bytes allocated per operation  
- **Allocations per op**: 0 allocations per operation

### Performance Regression Detection
Use `benchcmp` to compare results over time:
```bash
# Save baseline
go test -bench=. ./testing/benchmarks/... > baseline.txt

# After changes
go test -bench=. ./testing/benchmarks/... > current.txt

# Compare
benchcmp baseline.txt current.txt
```

### Statistical Significance
Run multiple iterations for stable results:
```bash
# Run 10 times and analyze variance
go test -bench=BenchmarkSpecific -count=10 ./testing/benchmarks/... | tee results.txt
```

## Benchmark Environment

### Requirements
- Go 1.21+ for generics performance optimizations
- Stable system load (avoid running during builds/deployments)
- Consistent CPU frequency (disable CPU scaling if needed)
- Sufficient RAM to avoid swapping

### System Configuration
For reproducible benchmarks:
```bash
# Set CPU governor to performance mode
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# Disable CPU turbo boost
echo 1 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo

# Set process priority
nice -n -10 go test -bench=...
```

## Custom Benchmarks

### Adding New Benchmarks
Follow these patterns when adding benchmarks:

1. **Benchmark naming**: `BenchmarkCategory_Specific`
2. **Setup/teardown**: Use `b.ResetTimer()` after setup
3. **Loop structure**: Standard `for i := 0; i < b.N; i++` loop
4. **Error handling**: Use `b.Fatal(err)` for setup errors, ignore expected errors in loops
5. **Memory measurement**: Include `-benchmem` relevant benchmarks

### Example Benchmark Structure
```go
func BenchmarkNewFeature_SpecificCase(b *testing.B) {
    // Setup (not measured)
    processor := createTestProcessor()
    data := createTestData()
    ctx := context.Background()
    
    b.ResetTimer() // Start timing here
    b.ReportAllocs() // Include allocation metrics
    
    for i := 0; i < b.N; i++ {
        result, err := processor.Process(ctx, data)
        if err != nil {
            b.Fatal(err) // Only for unexpected errors
        }
        // Use result to prevent optimization
        _ = result
    }
}
```

## Performance Optimization Guidelines

### Hot Path Optimization
1. **Avoid allocations in common paths**
2. **Minimize interface{} usage**
3. **Prefer value types over pointers where safe**
4. **Use sync.Pool for expensive temporary objects**
5. **Cache frequently computed values**

### Concurrent Performance
1. **Minimize lock contention**
2. **Use atomic operations for counters**
3. **Prefer lock-free data structures**
4. **Avoid false sharing on cache lines**

### Memory Management
1. **Reuse slices and maps where possible**
2. **Use make() with capacity for known sizes**
3. **Implement proper Clone() methods for concurrent access**
4. **Avoid string concatenation in loops**

## Continuous Integration

### Automated Benchmarking
Include benchmark runs in CI:
```yaml
# Example GitHub Actions step
- name: Run benchmarks
  run: |
    go test -bench=. -benchmem ./testing/benchmarks/... > benchmarks.txt
    cat benchmarks.txt
```

### Performance Alerts
Set up alerts for performance regressions:
- **CPU time increase**: > 10% slower
- **Memory usage increase**: > 20% more allocations
- **Allocation count increase**: Any increase in zero-allocation paths

### Historical Tracking
Track performance metrics over time:
- Store benchmark results in time-series database
- Graph performance trends
- Correlate with code changes

---

The benchmark suite ensures pipz maintains excellent performance characteristics while providing comprehensive measurement tools for optimization work.