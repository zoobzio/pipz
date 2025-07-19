# pipz API Reference

## Core Interface

### Chainable[T]

The fundamental interface that all processors and connectors implement.

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
}
```

## Adapter Functions

### Apply

Wraps a function that transforms data and may return an error.

```go
func Apply[T any](name string, fn func(context.Context, T) (T, error)) Chainable[T]
```

**Usage:**
```go
processor := pipz.Apply("transform", func(ctx context.Context, data MyType) (MyType, error) {
    data.Value = data.Value * 2
    return data, nil
})
```

### Validate

Wraps a validation function that returns an error if validation fails.

```go
func Validate[T any](name string, fn func(context.Context, T) error) Chainable[T]
```

**Usage:**
```go
validator := pipz.Validate("check_positive", func(ctx context.Context, data MyType) error {
    if data.Value <= 0 {
        return errors.New("value must be positive")
    }
    return nil
})
```

### Effect

Wraps a function that performs side effects without modifying the data.

```go
func Effect[T any](name string, fn func(context.Context, T) error) Chainable[T]
```

**Usage:**
```go
logger := pipz.Effect("log", func(ctx context.Context, data MyType) error {
    log.Printf("Processing: %+v", data)
    return nil
})
```

## Connectors

### Sequential

Executes chainables in order, stopping at the first error.

```go
func Sequential[T any](chainables ...Chainable[T]) Chainable[T]
```

**Usage:**
```go
pipeline := pipz.Sequential(
    validateInput,
    transformData,
    saveToDatabase,
)
```

### Switch

Routes to different chainables based on a condition function.

```go
func Switch[T any](
    condition func(context.Context, T) string,
    routes map[string]Chainable[T],
) Chainable[T]
```

**Usage:**
```go
router := pipz.Switch(
    func(ctx context.Context, req Request) string {
        return req.Type
    },
    map[string]pipz.Chainable[Request]{
        "api":     handleAPI,
        "webhook": handleWebhook,
        "default": handleUnknown,
    },
)
```

### Fallback

Tries the primary chainable, falling back to the secondary on error.

```go
func Fallback[T any](primary, fallback Chainable[T]) Chainable[T]
```

**Usage:**
```go
resilient := pipz.Fallback(primaryDatabase, backupDatabase)
```

### Retry

Retries a chainable up to maxAttempts times.

```go
func Retry[T any](chainable Chainable[T], maxAttempts int) Chainable[T]
```

**Usage:**
```go
reliable := pipz.Retry(unreliableService, 3)
```

### RetryWithBackoff

Retries with exponential backoff between attempts.

```go
func RetryWithBackoff[T any](
    chainable Chainable[T], 
    maxAttempts int,
    baseDelay time.Duration,
) Chainable[T]
```

**Usage:**
```go
resilient := pipz.RetryWithBackoff(
    apiCall,
    5,                      // max attempts
    100*time.Millisecond,   // base delay
)
```

### Timeout

Enforces a timeout on a chainable's execution.

```go
func Timeout[T any](chainable Chainable[T], timeout time.Duration) Chainable[T]
```

**Usage:**
```go
fast := pipz.Timeout(slowOperation, 5*time.Second)
```

## Utility Types

### ProcessorFunc[T]

Function type that implements Chainable[T].

```go
type ProcessorFunc[T any] func(context.Context, T) (T, error)

func (f ProcessorFunc[T]) Process(ctx context.Context, data T) (T, error) {
    return f(ctx, data)
}
```

**Usage:**
```go
custom := pipz.ProcessorFunc[MyType](func(ctx context.Context, data MyType) (MyType, error) {
    // Custom processing logic
    return data, nil
})
```

### Error Handling

All pipz errors include context about where they occurred:

```go
// Errors include the processor name
result, err := pipeline.Process(ctx, data)
if err != nil {
    // Error format: "processor_name: error message"
    log.Printf("Pipeline failed: %v", err)
}
```

## Context Handling

All processors receive a context.Context as their first parameter:

- Check for cancellation: `ctx.Err()`
- Access values: `ctx.Value(key)`
- Respect deadlines: `ctx.Deadline()`

**Example:**
```go
processor := pipz.Apply("cancelable", func(ctx context.Context, data MyType) (MyType, error) {
    select {
    case <-ctx.Done():
        return data, ctx.Err()
    default:
        // Process data
        return data, nil
    }
})
```

## Type Safety

pipz uses Go generics to maintain type safety:

```go
// Type is preserved through the pipeline
type Order struct {
    ID    string
    Total float64
}

pipeline := pipz.Sequential(
    pipz.Validate("validate", validateOrder),
    pipz.Apply("tax", calculateTax),
    pipz.Effect("log", logOrder),
)

// Input and output types match
order := Order{ID: "123", Total: 100}
result, err := pipeline.Process(ctx, order) // result is Order
```

## Performance Considerations

- **Minimal Overhead**: Each processor adds ~10-50ns
- **No Reflection**: Type safety through generics
- **Context Propagation**: Efficient context passing
- **Memory Efficiency**: No unnecessary allocations

## Best Practices

1. **Name your processors** for better error messages
2. **Keep processors focused** on a single responsibility
3. **Handle context cancellation** in long-running operations
4. **Use appropriate connectors** for your use case
5. **Test processors independently** before composing

## Examples

See the [examples](examples/) directory for complete working examples:
- [validation](examples/validation/) - Business rule validation
- [webhook](examples/webhook/) - Multi-provider webhook processing
- [ai](examples/ai/) - AI/LLM pipeline with caching and fallbacks
- [workflow](examples/workflow/) - Complex e-commerce order flow

## Migration from v0.4.x

If upgrading from the old Pipeline/Chain API:

```go
// Old
chain := pipz.NewChain[T]()
chain.Add(processor1)
chain.Add(processor2)

// New
pipeline := pipz.Sequential(processor1, processor2)
```