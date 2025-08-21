# Core Concepts

## The Pipeline Mental Model

Think of pipz as a conveyor belt for your data. Each processor is a station that transforms, validates, or enriches data as it passes through. Connectors determine how data flows between stations - sequentially, in parallel, or conditionally.

### Pipeline Lifecycle & Data Flow

```
┌──────────────────────────────────────────────────────────────────┐
│                     Complete Pipeline Lifecycle                  │
└──────────────────────────────────────────────────────────────────┘

┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│  Input   │─────→│ Validate │─────→│Transform │─────→│  Output  │
│   Data   │      │  Stage   │      │  Stage   │      │   Data   │
└──────────┘      └──────────┘      └──────────┘      └──────────┘
      │                 │                 │                 │
      ▼                 ▼                 ▼                 ▼
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│ Context  │      │  Error   │      │  Error   │      │  Result  │
│  State   │      │ Handling │      │ Handling │      │  State   │
└──────────┘      └──────────┘      └──────────┘      └──────────┘

Legend:
─────→  Data flow
  ▼     Context/Error propagation
  [✓]   Success state
  [✗]   Failure state
```

### Data Flow Patterns

```
Sequential:  Input → [A] → [B] → [C] → Output
                      ↓     ↓     ↓
                    (T,e) (T,e) (T,e)

Parallel:    Input → ┌─[A]─┐
                     ├─[B]─┼→ Output
                     └─[C]─┘

Conditional: Input → [?] → [A] → Output
                      ↓
                     [B] → Output

Error Flow:  Input → [A] → [B] ✗ → Error[T]
                            ↓
                          Stop
```

The power comes from composition: small, focused functions combine into complex workflows. This isn't just functional programming - it's a structured way to organize business logic that scales from simple validations to distributed systems.

## The Chainable Interface

Everything in pipz implements a single interface:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Name() string
}
```

This simplicity is deliberate. Any type implementing this interface can be:
- Used in any pipeline
- Composed with any other Chainable
- Tested in isolation
- Replaced at runtime

You can implement Chainable directly for custom needs, or use the provided wrappers for common patterns.

## Processors: Transform Your Data

Processors are immutable functions that act on data. pipz provides wrappers for common patterns:

| Processor | Purpose | Can Fail? | Modifies Data? | Example Use |
|-----------|---------|-----------|----------------|-------------|
| **Transform** | Pure transformation | No | Yes | Formatting, calculations |
| **Apply** | Fallible transformation | Yes | Yes | Parsing, validation |
| **Effect** | Side effects | Yes | No | Logging, metrics |
| **Mutate** | Conditional modification | No | Yes/No | Feature flags |
| **Enrich** | Optional enhancement | Logs errors | Yes | Adding metadata |
| **Filter** | Pass/block data | No | No | Access control |

### Example: Building a Validation Pipeline

```go
// Define processor names as constants (best practice)
const (
    ValidateEmail = "validate-email"
    NormalizeData = "normalize"
    AuditLog      = "audit"
)

