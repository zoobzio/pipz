# pipz API Reference

This document provides a comprehensive reference for all public APIs in the pipz library.

## Table of Contents

- [Core Types](#core-types)
  - [Processor](#processor)
  - [Contract](#contract)
  - [Chain](#chain)
  - [Chainable](#chainable)
- [Constructor Functions](#constructor-functions)
  - [NewContract](#newcontract)
  - [NewChain](#newchain)
- [Adapter Functions](#adapter-functions)
  - [Transform](#transform)
  - [Validate](#validate)
  - [Apply](#apply)
  - [Mutate](#mutate)
  - [Effect](#effect)
  - [Enrich](#enrich)
- [Methods](#methods)
  - [Contract Methods](#contract-methods)
  - [Chain Methods](#chain-methods)

## Core Types

### Processor

```go
type Processor[T any] func(T) (T, error)
```

A function that processes data of type T, returning the processed data and an error.
This is the fundamental building block of all pipelines.

**Example:**
```go
var processor Processor[User] = func(u User) (User, error) {
    if u.Email == "" {
        return u, errors.New("email required")
    }
    u.Email = strings.ToLower(u.Email)
    return u, nil
}
```

### Contract

```go
type Contract[T any] struct {
    processor Processor[T]
}
```

A Contract wraps a single Processor and provides methods for composition and execution.

**Key Features:**
- Immutable after creation
- Implements the Chainable interface
- Can be composed with other Contracts or Chains

### Chain

```go
type Chain[T any] struct {
    contracts []Chainable[T]
}
```

A Chain represents a sequence of Chainable processors executed in order.

**Key Features:**
- Executes processors sequentially
- Stops at the first error (fail-fast)
- Can be composed with other Chains or Contracts

### Chainable

```go
type Chainable[T any] interface {
    Process(T) (T, error)
    Then(Chainable[T]) *Chain[T]
}
```

Interface implemented by both Contract and Chain, enabling composition.

## Constructor Functions

### NewContract

```go
func NewContract[T any](processor Processor[T]) *Contract[T]
```

Creates a new Contract wrapping the given Processor.

**Parameters:**
- `processor`: The Processor function to wrap

**Returns:**
- A new Contract instance

**Example:**
```go
contract := pipz.NewContract(func(n int) (int, error) {
    if n < 0 {
        return 0, errors.New("negative number")
    }
    return n * 2, nil
})
```

### NewChain

```go
func NewChain[T any](chainables ...Chainable[T]) *Chain[T]
```

Creates a new Chain from the given Chainable processors.

**Parameters:**
- `chainables`: Variable number of Chainable processors

**Returns:**
- A new Chain instance

**Example:**
```go
chain := pipz.NewChain(
    validateUser,
    enrichUser,
    saveUser,
)
```

## Adapter Functions

### Transform

```go
func Transform[T any](fn func(T) T) *Contract[T]
```

Creates a Contract from a pure transformation function that cannot fail.

**Use Cases:**
- Data formatting
- Field transformations
- Calculations

**Example:**
```go
uppercase := pipz.Transform(func(s string) string {
    return strings.ToUpper(s)
})
```

### Validate

```go
func Validate[T any](fn func(T) error) *Contract[T]
```

Creates a Contract that validates data without modifying it.

**Use Cases:**
- Input validation
- Business rule checks
- Precondition verification

**Example:**
```go
validateAge := pipz.Validate(func(user User) error {
    if user.Age < 18 {
        return errors.New("must be 18 or older")
    }
    return nil
})
```

### Apply

```go
func Apply[T any](fn func(T) (T, error)) *Contract[T]
```

Creates a Contract from a function that might fail.

**Use Cases:**
- External API calls
- Database operations
- Parsing operations

**Example:**
```go
parseJSON := pipz.Apply(func(data string) (string, error) {
    var result map[string]interface{}
    err := json.Unmarshal([]byte(data), &result)
    if err != nil {
        return "", err
    }
    return result["id"].(string), nil
})
```

### Mutate

```go
func Mutate[T any](condition func(T) bool, mutation func(T) T) *Contract[T]
```

Creates a Contract that conditionally transforms data based on a predicate.

**Use Cases:**
- Conditional updates
- Feature flags
- Dynamic behavior

**Example:**
```go
applyDiscount := pipz.Mutate(
    func(order Order) bool { return order.Total > 100 },
    func(order Order) Order {
        order.Discount = order.Total * 0.1
        return order
    },
)
```

### Effect

```go
func Effect[T any](fn func(T)) *Contract[T]
```

Creates a Contract for side effects that don't modify the data.

**Use Cases:**
- Logging
- Metrics collection
- Event publishing

**Example:**
```go
logUser := pipz.Effect(func(user User) {
    log.Printf("Processing user: %s", user.Email)
})
```

### Enrich

```go
func Enrich[T any](fn func(T) (T, error)) *Contract[T]
```

Creates a Contract that attempts to enhance data but doesn't fail if unsuccessful.

**Use Cases:**
- Optional data enrichment
- Best-effort operations
- Non-critical enhancements

**Example:**
```go
enrichLocation := pipz.Enrich(func(user User) (User, error) {
    // Try to geocode address, but don't fail if service is down
    coords, err := geocodeService.Lookup(user.Address)
    if err == nil {
        user.Coordinates = coords
    }
    return user, nil
})
```

## Methods

### Contract Methods

#### Process

```go
func (c *Contract[T]) Process(data T) (T, error)
```

Executes the Contract's processor on the input data.

**Parameters:**
- `data`: The input data to process

**Returns:**
- Processed data and error (if any)

#### Then

```go
func (c *Contract[T]) Then(next Chainable[T]) *Chain[T]
```

Composes this Contract with another Chainable, creating a Chain.

**Parameters:**
- `next`: The Chainable to execute after this Contract

**Returns:**
- A new Chain containing both processors

**Example:**
```go
pipeline := validate.Then(transform).Then(save)
```

### Chain Methods

#### Process

```go
func (c *Chain[T]) Process(data T) (T, error)
```

Executes all processors in the Chain sequentially.

**Parameters:**
- `data`: The input data to process

**Returns:**
- Final processed data and error (if any)

**Behavior:**
- Executes processors in order
- Passes output of each processor as input to the next
- Stops at first error (fail-fast)

#### Then

```go
func (c *Chain[T]) Then(next Chainable[T]) *Chain[T]
```

Appends another Chainable to this Chain.

**Parameters:**
- `next`: The Chainable to add to the Chain

**Returns:**
- A new Chain with the appended processor

**Example:**
```go
extendedPipeline := pipeline.Then(notify).Then(cleanup)
```

## Error Handling

All pipz operations follow these error handling principles:

1. **Fail-Fast**: Processing stops at the first error
2. **Error Propagation**: Errors are returned unchanged
3. **Zero Values**: On error, the zero value of T is returned
4. **No Panic**: The library never panics, all errors are returned

## Performance Characteristics

- **Processor Overhead**: ~20-30ns per processor
- **Memory**: Zero allocations in hot paths
- **Composition**: No overhead for composition
- **Type Safety**: All checks at compile time

## Thread Safety

- All types are immutable after creation
- Safe for concurrent use without synchronization
- No shared state between executions