# pipz Concepts and Architecture

This document explains the core concepts, design decisions, and architectural patterns in pipz.

## Table of Contents

- [The Pipeline Pattern](#the-pipeline-pattern)
- [Type System Design](#type-system-design)
- [Error Handling Philosophy](#error-handling-philosophy)
- [Composition Mechanics](#composition-mechanics)
- [Memory and Performance](#memory-and-performance)
- [Design Patterns](#design-patterns)
- [Testing Strategies](#testing-strategies)

## The Pipeline Pattern

### What is a Pipeline?

A pipeline in pipz is a sequence of data transformations where:
- Each step receives input from the previous step
- Each step can transform the data or produce an error
- The pipeline stops at the first error (fail-fast)

### Why Pipelines?

Traditional Go code often looks like this:

```go
func ProcessUser(u User) (User, error) {
    // Validate
    if err := validateUser(u); err != nil {
        return User{}, err
    }
    
    // Transform
    u = normalizeUser(u)
    
    // Enrich
    enriched, err := enrichUser(u)
    if err != nil {
        return User{}, err
    }
    
    // Save
    saved, err := saveUser(enriched)
    if err != nil {
        return User{}, err
    }
    
    return saved, nil
}
```

With pipz, the same logic becomes:

```go
var processUser = pipz.NewChain(
    validateUser,
    normalizeUser,
    enrichUser,
    saveUser,
)
```

### Benefits of the Pipeline Pattern

1. **Separation of Concerns**: Each step has a single responsibility
2. **Testability**: Test each step in isolation
3. **Reusability**: Compose pipelines from existing steps
4. **Readability**: Business logic reads like a series of steps
5. **Error Handling**: Centralized error handling reduces boilerplate

## Type System Design

### Generic Types

pipz uses Go generics to ensure type safety:

```go
type Processor[T any] func(T) (T, error)
```

This means:
- A pipeline for `User` types only accepts and returns `User`
- Type mismatches are caught at compile time
- No runtime type assertions needed

### The Chainable Interface

```go
type Chainable[T any] interface {
    Process(T) (T, error)
    Then(Chainable[T]) *Chain[T]
}
```

Both `Contract` and `Chain` implement this interface, enabling:
- Uniform composition regardless of type
- Recursive composition (chains of chains)
- Clean API surface

### Why Immutability?

All pipz types are immutable after creation:
- Thread-safe without locks
- Predictable behavior
- Safe to share between goroutines
- Enables functional programming patterns

## Error Handling Philosophy

### Fail-Fast Principle

pipz stops processing at the first error:

```go
pipeline := transform1.Then(transform2).Then(transform3)
// If transform1 returns error, transform2 and transform3 never execute
```

### Why Fail-Fast?

1. **Predictability**: Easier to reason about error states
2. **Performance**: No wasted computation after errors
3. **Simplicity**: No complex error accumulation logic
4. **Go Idioms**: Aligns with Go's error handling patterns

### Error Context

Errors are returned unchanged, preserving context:

```go
validate := pipz.Validate(func(u User) error {
    if u.Age < 0 {
        return fmt.Errorf("invalid age %d for user %s", u.Age, u.ID)
    }
    return nil
})
```

## Composition Mechanics

### How Then() Works

The `Then` method creates a new Chain containing all processors:

```go
// These are equivalent:
pipeline1 := a.Then(b).Then(c)
pipeline2 := pipz.NewChain(a, b, c)
```

### Composition Patterns

#### Sequential Composition
```go
process := validate.Then(transform).Then(save)
```

#### Conditional Composition
```go
var pipeline Chainable[Order]
if config.ValidateEnabled {
    pipeline = validate.Then(process)
} else {
    pipeline = process
}
```

#### Dynamic Composition
```go
func BuildPipeline(steps ...Chainable[Data]) *Chain[Data] {
    return pipz.NewChain(steps...)
}
```

## Memory and Performance

### Zero Allocation Design

pipz minimizes allocations:
- Processors are function pointers (no allocation)
- Contracts store a single function pointer
- Chains store a slice of interfaces (single allocation)

### Performance Characteristics

```
BenchmarkContract-8          48.32 ns/op       0 B/op       0 allocs/op
BenchmarkChain_3-8           88.38 ns/op       0 B/op       0 allocs/op
BenchmarkChain_10-8         270.4 ns/op        0 B/op       0 allocs/op
```

### Why So Fast?

1. **No Reflection**: All type checking at compile time
2. **No Locks**: Immutable design eliminates synchronization
3. **Direct Calls**: Function pointers enable inlining
4. **Minimal Overhead**: ~20-30ns per processor

## Design Patterns

### Adapter Pattern

The adapter functions wrap different function signatures into the uniform Processor interface:

```go
// Transform adapter: func(T) T → Processor[T]
func Transform[T any](fn func(T) T) *Contract[T] {
    return NewContract(func(data T) (T, error) {
        return fn(data), nil
    })
}
```

### Builder Pattern

Chains use a fluent builder pattern:

```go
pipeline := base.
    Then(step1).
    Then(step2).
    Then(step3)
```

### Strategy Pattern

Different adapters represent different processing strategies:
- `Transform`: Pure functions
- `Validate`: Assertions
- `Apply`: Fallible operations
- `Mutate`: Conditional logic
- `Effect`: Side effects
- `Enrich`: Optional enhancements

## Testing Strategies

### Unit Testing Processors

Test individual processors in isolation:

```go
func TestValidateEmail(t *testing.T) {
    processor := ValidateEmail()
    
    _, err := processor.Process(User{Email: "invalid"})
    assert.Error(t, err)
    
    _, err = processor.Process(User{Email: "user@example.com"})
    assert.NoError(t, err)
}
```

### Integration Testing Pipelines

Test complete pipelines:

```go
func TestUserPipeline(t *testing.T) {
    pipeline := BuildUserPipeline()
    
    user := User{Email: "TEST@EXAMPLE.COM", Name: "  John  "}
    result, err := pipeline.Process(user)
    
    assert.NoError(t, err)
    assert.Equal(t, "test@example.com", result.Email)
    assert.Equal(t, "John", result.Name)
}
```

### Mocking External Dependencies

Use Apply for testable external calls:

```go
func SaveUser(repo Repository) *Contract[User] {
    return pipz.Apply(func(u User) (User, error) {
        return repo.Save(u)
    })
}

// In tests, pass a mock repository
mockRepo := &MockRepository{}
pipeline := process.Then(SaveUser(mockRepo))
```

### Property-Based Testing

pipz's functional nature works well with property testing:

```go
func TestPipelineProperties(t *testing.T) {
    // Property: Pipeline preserves non-error data
    quick.Check(func(data TestData) bool {
        result, _ := pipeline.Process(data)
        return result.ID == data.ID
    }, nil)
}
```

## Common Patterns and Anti-Patterns

### ✅ Good Patterns

1. **Small, Focused Processors**: Each does one thing well
2. **Descriptive Names**: `ValidatePayment` not `Check`
3. **Error Context**: Include relevant data in errors
4. **Composition over Inheritance**: Build complex from simple

### ❌ Anti-Patterns

1. **Side Effects in Transform**: Use Effect instead
2. **Error Handling in Processors**: Let pipeline handle errors
3. **Mutable Shared State**: Keep processors pure
4. **Long Processor Chains**: Consider breaking into sub-pipelines

## Advanced Techniques

### Pipeline Factories

Create pipelines dynamically based on configuration:

```go
func CreatePipeline(config Config) Chainable[Data] {
    steps := []Chainable[Data]{baseProcessor}
    
    if config.ValidateEnabled {
        steps = append(steps, validator)
    }
    
    if config.EnrichEnabled {
        steps = append(steps, enricher)
    }
    
    return pipz.NewChain(steps...)
}
```

### Error Recovery

Implement retry logic in Apply:

```go
retryable := pipz.Apply(func(data Data) (Data, error) {
    for i := 0; i < 3; i++ {
        result, err := unstableOperation(data)
        if err == nil {
            return result, nil
        }
        time.Sleep(time.Second * time.Duration(i+1))
    }
    return data, fmt.Errorf("operation failed after 3 retries")
})
```

### Parallel Processing

Process collections in parallel:

```go
func ProcessBatch(items []Item) ([]Item, error) {
    var wg sync.WaitGroup
    results := make([]Item, len(items))
    errors := make([]error, len(items))
    
    for i, item := range items {
        wg.Add(1)
        go func(idx int, it Item) {
            defer wg.Done()
            results[idx], errors[idx] = pipeline.Process(it)
        }(i, item)
    }
    
    wg.Wait()
    
    // Check for errors
    for _, err := range errors {
        if err != nil {
            return nil, err
        }
    }
    
    return results, nil
}
```