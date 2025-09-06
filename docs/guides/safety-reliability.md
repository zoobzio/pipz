# Safety and Reliability Guide

pipz is designed with safety and reliability as core principles. This guide covers the built-in protections that make your pipelines robust in production environments.

## Automatic Panic Recovery

**Every processor and connector in pipz includes comprehensive panic recovery.** This is not optional - it's always enabled to ensure your applications never crash due to unexpected panics in pipeline processors.

### What Gets Protected

**Complete Coverage**:
- All processors: `Apply`, `Transform`, `Effect`, `Mutate`, `Enrich`
- All connectors: `Sequence`, `Concurrent`, `WorkerPool`, `Scaffold`, `Switch`, `Fallback`, `Race`, `Contest`, `Retry`, `Backoff`, `Timeout`, `Handle`, `Filter`, `RateLimiter`, `CircuitBreaker`
- All user-defined functions passed to processors
- All condition functions, transformation functions, and side-effect functions

### How It Works

```go
// ANY panic in this function will be automatically recovered
riskyProcessor := pipz.Apply("risky", func(ctx context.Context, data string) (string, error) {
    // All of these panics are automatically handled:
    if data == "bounds" {
        arr := []int{1, 2, 3}
        return fmt.Sprintf("%d", arr[10]), nil // Index out of bounds - RECOVERED
    }
    if data == "nil" {
        var ptr *string
        return *ptr, nil // Nil pointer dereference - RECOVERED
    }
    if data == "assert" {
        var val interface{} = "not a number"
        num := val.(int) // Type assertion failure - RECOVERED
        return fmt.Sprintf("%d", num), nil
    }
    if data == "explicit" {
        panic("something went wrong!") // Explicit panic - RECOVERED
    }
    return data, nil
})

// Use it safely - panics become regular errors
result, err := riskyProcessor.Process(ctx, "nil")
if err != nil {
    fmt.Printf("Caught panic as error: %v\n", err)
    // Output: risky failed after 123μs: panic in processor "risky": panic occurred: runtime error: invalid memory address or nil pointer dereference
}
```

### Security Sanitization

Panic messages often contain sensitive information that shouldn't be exposed in logs or error responses. pipz automatically sanitizes panic messages to prevent information leakage:

**Memory Address Sanitization**:
```go
// Original panic: "invalid memory access at 0x1234567890abcdef"
// Sanitized: "panic occurred: invalid memory access at 0x***"
```

**File Path Sanitization**:
```go
// Original panic: "error in /sensitive/internal/path/file.go:123"
// Sanitized: "panic occurred (file path sanitized)"
```

**Stack Trace Sanitization**:
```go
// Original panic: "error\ngoroutine 1 [running]:\nruntime.main()\n..."
// Sanitized: "panic occurred (stack trace sanitized)"
```

**Length Limiting**:
```go
// Very long panic messages (>200 chars) are truncated:
// "panic occurred (message truncated for security)"
```

**Sanitization Patterns**:

The sanitization process applies these specific patterns:
- **Memory addresses**: Replaces hex sequences after `0x` with `0x***`
- **File paths**: Detects `/` or `\` characters and replaces entire message
- **Stack traces**: Detects `goroutine` or `runtime.` keywords and replaces entire message
- **Length limit**: Messages longer than 200 characters are truncated
- **Nil panics**: Converts nil panic values to `"unknown panic (nil value)"`

### Performance Impact

Panic recovery is implemented with minimal performance overhead:

- **Cost**: ~20 nanoseconds per operation (measured via benchmarks)
- **Implementation**: Uses Go's built-in `defer` and `recover()` mechanisms
- **Zero allocation** in the normal (no-panic) path
- **Always enabled**: No build tags or configuration options to minimize complexity

### Real-World Example

```go
// Processing user data that might come from untrusted sources
processUserInput := pipz.Apply("parse-input", func(ctx context.Context, input string) (UserData, error) {
    // Third-party JSON library might panic on malformed input
    var userData UserData
    if err := someJSONLibrary.Unmarshal([]byte(input), &userData); err != nil {
        return userData, err
    }
    
    // Array access that might panic if data is unexpected
    if len(userData.Scores) > 0 {
        userData.AverageScore = userData.Scores[0] // Could panic if Scores is nil
    }
    
    return userData, nil
})

