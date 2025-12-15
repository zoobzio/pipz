---
title: "Apply"
description: "Creates a processor from a function that can return an error for fallible operations"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - processors
  - error-handling
  - validation
---

# Apply

Creates a processor from a function that can return an error.

> **Note**: Apply is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Apply[T any](name Name, fn func(context.Context, T) (T, error)) Chainable[T]
```

## Parameters

- `name` (`Name`) - Identifier for the processor used in error messages and debugging
- `fn` - Processing function that takes a context and input, returns output or error

## Returns

Returns a `Chainable[T]` that can be composed with other processors.

## Behavior

- **Fallible operations** - Can return errors that stop pipeline execution
- **Error wrapping** - Errors are automatically wrapped with context (path, timing, input data)
- **Context aware** - Respects cancellation and timeouts
- **Fail-fast** - Pipeline stops on first error

## Example

```go
// Validation
validate := pipz.Apply("validate", func(ctx context.Context, user User) (User, error) {
    if user.Email == "" {
        return user, errors.New("email required")
    }
    if user.Age < 0 || user.Age > 150 {
        return user, fmt.Errorf("invalid age: %d", user.Age)
    }
    return user, nil
})

// External API call
fetchData := pipz.Apply("fetch", func(ctx context.Context, id string) (Data, error) {
    resp, err := http.Get(ctx, fmt.Sprintf("/api/data/%s", id))
    if err != nil {
        return Data{}, fmt.Errorf("fetch failed: %w", err)
    }
    return parseResponse(resp)
})

// Database operation
saveUser := pipz.Apply("save", func(ctx context.Context, user User) (User, error) {
    user.ID = uuid.New()
    if err := db.Save(ctx, &user); err != nil {
        return user, fmt.Errorf("database save failed: %w", err)
    }
    return user, nil
})
```

## Error Handling

When Apply returns an error, it's wrapped in `*Error[T]` with rich context:

```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[User]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Failed at: %v\n", pipeErr.Path)
        fmt.Printf("Input: %+v\n", pipeErr.InputData)
        fmt.Printf("Duration: %v\n", pipeErr.Duration)
        
        // Check specific conditions
        if pipeErr.Timeout {
            // Handle timeout
        }
    }
}
```

## When to Use

Use `Apply` when:
- Your operation **can fail** (parsing, validation, I/O)
- You're calling external services (APIs, databases)
- You're doing validation that should stop the pipeline
- You need explicit error handling
- Parsing data (JSON, XML, user input)
- Type conversions that might fail

## When NOT to Use

Don't use `Apply` when:
- Your operation cannot fail (use `Transform` for better performance)
- You only need side effects (use `Effect` instead)
- You want to ignore errors (use `Enrich` instead)
- Simple math or string operations (use `Transform`)
- Logging or metrics (use `Effect`)

## Performance

Apply has minimal overhead for error handling:
- ~46ns per operation (success case)
- Zero allocations on success
- Small allocation only on error

## Common Patterns

```go
// Validation pipeline
validation := pipz.NewSequence[User]("validation",
    pipz.Apply("required", checkRequired),
    pipz.Apply("format", checkFormat),
    pipz.Apply("unique", checkUnique),
)

// API with retry
reliableAPI := pipz.NewRetry("api-retry",
    pipz.Apply("api-call", callExternalAPI),
    3,
)

// Database with fallback
saveWithFallback := pipz.NewFallback("save",
    pipz.Apply("primary-db", saveToPrimary),
    pipz.Apply("backup-db", saveToBackup),
)

// Parse with validation
parseAndValidate := pipz.Apply("parse-json", func(ctx context.Context, raw string) (Config, error) {
    var config Config
    if err := json.Unmarshal([]byte(raw), &config); err != nil {
        return config, fmt.Errorf("invalid JSON: %w", err)
    }
    if config.Version == "" {
        return config, errors.New("version required")
    }
    return config, nil
})
```

## Gotchas

### ❌ Don't ignore context
```go
// WRONG - Ignoring context cancellation
apply := pipz.Apply("slow", func(ctx context.Context, data Data) (Data, error) {
    time.Sleep(10 * time.Second) // Blocks even if context cancelled
    return data, nil
})
```

### ✅ Respect context cancellation
```go
// RIGHT - Check context
apply := pipz.Apply("slow", func(ctx context.Context, data Data) (Data, error) {
    select {
    case <-time.After(10 * time.Second):
        return data, nil
    case <-ctx.Done():
        return data, ctx.Err()
    }
})
```

### ❌ Don't use for pure transforms
```go
// WRONG - Unnecessary error handling
apply := pipz.Apply("double", func(ctx context.Context, n int) (int, error) {
    return n * 2, nil // Never fails!
})
```

### ✅ Use Transform for infallible operations
```go
// RIGHT - No error overhead
transform := pipz.Transform("double", func(ctx context.Context, n int) int {
    return n * 2
})
```

## See Also

- [Transform](./transform.md) - For pure transformations
- [Effect](./effect.md) - For side effects
- [Enrich](./enrich.md) - For optional operations