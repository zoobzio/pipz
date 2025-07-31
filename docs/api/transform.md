# Transform

Creates a processor from a pure transformation function that cannot fail.

## Function Signature

```go
func Transform[T any](name Name, fn func(context.Context, T) T) Chainable[T]
```

## Parameters

- `name` (`Name`) - Identifier for the processor used in error messages and debugging
- `fn` - Transformation function that takes a context and input, returns transformed output

## Returns

Returns a `Chainable[T]` that can be composed with other processors.

## Behavior

- **Pure transformation** - Cannot return errors
- **Always succeeds** - Unless context is cancelled
- **Zero allocations** - Optimal performance for simple transformations
- **Context aware** - Respects cancellation

## Example

```go
// Simple transformation
double := pipz.Transform("double", func(ctx context.Context, n int) int {
    return n * 2
})

// String manipulation
normalize := pipz.Transform("normalize", func(ctx context.Context, s string) string {
    return strings.ToLower(strings.TrimSpace(s))
})

// Struct transformation
addTimestamp := pipz.Transform("timestamp", func(ctx context.Context, event Event) Event {
    event.Timestamp = time.Now()
    return event
})
```

## When to Use

Use `Transform` when:
- Your operation cannot fail
- You're doing simple data transformations
- You want optimal performance (no error handling overhead)

## When NOT to Use

Don't use `Transform` when:
- Your operation might fail (use `Apply` instead)
- You need side effects without changing data (use `Effect` instead)
- You need conditional logic (consider `Mutate` instead)

## Performance

Transform has the best performance of all processors:
- ~2.7ns per operation
- Zero allocations
- Minimal overhead

## Common Patterns

```go
// Chain multiple transforms
pipeline := pipz.NewSequence[string]("text-processing",
    pipz.Transform("trim", strings.TrimSpace),
    pipz.Transform("lower", strings.ToLower),
    pipz.Transform("capitalize", capitalize),
)

// Data enrichment
enrichUser := pipz.Transform("enrich", func(ctx context.Context, user User) User {
    user.DisplayName = fmt.Sprintf("%s (%s)", user.Name, user.Role)
    return user
})
```

## See Also

- [Apply](./apply.md) - For operations that can fail
- [Effect](./effect.md) - For side effects
- [Mutate](./mutate.md) - For conditional transformations