// Even with malicious or malformed input, your application won't crash
pipeline := pipz.NewSequence("user-pipeline",
    processUserInput,
    validateUserData,
    enrichWithDefaults,
)

// Malicious input that would normally crash your application
maliciousInput := `{"scores": null, "data": "` + strings.Repeat("x", 100000) + `"}`
result, err := pipeline.Process(ctx, maliciousInput)
if err != nil {
    // The application continues running, panic is converted to error
    log.Printf("Input processing failed safely: %v", err)
    return handleBadInput(err)
}
```

## Error Handling Integration

Panic recovery integrates seamlessly with pipz's error handling system:

### Error Path and Context

```go
result, err := panickyPipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[DataType]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Pipeline: %s\n", strings.Join(pipeErr.Path, " -> "))
        fmt.Printf("Duration: %v\n", pipeErr.Duration)
        fmt.Printf("Input data: %+v\n", pipeErr.InputData)
        
        // Check if the underlying error is a panic
        if strings.Contains(pipeErr.Error(), "panic in processor") {
            fmt.Println("This was a recovered panic")
        }
    }
}
```

### Error Recovery Patterns

Use Handle to process panic errors specifically:

```go
pipeline := pipz.NewSequence("resilient",
    riskyProcessor,
    pipz.NewHandle("panic-handler", nextProcessor,
        pipz.Effect("log-panic", func(ctx context.Context, err *pipz.Error[DataType]) error {
            if strings.Contains(err.Error(), "panic in processor") {
                log.Printf("ALERT: Panic recovered in %s: %v", 
                    strings.Join(err.Path, "->"), err.Err)
                // Send to monitoring system, etc.
            }
            return nil
        }),
    ),
)
```

## Reliability Through Layered Protection

Combine panic recovery with other reliability patterns for maximum resilience:

```go
// Multi-layered protection
resilientPipeline := pipz.NewSequence("fortress",
    pipz.Apply("validate", validateInput), // Input validation
    pipz.NewTimeout("timeout", // Timeout protection
        pipz.NewRetry("retry", // Retry on transient failures
            pipz.NewCircuitBreaker("circuit", // Circuit breaker for cascading failures
                pipz.NewFallback("fallback", // Fallback for persistent failures
                    riskyMainProcessor,    // Main logic (protected by panic recovery)
                    safeDefaultProcessor,  // Safe fallback (also protected by panic recovery)
                ),
            ),
        ),
        30*time.Second,
    ),
)
```

In this architecture:
1. **Input validation** prevents known bad data
2. **Timeout** prevents hanging operations
3. **Retry** handles transient failures
4. **Circuit breaker** prevents cascade failures
5. **Fallback** provides alternate processing paths
6. **Panic recovery** (automatic) catches unexpected failures
7. **Rich error context** helps with debugging and monitoring

## Monitoring and Observability

Track panic occurrences for operational insights:

```go
var panicCounter int64

monitoredPipeline := pipz.NewSequence("monitored",
    riskyProcessor,
    pipz.NewHandle("monitor", continueProcessor,
        pipz.Effect("track-panics", func(ctx context.Context, err *pipz.Error[Data]) error {
            if strings.Contains(err.Error(), "panic in processor") {
                atomic.AddInt64(&panicCounter, 1)
                
                // Extract processor name from panic error
                parts := strings.Split(err.Error(), "panic in processor ")
                if len(parts) > 1 {
                    processorName := strings.Split(parts[1], ":")[0]
                    metrics.IncrementCounter("pipz.panics", map[string]string{
                        "processor": processorName,
                        "pipeline":  strings.Join(err.Path[:len(err.Path)-1], "->"),
                    })
                }
            }
            return nil
        }),
    ),
)
```

## Best Practices for Safety

### 1. Trust the Safety Net, But Don't Abuse It

```go
// Good - Panic recovery handles unexpected failures
processor := pipz.Apply("parse", func(ctx context.Context, data string) (Result, error) {
    result, err := someLibrary.Parse(data)
    return result, err // Library might panic, that's handled
})

