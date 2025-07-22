# Processors API Reference

Complete reference for all processor-related functions and types.

## Core Interface

### Chainable[T]

The fundamental interface that all processors implement.

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
}
```

**Type Parameters:**
- `T` - The type of data being processed

**Methods:**
- `Process(ctx context.Context, data T) (T, error)` - Processes the input data and returns the result or an error

## Processor Adapters

### Transform

Creates a processor from a pure transformation function that cannot fail.

```go
func Transform[T any](name string, fn func(context.Context, T) T) Chainable[T]
```

**Parameters:**
- `name` - Identifier for the processor (used in error messages)
- `fn` - Transformation function

**Example:**
```go
double := pipz.Transform("double", func(ctx context.Context, n int) int {
    return n * 2
})
```

### Apply

Creates a processor from a function that can return an error.

```go
func Apply[T any](name string, fn func(context.Context, T) (T, error)) Chainable[T]
```

**Parameters:**
- `name` - Identifier for the processor
- `fn` - Processing function that may fail

**Example:**
```go
validate := pipz.Apply("validate", func(ctx context.Context, user User) (User, error) {
    if user.Email == "" {
        return user, errors.New("email required")
    }
    return user, nil
})
```

### Effect

Creates a processor that performs side effects without modifying the input.

```go
func Effect[T any](name string, fn func(context.Context, T) error) Chainable[T]
```

**Parameters:**
- `name` - Identifier for the processor
- `fn` - Side effect function

**Returns:** Original input unchanged (unless error occurs)

**Example:**
```go
logger := pipz.Effect("log", func(ctx context.Context, order Order) error {
    log.Printf("Processing order: %s", order.ID)
    return nil
})
```

### Mutate

Creates a processor that conditionally modifies data.

```go
func Mutate[T any](
    name string,
    condition func(context.Context, T) bool,
    mutation func(context.Context, T) T,
) Chainable[T]
```

**Parameters:**
- `name` - Identifier for the processor
- `condition` - Function that determines if mutation should occur
- `mutation` - Function that performs the mutation

**Example:**
```go
autoVerify := pipz.Mutate("auto_verify",
    func(ctx context.Context, user User) bool {
        return strings.HasSuffix(user.Email, "@trusted.com")
    },
    func(ctx context.Context, user User) User {
        user.Verified = true
        return user
    },
)
```

### Enrich

Creates a processor that attempts to enhance data but doesn't fail on error.

```go
func Enrich[T any](name string, fn func(context.Context, T) (T, error)) Chainable[T]
```

**Parameters:**
- `name` - Identifier for the processor
- `fn` - Enrichment function

**Behavior:** Returns original input if enrichment fails

**Example:**
```go
enrichLocation := pipz.Enrich("geocode", func(ctx context.Context, user User) (User, error) {
    coords, err := geocodeAPI.Lookup(ctx, user.Address)
    if err != nil {
        return user, err // Will be ignored
    }
    user.Coordinates = coords
    return user, nil
})
```

## Utility Types

### ProcessorFunc[T]

Function adapter that implements Chainable[T].

```go
type ProcessorFunc[T any] func(context.Context, T) (T, error)

func (f ProcessorFunc[T]) Process(ctx context.Context, data T) (T, error) {
    return f(ctx, data)
}
```

**Use Case:** Creating anonymous processors

**Example:**
```go
custom := pipz.ProcessorFunc[string](func(ctx context.Context, s string) (string, error) {
    return strings.ToUpper(s), nil
})
```

## Type Constraints

### Cloner[T]

Required interface for types used with concurrent processors.

```go
type Cloner[T any] interface {
    Clone() T
}
```

**Required By:**
- `Concurrent`
- `Race`

**Example Implementation:**
```go
type Order struct {
    ID    string
    Items []Item
}

func (o Order) Clone() Order {
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    return Order{
        ID:    o.ID,
        Items: items,
    }
}
```

## Error Types

### PipelineError[T]

Error type that includes context about pipeline failures.

```go
type PipelineError[T any] struct {
    Stage    string    // Processor name where error occurred
    Cause    error     // Original error
    Input    T         // Input when error occurred
    Pipeline string    // Pipeline identifier (if set)
    Timeout  bool      // Was this a timeout?
    Canceled bool      // Was this cancelled?
}
```

**Methods:**
- `Error() string` - Implements error interface
- `Unwrap() error` - Returns the cause for errors.Is/As
- `IsTimeout() bool` - Checks if error was due to timeout
- `IsCanceled() bool` - Checks if error was due to cancellation

**Example:**
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.PipelineError[Order]
    if errors.As(err, &pipeErr) {
        log.Printf("Failed at stage: %s", pipeErr.Stage)
        log.Printf("Input was: %+v", pipeErr.Input)
    }
}
```

## Function Signatures

### Condition[T, K]

Function type for routing decisions.

```go
type Condition[T any, K comparable] func(context.Context, T) K
```

**Type Parameters:**
- `T` - Input data type
- `K` - Route key type (must be comparable)

**Used By:** `Switch` connector

**Example:**
```go
func routeByPriority(ctx context.Context, order Order) Priority {
    if order.Amount > 1000 {
        return PriorityHigh
    }
    return PriorityNormal
}
```

## Next Steps

- [Connectors API](./connectors.md) - Connector function reference
- [Pipeline API](./pipeline.md) - Pipeline type reference
- [Examples](../examples/payment-processing.md) - See the API in use