# Pipz Performance Analysis

## Executive Summary

Pipz demonstrates excellent performance characteristics with minimal overhead and efficient memory usage. The library is highly optimized for both latency and throughput across all connector types.

## Key Performance Metrics

### Basic Processors (Zero Allocation)

- **Transform**: 2.7 ns/op, 0 allocations
- **Mutate**: 2.8-3.5 ns/op, 0 allocations
- **Enrich**: 3.0 ns/op, 0 allocations

These processors operate with zero heap allocations for simple operations, making them essentially free in terms of memory overhead.

### Core Processors

- **Apply (Success)**: 46 ns/op, 0 allocations
- **Apply (Error)**: 339 ns/op, 128 B/op, 3 allocations
- **Effect (Success)**: 46 ns/op, 0 allocations
- **Effect (Error)**: 305 ns/op, 128 B/op, 3 allocations

Error cases incur allocation overhead due to error wrapping, but successful operations remain allocation-free.

### Connectors Performance

#### Sequence (Pipeline)
- **Single Processor**: 243 ns/op, 88 B/op, 3 allocations
- **5 Processors**: 563 ns/op, 88 B/op, 3 allocations
- **10 Processors**: 836 ns/op, 88 B/op, 3 allocations
- **50 Processors**: 2,807 ns/op, 88 B/op, 3 allocations

Linear scaling with constant memory overhead regardless of pipeline length.

#### Concurrent
- **Single Processor**: 2,526 ns/op, 456 B/op, 12 allocations
- **Three Processors**: 3,664 ns/op, 648 B/op, 16 allocations
- **Ten Processors**: 6,969 ns/op, 1,320 B/op, 30 allocations

Predictable overhead for goroutine coordination with linear scaling.

#### Race
- **Two Processors**: 3,433 ns/op, 856 B/op, 15 allocations
- **Five Processors**: 5,114 ns/op, 1,656 B/op, 24 allocations

Efficient implementation with minimal overhead beyond concurrent execution.

#### Switch
- **Two Routes**: 193 ns/op, 88 B/op, 3 allocations
- **Ten Routes**: 244 ns/op, 92 B/op, 4 allocations
- **No Match (Passthrough)**: 185 ns/op, 88 B/op, 3 allocations

Near-constant time routing with minimal overhead.

#### Retry/Backoff
- **Retry (First Success)**: 174 ns/op, 88 B/op, 3 allocations
- **Retry (Second Attempt)**: 704 ns/op, 248 B/op, 7 allocations
- **Backoff (With Delays)**: ~100μs for exponential backoff

Efficient retry logic with predictable overhead per attempt.

#### Fallback
- **Primary Success**: 163 ns/op, 88 B/op, 3 allocations
- **Primary Failure**: 588 ns/op, 248 B/op, 7 allocations

Fast-path optimization when primary succeeds.

#### Handle (Error Handler)
- **No Error**: 161 ns/op, 88 B/op, 3 allocations
- **With Error**: 635 ns/op, 248 B/op, 7 allocations

Minimal overhead when no error handling is needed.

#### Timeout
- **Fast Processor**: 2,265 ns/op, 688 B/op, 12 allocations
- **Near Timeout**: ~1ms (includes actual timeout wait)

Timeout overhead is primarily from context and timer management.

#### RateLimiter
- **No Rate Limiting**: 1,050 ns/op, 11 B/op, 0 allocations
- **With Rate Limiting (Wait Mode)**: 99,179 ns/op, 25 B/op, 0 allocations
- **Drop Mode (Rate Exceeded)**: 475 ns/op, 151 B/op, 3 allocations

Rate limiting overhead is primarily from token bucket algorithm when waiting for tokens. Drop mode fails fast with minimal overhead.

#### CircuitBreaker
- **Closed State (Normal)**: 76 ns/op, 0 B/op, 0 allocations
- **Open State (Blocking)**: 408 ns/op, 152 B/op, 4 allocations
- **State Transitions**: ~612μs (includes actual delays for state management)

Circuit breaker has minimal overhead in closed state, with fast failure in open state.

## Performance Characteristics

### Strengths

1. **Zero-allocation primitives**: Transform, Mutate, and Enrich have zero allocations for simple operations
2. **Predictable scaling**: Linear performance degradation with pipeline length
3. **Efficient error handling**: Only allocates when errors actually occur
4. **Fast routing**: Switch connector has near-constant time performance
5. **Memory efficient**: Fixed allocation patterns regardless of data size for most operations

### Optimization Opportunities

1. **Context propagation**: Path tracking adds ~88 bytes per operation
2. **Concurrent overhead**: Goroutine coordination adds ~2-3μs baseline
3. **Timeout management**: Context with timeout adds ~2μs overhead

### Real-World Performance

For typical use cases:
- **Simple data transformation pipeline (5 steps)**: ~560 ns total
- **Concurrent processing (3 operations)**: ~3.6 μs total
- **Error handling with fallback**: ~590 ns on error path
- **Conditional routing**: ~190 ns per decision
- **Rate limiting (within limits)**: ~1.05 μs overhead
- **Circuit breaker (healthy service)**: ~76 ns overhead
- **Resilient pipeline (rate limit + circuit break + retry)**: ~1.2 μs when healthy

### Memory Efficiency

The library demonstrates excellent memory efficiency:
- Most operations use < 100 bytes
- Allocation count is predictable and bounded
- No memory leaks or unbounded growth
- Efficient reuse of resources in loops

## Recommendations

1. **For maximum performance**: Use Transform/Mutate/Enrich for simple operations
2. **For error handling**: Apply/Effect add minimal overhead only when errors occur
3. **For parallelism**: Concurrent/Race are efficient for I/O-bound operations
4. **For routing**: Switch is highly efficient even with many routes
5. **For resilience**: Retry/Backoff/Fallback add predictable, acceptable overhead
6. **For rate limiting**: RateLimiter adds ~1μs overhead with excellent burst handling
7. **For circuit breaking**: CircuitBreaker adds only ~76ns overhead in healthy state
8. **For combined protection**: Layering resilience patterns (rate limit + circuit break + retry) adds ~1.2μs total overhead

## Conclusion

Pipz achieves its design goal of providing powerful data processing abstractions with minimal overhead. The library is suitable for high-performance scenarios where every nanosecond counts, while still providing rich functionality for complex processing pipelines.