// Bad - Don't use panic recovery as flow control
badProcessor := pipz.Apply("bad", func(ctx context.Context, data string) (Result, error) {
    if data == "invalid" {
        panic("invalid data") // Use proper error returns instead!
    }
    return Result{}, nil
})
```

### 2. Combine with Proper Error Handling

```go
// Good - Handle expected errors properly, let panic recovery handle unexpected ones
goodProcessor := pipz.Apply("good", func(ctx context.Context, data string) (Result, error) {
    if data == "" {
        return Result{}, errors.New("empty input") // Expected error
    }
    
    // Unexpected panics from third-party code are automatically handled
    return complexThirdPartyOperation(data), nil
})
```

### 3. Log and Monitor Panics

```go
// Set up monitoring to track panic frequency
panicMonitor := pipz.Effect("panic-monitor", func(ctx context.Context, err *pipz.Error[Data]) error {
    if strings.Contains(err.Error(), "panic in processor") {
        log.WithFields(log.Fields{
            "processor": extractProcessorName(err),
            "path":      strings.Join(err.Path, "->"),
            "duration":  err.Duration,
            "input":     fmt.Sprintf("%+v", err.InputData),
        }).Warn("Panic recovered in pipeline")
    }
    return nil
})
```

### 4. Test Panic Scenarios

```go
func TestPanicRecovery(t *testing.T) {
    panicProcessor := pipz.Transform("test-panic", func(ctx context.Context, data string) string {
        if data == "panic" {
            panic("test panic")
        }
        return data
    })
    
    result, err := panicProcessor.Process(context.Background(), "panic")
    
    // Verify panic was recovered as error
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "panic in processor")
    assert.Equal(t, "", result) // Result should be zero value
    
    // Verify normal operation still works
    result, err = panicProcessor.Process(context.Background(), "normal")
    assert.NoError(t, err)
    assert.Equal(t, "normal", result)
}
```

## Production Deployment Considerations

### Security
- **Information Leakage Prevention**: Panic sanitization prevents accidental exposure of internal details
- **Safe Defaults**: Failed operations return zero values, preventing undefined behavior
- **Attack Surface Reduction**: Malformed input cannot crash your application through panics

### Reliability
- **Graceful Degradation**: Panics become errors in the normal error handling flow
- **Service Continuity**: One panicking operation doesn't crash the entire service
- **Error Context**: Full path and input data for debugging (timing not tracked for panics)

### Performance
- **Minimal Overhead**: ~20ns per operation is typically negligible
- **No Allocations**: Panic recovery path only allocates when panics actually occur
- **Predictable Behavior**: Same error handling patterns whether errors are panics or regular errors

## Troubleshooting Panic-Related Issues

### High Panic Frequency
If you're seeing many panics in your logs:

1. **Identify the source**: Use error path information to pinpoint problematic processors
2. **Review input validation**: Ensure data is properly validated before processing
3. **Check third-party libraries**: Some libraries may panic on edge cases
4. **Consider upstream changes**: Has input data format changed recently?

### Performance Impact
If panic recovery is impacting performance:

1. **Benchmark without panics**: ~20ns overhead should be negligible in most cases
2. **Check panic frequency**: High panic rates indicate underlying issues
3. **Profile allocation**: Ensure panics aren't causing excessive allocations
4. **Consider alternatives**: If panics are frequent, address root causes rather than relying on recovery

### Debugging Sanitized Messages
If you need more details from sanitized panic messages for debugging:

1. **Use development logging**: Log full errors in development environments
2. **Add debug processors**: Wrap risky operations with additional logging
3. **Structured error handling**: Return structured errors instead of relying on panics
4. **Unit test edge cases**: Identify and test specific panic scenarios

The goal is to combine automatic safety with proper engineering practices for maximum reliability.