// Create processors
validators := []pipz.Chainable[User]{
    pipz.Apply(ValidateEmail, func(ctx context.Context, u User) (User, error) {
        if !strings.Contains(u.Email, "@") {
            return u, errors.New("invalid email")
        }
        return u, nil
    }),
    pipz.Transform(NormalizeData, func(ctx context.Context, u User) User {
        u.Email = strings.ToLower(u.Email)
        return u
    }),
    pipz.Effect(AuditLog, func(ctx context.Context, u User) error {
        log.Printf("User validated: %s", u.Email)
        return nil
    }),
}
```

## Connectors: Control the Flow

Connectors compose processors and control execution flow:

### Sequential Processing

**Sequence** - Process data through steps in order:
```go
pipeline := pipz.NewSequence("user-flow", validators...)
```

### Parallel Processing

These require `T` to implement `Cloner[T]` for safe concurrent execution:

**Concurrent** - Run all processors simultaneously:
```go
notifications := pipz.NewConcurrent("notify",
    sendEmail,
    sendSMS,
    updateMetrics,
)
```

**Race** - Return first successful result:
```go
fetch := pipz.NewRace("fetch",
    primaryDB,
    replicaDB,
    cache,
)
```

**Contest** - Return first result meeting criteria:
```go
quality := pipz.NewContest("quality",
    func(ctx context.Context, result Result) bool {
        return result.Confidence > 0.9
    },
    aiModel1, aiModel2, aiModel3,
)
```

### Conditional Processing

**Switch** - Route based on conditions:
```go
router := pipz.NewSwitch("router",
    func(ctx context.Context, req Request) string {
        if req.Premium {
            return "premium"
        }
        return "standard"
    },
).
AddRoute("premium", premiumPipeline).
AddRoute("standard", standardPipeline)
```

### Error Handling

**Fallback** - Provide alternative on failure:
```go
safe := pipz.NewFallback("safe", riskyOperation, safeDefault)
```

**Retry** - Retry transient failures:
```go
reliable := pipz.NewRetry("api", apiCall, 3)
```

### Resilience

**CircuitBreaker** - Prevent cascading failures:
```go
protected := pipz.NewCircuitBreaker("service", processor,
    pipz.WithCircuitBreakerThreshold(5),
)
```

**RateLimiter** - Control throughput:
```go
throttled := pipz.NewRateLimiter("api", processor,
    pipz.WithRateLimiterRate(100),
    pipz.WithRateLimiterPeriod(time.Second),
)
```

**Timeout** - Bound execution time:
```go
bounded := pipz.NewTimeout("slow", processor, 5*time.Second)
```

## Type Safety Through Generics

pipz leverages Go generics for compile-time type safety:

```go
// Type is locked at pipeline creation
pipeline := pipz.NewSequence[User]("user-pipeline")

// This won't compile - type mismatch
pipeline.Register(processOrder) // Error: expects Chainable[User], got Chainable[Order]

// Transform between types explicitly
converter := pipz.Apply("convert", func(ctx context.Context, u User) (Order, error) {
    return u.CreateOrder()
})
```

### The Cloner Constraint

For concurrent processing, your type must implement `Cloner[T]`:

```go
type Data struct {
    Values []int
}

func (d Data) Clone() Data {
    newValues := make([]int, len(d.Values))
    copy(newValues, d.Values)
    return Data{Values: newValues}
}
```

## Error Philosophy

Errors in pipz are first-class citizens with rich context:

```go
type Error[T any] struct {
    Stage Name    // Where it failed
    Cause error   // Why it failed  
    State T       // Data at failure
}
```

This design enables:
- **Precise debugging**: Know exactly where and why failures occur
- **Error recovery**: Access data state for compensation
- **Error pipelines**: Process errors through their own pipelines

### Error Pipeline Pattern

```go
// Errors are just data - process them like anything else
errorPipeline := pipz.NewSequence[*pipz.Error[Order]]("error-handler",
    pipz.Effect("log", logError),
    pipz.Apply("classify", classifyError),
    pipz.Switch("recovery", selectRecoveryStrategy),
)

// Use with main pipeline
mainPipeline := pipz.NewFallback("main",
    orderProcessing,
    pipz.Handle("recover", errorPipeline),
)
```

## Best Practices

1. **Name processors with constants** - Use consistent keys throughout your system
2. **Keep processors focused** - Each should do one thing well
3. **Compose, don't configure** - Build complex behavior from simple parts
4. **Test in isolation** - Each processor should be independently testable
5. **Handle context** - Always respect cancellation and timeouts
6. **Clone properly** - Deep copy slices and maps in Clone() methods

## Next Steps

- [Architecture](./architecture.md) - System design and internals
- [Quickstart Tutorial](../tutorials/quickstart.md) - Build your first pipeline
- [Connector Selection](../guides/connector-selection.md) - Choose the right connector
- [API Reference](../reference/) - Complete API documentation