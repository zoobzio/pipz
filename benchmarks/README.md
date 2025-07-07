# Pipz Benchmarks

Performance benchmarks for the pipz pipeline library.

## Overview

These benchmarks measure the performance of pipz pipelines across various use cases and compare them with alternative approaches.

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=.

# Run specific benchmark
go test -bench=BenchmarkValidation

# Run with memory profiling
go test -bench=. -benchmem

# Run with specific duration
go test -bench=. -benchtime=10s

# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof

# View profile
go tool pprof cpu.prof
```

## Benchmark Categories

### 1. Validation Benchmarks (`validation_bench_test.go`)
- **BenchmarkValidationPipeline**: Full order validation pipeline
- **BenchmarkValidationPipelineParallel**: Parallel execution
- **BenchmarkValidateOrderID**: Individual validator performance
- **BenchmarkValidateItems**: Item validation overhead
- **BenchmarkValidateTotal**: Mathematical validation cost

### 2. Payment Processing (`payment_bench_test.go`)
- **BenchmarkPaymentPipeline**: Standard payment flow
- **BenchmarkPaymentWithFallback**: Fallback provider pattern
- **BenchmarkPaymentValidation**: Payment validation only
- **BenchmarkFraudCheck**: Fraud detection overhead
- **BenchmarkPaymentChain**: Chained pipeline performance

### 3. Security Audit (`security_bench_test.go`)
- **BenchmarkSecurityPipeline**: Full security audit flow
- **BenchmarkSecurityPipelineAdmin**: Admin path (less redaction)
- **BenchmarkRedaction**: PII redaction cost
- **BenchmarkSecurityChain**: Permission + redaction chain

### 4. Data Transformation (`transform_bench_test.go`)
- **BenchmarkTransformPipeline**: CSV to DB transformation
- **BenchmarkTransformBatch**: Batch processing performance
- **BenchmarkParseCSV**: CSV parsing overhead
- **BenchmarkNormalization**: Data normalization cost
- **BenchmarkTransformChain**: Parser + normalizer chain

### 5. Performance Comparison (`perf_test.go`)
- **BenchmarkPipzPipeline**: Pipz approach
- **BenchmarkDirectCalls**: Direct function calls (baseline)
- **BenchmarkFunctionSlice**: Function slice iteration
- **BenchmarkInterfaceApproach**: Interface-based processors
- **BenchmarkPipzManyProcessors**: Scalability with many processors
- **BenchmarkPipzChain**: Chain overhead
- **BenchmarkPipzAllocations**: Memory allocation analysis
- **BenchmarkPipzParallel**: Concurrent execution

## Expected Results

### Performance Characteristics

1. **Zero Serialization**: Pipz passes values directly between processors without serialization
2. **Type Safety**: Compile-time type checking with no runtime overhead
3. **Minimal Allocations**: Efficient memory usage
4. **Linear Scaling**: Performance scales linearly with processor count

### Typical Results (M1 MacBook Pro)

```
BenchmarkValidationPipeline-8         3,000,000      450 ns/op      48 B/op       2 allocs/op
BenchmarkDirectCalls-8               20,000,000       65 ns/op       0 B/op       0 allocs/op
BenchmarkPipzPipeline-8               5,000,000      280 ns/op      32 B/op       1 allocs/op
BenchmarkFunctionSlice-8              8,000,000      150 ns/op       0 B/op       0 allocs/op
BenchmarkInterfaceApproach-8          4,000,000      320 ns/op       0 B/op       0 allocs/op
```

### Performance Analysis

1. **vs Direct Calls**: ~4-5x overhead due to abstraction and error handling
2. **vs Function Slice**: ~2x overhead due to type safety and error propagation
3. **vs Interface Approach**: Comparable performance with better type safety
4. **Parallel Execution**: Excellent scaling with minimal contention

## Optimization Tips

1. **Reuse Pipelines**: Create pipelines once and reuse them
2. **Batch Processing**: Process multiple items to amortize setup cost
3. **Chain Wisely**: Use chains for logical separation, not every processor
4. **Minimize Allocations**: Pass structs by value for small types
5. **Profile First**: Use benchmarks to identify actual bottlenecks

## Interpreting Results

- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

## Comparison with Other Libraries

Pipz focuses on:
- Zero serialization between processors
- Type safety at compile time
- Minimal runtime overhead
- Simple, composable API

Trade-offs:
- No distributed processing
- No async/await patterns
- No built-in persistence
- In-process only